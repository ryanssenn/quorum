package core

import (
	"testing"
	"time"
)

func electionPeers() map[string]string {
	return map[string]string{
		"node1": "localhost:19101",
		"node2": "localhost:19102",
		"node3": "localhost:19103",
	}
}

func startElectionCluster(t *testing.T, peers map[string]string) map[string]*Node {
	t.Helper()
	t.Chdir(t.TempDir())

	nodes := map[string]*Node{}
	for id := range peers {
		n := NewNode(id, peers)
		n.Logger.ClearData()
		n.Events = nil
		nodes[id] = n
	}
	for _, n := range nodes {
		n.StartServer()
	}
	for _, n := range nodes {
		n.StartClients()
	}
	for _, n := range nodes {
		go n.StartElectionTimer()
	}
	return nodes
}

func waitForElectedLeader(nodes map[string]*Node, minTerm int64, exclude string, timeout time.Duration) *Node {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for id, n := range nodes {
			if id == exclude {
				continue
			}
			if n.GetState() == Leader && n.Term.Load() >= minTerm {
				return n
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

// Simulates the playground "crash node" on the leader of a healthy 3-node
// cluster and asserts the two survivors elect a new leader. Run with -race to
// guard against unsynchronized access to Node.state, which previously let a
// follower's election-timer goroutine observe a stale role and never call
// StartElection after the leader died.
func TestLeaderCrashReelection(t *testing.T) {
	peers := electionPeers()
	nodes := startElectionCluster(t, peers)

	leader := waitForElectedLeader(nodes, 1, "", 5*time.Second)
	if leader == nil {
		t.Fatal("no initial leader elected")
	}
	t.Logf("initial leader %s at term %d", leader.Id, leader.Term.Load())
	crashedTerm := leader.Term.Load()

	// Crash the leader: isolate it from every peer in both directions, the way a
	// killed process would stop both sending and receiving.
	for id, n := range nodes {
		if id == leader.Id {
			for pid := range peers {
				if pid != id {
					n.BlockPeer(pid)
				}
			}
		} else {
			n.BlockPeer(leader.Id)
		}
	}

	newLeader := waitForElectedLeader(nodes, crashedTerm+1, leader.Id, 6*time.Second)
	if newLeader == nil {
		t.Fatalf("no new leader elected after crashing %s (term %d)", leader.Id, crashedTerm)
	}
	t.Logf("new leader %s at term %d", newLeader.Id, newLeader.Term.Load())
}
