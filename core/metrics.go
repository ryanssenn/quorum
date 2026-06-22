package core

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	termGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_term",
		Help: "Current Raft term.",
	}, []string{"node"})

	commitIndexGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_commit_index",
		Help: "Highest committed log index.",
	}, []string{"node"})

	lastAppliedGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_last_applied",
		Help: "Highest applied log index.",
	}, []string{"node"})

	logLengthGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_log_length",
		Help: "Number of log entries.",
	}, []string{"node"})

	stateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_state",
		Help: "Raft state: 0=follower, 1=candidate, 2=leader.",
	}, []string{"node"})

	isLeaderGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "raftdb_is_leader",
		Help: "1 if this node is the leader.",
	}, []string{"node"})

	electionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "raftdb_elections_total",
		Help: "Election attempts started by this node.",
	}, []string{"node"})

	commitsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "raftdb_commits_total",
		Help: "Log entries committed by this node as leader.",
	}, []string{"node"})

	appendEntriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "raftdb_append_entries_total",
		Help: "AppendEntries RPC results.",
	}, []string{"node", "result"})
)

func RegisterNodeMetrics(n *Node) {
	id := n.Id
	termGauge.WithLabelValues(id).Set(float64(n.Term.Load()))
	commitIndexGauge.WithLabelValues(id).Set(float64(n.CommitIndex.Load()))
	lastAppliedGauge.WithLabelValues(id).Set(float64(n.LastApplied.Load()))
	logLengthGauge.WithLabelValues(id).Set(float64(n.GetLogSize()))
	stateGauge.WithLabelValues(id).Set(float64(n.State))
	if n.State == Leader {
		isLeaderGauge.WithLabelValues(id).Set(1)
	} else {
		isLeaderGauge.WithLabelValues(id).Set(0)
	}
}

func (n *Node) RefreshMetrics() {
	RegisterNodeMetrics(n)
}

func (n *Node) RecordElection() {
	electionsTotal.WithLabelValues(n.Id).Inc()
}

func (n *Node) RecordCommit() {
	commitsTotal.WithLabelValues(n.Id).Inc()
}

func (n *Node) RecordAppendEntries(result string) {
	appendEntriesTotal.WithLabelValues(n.Id, result).Inc()
}
