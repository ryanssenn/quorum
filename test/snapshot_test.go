package test

import (
	"fmt"
	"testing"
	"time"
)

// TestSnapshotCatchUp forces the leader to compact past a stopped follower's log,
// then restarts that follower and verifies it recovers via InstallSnapshot rather
// than a full log replay.
func TestSnapshotCatchUp(t *testing.T) {
	SnapshotThresholdArg = 25
	t.Cleanup(func() { SnapshotThresholdArg = 0 })

	nodes := InitNodes(t)
	leader := WaitForLeader(t, nodes, 15*time.Second)

	// Pick a follower to take offline before the bulk of the writes.
	var victim *Node
	for _, n := range nodes {
		if n != leader {
			victim = n
			break
		}
	}
	t.Logf("stopping follower %s before bulk writes", victim.id)
	victim.StopNode()
	WaitForNodeDown(t, victim, 10*time.Second)

	live := make([]*Node, 0, len(nodes)-1)
	for _, n := range nodes {
		if n != victim {
			live = append(live, n)
		}
	}

	// Write enough to push the snapshot well past where the victim left off.
	for i := 0; i < 300; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("val%d", i)
		live[i%len(live)].PutMustSucceed(t, key, val)
	}

	// Confirm the surviving leader actually compacted (snapshotIndex advanced).
	l := WaitForLeader(t, live, 15*time.Second)
	leaderSnap := waitForSnapshot(t, l, 0, 15*time.Second)
	t.Logf("leader %s compacted to snapshotIndex=%d", l.id, leaderSnap)

	// Bring the victim back; it is far enough behind that the leader must ship a
	// snapshot to it.
	t.Logf("restarting follower %s", victim.id)
	victim.StartNode(t, "false")
	WaitForLeader(t, nodes, 30*time.Second)

	// The victim started with an empty log; any positive snapshotIndex means the
	// leader shipped it a snapshot. Then it should catch its applied index up to
	// the rest of the cluster.
	victimSnap := waitForSnapshot(t, victim, 0, 30*time.Second)
	t.Logf("follower %s installed snapshotIndex=%d (leader was %d)", victim.id, victimSnap, leaderSnap)
	waitForCaughtUp(t, nodes, 30*time.Second)

	// A fresh write after recovery is visible cluster-wide.
	l = WaitForLeader(t, nodes, 15*time.Second)
	l.PutMustSucceed(t, "final", "done")
	WaitForValue(t, nodes, "final", "done", 20*time.Second)

	// Spot-check earlier values that only exist below the snapshot boundary.
	WaitForValue(t, nodes, "key0", "val0", 15*time.Second)
	WaitForValue(t, nodes, "key299", "val299", 15*time.Second)
}

func waitForSnapshot(t *testing.T, node *Node, min int64, timeout time.Duration) int64 {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s, err := node.TryStatus(); err == nil && s.SnapshotIndex > min {
			return s.SnapshotIndex
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s snapshotIndex > %d", node.id, min)
	return 0
}

func waitForCaughtUp(t *testing.T, nodes []*Node, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if clusterCaughtUp(nodes) {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	dumpNodeStatuses(t, nodes)
	t.Fatal("timed out waiting for cluster to catch up")
}
