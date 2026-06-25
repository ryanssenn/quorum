package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/ryansenn/quorum/proto/nodepb"
)

func testPeers() map[string]string {
	return map[string]string{
		"node1": "localhost:19001",
		"node2": "localhost:19002",
		"node3": "localhost:19003",
	}
}

func testNode(t *testing.T, id string) *Node {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)

	n := NewNode(id, testPeers())
	n.Logger.ClearData()
	return n
}

func TestUpdateCommitIndex(t *testing.T) {
	n := testNode(t, "node1")
	n.State = Leader
	n.Term.Store(1)

	oldTerm := NewLogEntry(0, NewCommand("put", "old", "x"))
	current := NewLogEntry(1, NewCommand("put", "k", "v"))
	n.Log = []*LogEntry{oldTerm, current}

	n.MatchIndex["node1"].Store(1)
	n.MatchIndex["node2"].Store(1)
	n.MatchIndex["node3"].Store(0)

	n.UpdateCommitIndex()

	if got := n.CommitIndex.Load(); got != 1 {
		t.Fatalf("CommitIndex = %d, want 1", got)
	}
	if got := n.LastApplied.Load(); got != 1 {
		t.Fatalf("LastApplied = %d, want 1", got)
	}
	if got := n.Storage.Get("k"); got != "v" {
		t.Fatalf("storage value = %q, want %q", got, "v")
	}
	if got := n.Storage.Get("old"); got != "x" {
		t.Fatalf("prior log entry should be applied when later index commits, got %q", got)
	}
}

func TestApplyCommittedOrdering(t *testing.T) {
	n := testNode(t, "node1")
	n.Log = []*LogEntry{NewLogEntry(1, NewCommand("put", "k", "v"))}
	n.CommitIndex.Store(0)

	done := make(chan string, 1)
	go func() {
		done <- n.Get("k")
	}()

	deadline := time.After(2 * time.Second)
	select {
	case <-deadline:
		t.Fatal("Get blocked before ApplyCommitted")
	default:
	}

	time.Sleep(20 * time.Millisecond)
	n.ApplyCommitted()

	select {
	case val := <-done:
		if val != "v" {
			t.Fatalf("Get = %q, want %q", val, "v")
		}
	case <-deadline:
		t.Fatal("Get did not unblock after ApplyCommitted")
	}
}

func TestCommitWaitsForApply(t *testing.T) {
	n := testNode(t, "node1")
	n.State = Leader
	n.Term.Store(1)

	go func() {
		time.Sleep(20 * time.Millisecond)
		n.MatchIndex["node1"].Store(0)
		n.MatchIndex["node2"].Store(0)
		n.MatchIndex["node3"].Store(0)
		n.UpdateCommitIndex()
	}()

	n.Commit(NewCommand("put", "k", "v"))

	if got := n.Storage.Get("k"); got != "v" {
		t.Fatalf("storage value = %q, want %q after Commit", got, "v")
	}
}

func TestSnapshotCompaction(t *testing.T) {
	n := testNode(t, "node1")
	n.State = Leader
	n.Term.Store(1)

	prev := SnapshotThreshold
	SnapshotThreshold = 5
	defer func() { SnapshotThreshold = prev }()

	for i := 0; i < 20; i++ {
		n.AppendLog(NewCommand("put", fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i)))
	}
	n.CommitIndex.Store(int64(n.GetLogSize() - 1))
	n.ApplyCommitted()
	if n.LastApplied.Load() != 19 {
		t.Fatalf("LastApplied = %d, want 19", n.LastApplied.Load())
	}

	n.maybeCompact()

	if got := n.SnapshotIndex.Load(); got != 19 {
		t.Fatalf("SnapshotIndex = %d, want 19", got)
	}
	n.LogMu.Lock()
	tail := len(n.Log)
	n.LogMu.Unlock()
	if tail != 0 {
		t.Fatalf("log tail = %d, want 0 after compacting all applied entries", tail)
	}
	// GetLogSize stays absolute (last index + 1) so index math elsewhere holds.
	if got := n.GetLogSize(); got != 20 {
		t.Fatalf("GetLogSize = %d, want 20", got)
	}
	if got := n.GetLogTerm(-1); got != 1 {
		t.Fatalf("last term = %d, want snapshot term 1", got)
	}
	if got := n.Storage.Get("k19"); got != "v19" {
		t.Fatalf("state after compaction: k19 = %q, want v19", got)
	}

	// Appends keep using absolute indices after compaction.
	if idx := n.AppendLog(NewCommand("put", "k20", "v20")); idx != 20 {
		t.Fatalf("post-compaction append index = %d, want 20", idx)
	}
	n.CommitIndex.Store(20)
	n.ApplyCommitted()
	if got := n.Storage.Get("k20"); got != "v20" {
		t.Fatalf("post-compaction value k20 = %q, want v20", got)
	}

	// A fresh node sharing the on-disk state recovers from snapshot + tail.
	fresh := NewNode("node1", testPeers())
	fresh.Logger = n.Logger
	fresh.RecoverState()
	if got := fresh.SnapshotIndex.Load(); got != 19 {
		t.Fatalf("recovered SnapshotIndex = %d, want 19", got)
	}
	if got := fresh.CommitIndex.Load(); got != 20 {
		t.Fatalf("recovered CommitIndex = %d, want 20", got)
	}
	if got := fresh.Storage.Get("k0"); got != "v0" {
		t.Fatalf("recovered snapshot state k0 = %q, want v0", got)
	}
	if got := fresh.Storage.Get("k20"); got != "v20" {
		t.Fatalf("recovered tail entry k20 = %q, want v20", got)
	}
}

