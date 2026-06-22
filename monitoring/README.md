# Monitoring

Prometheus + Grafana for live Raft metrics.

## Start the stack

Terminal 1: playground and cluster

```bash
go run ./visualizer --no-browser --sandbox
```

Configure and start a cluster in the UI at http://localhost:8080.

Terminal 2: monitoring

```bash
docker compose -f monitoring/docker-compose.yml up
```

- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000
- Embedded panels appear at the bottom of the playground UI once Grafana is running

## Metrics

Per-node (`GET http://localhost:800N/metrics`):

| Metric | Description |
|---|---|
| `raftdb_term` | Current term |
| `raftdb_commit_index` | Committed log index |
| `raftdb_last_applied` | Applied log index |
| `raftdb_log_length` | Log size |
| `raftdb_state` | 0=follower, 1=candidate, 2=leader |
| `raftdb_is_leader` | 1 if leader |
| `raftdb_elections_total` | Elections started |
| `raftdb_commits_total` | Commits as leader |
| `raftdb_append_entries_total` | AppendEntries results |

Cluster (`GET http://localhost:8080/metrics`):

| Metric | Description |
|---|---|
| `raftdb_replication_lag` | Leader commit minus node commit |
| `raftdb_leader_count` | Nodes reporting leader state |

## Dashboard panels

1. Commit index by node
2. Replication lag
3. Raft term

## Notes

- Prometheus scrapes `host.docker.internal:8001-8005` and `:8080`. Adjust `monitoring/prometheus.yml` if you use a different node count or port.
- Disable per-node metrics with `--metrics=false` when starting ryanDB manually.
