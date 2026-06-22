package main

import (
	"fmt"
	"net/http"
)

func (srv *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	statuses := srv.clusterStatusLocked()

	var leaderCommit int64 = -1
	leaderCount := 0

	for _, ns := range statuses {
		if !ns.Running || !ns.Reachable {
			continue
		}
		if ns.State == 2 || ns.StateName == "leader" {
			leaderCount++
			leaderCommit = ns.CommitIndex
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	fmt.Fprintf(w, "# HELP raftdb_leader_count Number of nodes reporting leader state.\n")
	fmt.Fprintf(w, "# TYPE raftdb_leader_count gauge\n")
	fmt.Fprintf(w, "raftdb_leader_count %d\n\n", leaderCount)

	fmt.Fprintf(w, "# HELP raftdb_replication_lag Commit index lag vs current leader.\n")
	fmt.Fprintf(w, "# TYPE raftdb_replication_lag gauge\n")
	for _, ns := range statuses {
		if !ns.Running || !ns.Reachable {
			continue
		}
		lag := float64(0)
		if leaderCommit >= 0 {
			lag = float64(leaderCommit - ns.CommitIndex)
			if lag < 0 {
				lag = 0
			}
		}
		fmt.Fprintf(w, "raftdb_replication_lag{node=%q} %g\n", ns.ID, lag)
	}
}