func TestRequestVoteGrantDeny(t *testing.T) {
	follower := testNode(t, "node2")
	follower.Term.Store(1)
	storeString(&follower.VoteFor, "")

	srv := &server{node: follower}

	grant, err := srv.RequestVote(context.Background(), &pb.VoteRequest{
		Term:         1,
		CandidateId:  "node1",
		LastLogIndex: -1,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !grant.VoteGranted {
		t.Fatal("expected vote to be granted")
	}

	deny, err := srv.RequestVote(context.Background(), &pb.VoteRequest{
		Term:         1,
		CandidateId:  "node3",
		LastLogIndex: -1,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if deny.VoteGranted {
		t.Fatal("expected second vote in same term to be denied")
	}

	higherTerm, err := srv.RequestVote(context.Background(), &pb.VoteRequest{
		Term:         0,
		CandidateId:  "node1",
		LastLogIndex: -1,
		LastLogTerm:  0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if higherTerm.VoteGranted {
		t.Fatal("expected vote denied when follower term is higher")
	}
}

func TestAppendEntriesConsistency(t *testing.T) {
	follower := testNode(t, "node2")
	follower.Term.Store(1)
	follower.Log = []*LogEntry{NewLogEntry(1, NewCommand("put", "k1", "v1"))}

	srv := &server{node: follower}

	mismatch, err := srv.AppendEntries(context.Background(), &pb.AppendRequest{
		Term:         1,
		LeaderId:     "node1",
		PrevLogIndex: 0,
		PrevLogTerm:  99,
		Entries:      nil,
		LeaderCommit: -1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mismatch.Success {
		t.Fatal("expected append to fail on term mismatch")
	}

	cmdBytes, err := json.Marshal(NewCommand("put", "k2", "v2"))
	if err != nil {
		t.Fatal(err)
	}
	ok, err := srv.AppendEntries(context.Background(), &pb.AppendRequest{
		Term:         1,
		LeaderId:     "node1",
		PrevLogIndex: 0,
		PrevLogTerm:  1,
		Entries: []*pb.LogEntry{
			{Term: 1, Command: cmdBytes},
		},
		LeaderCommit: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ok.Success {
		t.Fatal("expected append to succeed")
	}
	if follower.GetLogSize() != 2 {
		t.Fatalf("log size = %d, want 2", follower.GetLogSize())
	}
}

func TestRecoverStateAppliesLog(t *testing.T) {
	n := testNode(t, "node1")
	n.Logger.AppendLog(NewLogEntry(1, NewCommand("put", "k", "v")))

	fresh := NewNode("node1", testPeers())
	fresh.Logger = n.Logger
	fresh.RecoverState()

	if got := fresh.Storage.Get("k"); got != "v" {
		t.Fatalf("recovered storage = %q, want %q", got, "v")
	}
}

func TestLoggerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	logger := newLogger("unit-node")
	logger.ClearData()
	logger.WriteMeta(3, "node2")

	entries := []*LogEntry{
		NewLogEntry(1, NewCommand("put", "a", "1")),
		NewLogEntry(2, NewCommand("put", "b", "2")),
	}
	for _, e := range entries {
		logger.AppendLog(e)
	}

	term, votedFor := logger.LoadMeta()
	if term != 3 || votedFor != "node2" {
		t.Fatalf("meta = (%d, %q), want (3, node2)", term, votedFor)
	}

	loaded := logger.LoadLogs()
	if len(loaded) != 2 {
		t.Fatalf("loaded %d entries, want 2", len(loaded))
	}
	if loaded[1].Command.Key != "b" || loaded[1].Command.Value != "2" {
		t.Fatalf("second entry = %+v", loaded[1].Command)
	}

	// Ensure files live under the temp logs directory.
	if _, err := os.Stat(filepath.Join("logs", "unit-node.rlog")); err != nil {
		t.Fatalf("expected log file in temp dir: %v", err)
	}
}
