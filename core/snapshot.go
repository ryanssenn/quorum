package core

import (
	"context"
	"encoding/json"
	"time"

	pb "github.com/ryansenn/quorum/proto/nodepb"
)

// SnapshotThreshold is the number of applied entries that must accumulate beyond
// the current snapshot before the log is compacted. It is a var (not a const) so
// tests can lower it to exercise compaction without writing thousands of keys.
var SnapshotThreshold int64 = 2000

// lastLogIndexLocked returns the absolute index of the last log entry, or the
// snapshot's last-included index when the in-memory log is empty. LogMu held.
func (n *Node) lastLogIndexLocked() int64 {
	return n.SnapshotIndex.Load() + int64(len(n.Log))
}

// termAtLocked returns the term of the entry at an absolute index. The snapshot
// boundary (and anything compacted below it) reports the snapshot term. LogMu held.
func (n *Node) termAtLocked(absIndex int64) int64 {
	si := n.SnapshotIndex.Load()
	if absIndex <= si {
		return n.SnapshotTerm.Load()
	}
	rel := absIndex - si - 1
	if rel < 0 || rel >= int64(len(n.Log)) {
		return 0
	}
	return n.Log[rel].Term
}

// runCompactor periodically folds the applied prefix of the log into a snapshot
// and truncates it, keeping the log (and replay-on-restart cost) bounded.
func (n *Node) runCompactor() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		n.maybeCompact()
	}
}

// maybeCompact snapshots the state machine up to LastApplied and drops the
// covered log prefix, once enough new entries have been applied since the last
// snapshot. Holding ApplyMu pins the state machine to a consistent cut; LogMu
// guards the log/offset rewrite.
func (n *Node) maybeCompact() {
	n.ApplyMu.Lock()
	defer n.ApplyMu.Unlock()

	applied := n.LastApplied.Load()
	si := n.SnapshotIndex.Load()
	if applied-si < SnapshotThreshold {
		return
	}

	n.LogMu.Lock()
	term := n.termAtLocked(applied)
	dropRel := applied - si
	if dropRel > int64(len(n.Log)) {
		dropRel = int64(len(n.Log))
	}
	remaining := append([]*LogEntry(nil), n.Log[dropRel:]...)
	state := n.Storage.Snapshot()
	data, _ := json.Marshal(state)

	n.Logger.WriteSnapshot(&Snapshot{LastIncludedIndex: applied, LastIncludedTerm: term, Data: state})
	n.Logger.Rewrite(remaining)
	n.Log = remaining
	n.SnapshotIndex.Store(applied)
	n.SnapshotTerm.Store(term)
	n.snapshotData = data
	n.LogMu.Unlock()
}

// sendSnapshot ships the current snapshot to a follower that has fallen behind
// the compacted prefix, then advances its NextIndex/MatchIndex. Returns true on
// a successful install.
func (n *Node) sendSnapshot(id string) bool {
	if err := n.checkPeerBlocked(id); err != nil {
		return false
	}

	n.LogMu.Lock()
	si := n.SnapshotIndex.Load()
	st := n.SnapshotTerm.Load()
	data := n.snapshotData
	n.LogMu.Unlock()
	if si < 0 || data == nil {
		return false
	}

	req := &pb.SnapshotRequest{
		Term:              n.Term.Load(),
		LeaderId:          n.Id,
		LastIncludedIndex: si,
		LastIncludedTerm:  st,
		Data:              data,
	}

	n.recordEvent(Event{
		Type:    "install_snapshot",
		From:    n.Id,
		To:      id,
		Term:    n.Term.Load(),
		Entries: int(si + 1),
	})

	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	resp, err := n.Clients[id].InstallSnapshot(ctx, req)
	cancel()
	if err != nil {
		return false
	}
	if resp.Term > n.Term.Load() {
		return false
	}

	n.NextIndex[id].Store(si + 1)
	n.MatchIndex[id].Store(si)
	n.UpdateCommitIndex()
	return true
}
