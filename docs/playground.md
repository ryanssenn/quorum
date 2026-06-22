# Playground User Guide

The playground runs real `ryanDB` processes. Not a simulation.

## Getting started

```bash
go run ./visualizer --no-browser --sandbox
```

1. Set the node count (3–9) and click **Configure**
2. Click **Start** and wait for a leader to appear in the HUD
3. Pick a target node, enter a key/value, and click **Write**
4. Watch numbered log entries on each node card update as entries replicate and commit

Screenshots: [playground-start.png](images/playground-start.png), [playground-write.png](images/playground-write.png), [playground-failover.png](images/playground-failover.png)

## What you'll see

Each node card shows:

| Element | Meaning |
|---|---|
| Role badge | follower, candidate, leader, or offline |
| Term / commit | Current Raft term and commit index |
| Numbered log list | Last ~12 entries with `#index op key=value` |
| Green row | Entry is committed (`index <= commitIndex`) |
| Gray row | Entry replicated but not yet committed |

The HUD at the top tracks cluster-wide term, commit index, leader, and quorum progress.

### Advanced drawer

Open **Advanced** in the sidebar for chaos experiments:

- Click a node card to select it, then **Kill**, **Restart**, or **Isolate**
- Load and run guided tours
- Scroll the event log for step-by-step scenario output

## Metrics (optional)

For live graphs, start the monitoring stack in a second terminal:

```bash
docker compose -f monitoring/docker-compose.yml up
```

Then refresh the playground UI. Three Grafana panels embed at the bottom (commit index, replication lag, Raft term). Open the full dashboard at http://localhost:3000.

See [monitoring/README.md](../monitoring/README.md) for metric names and scrape targets.

## Experiments to try

### Leader failure
1. Note the current leader in the HUD
2. Click the leader's node card, open **Advanced**, and click **Kill**
3. Watch term increase and a new leader appear
4. Send writes under the new leader
5. **Restart** the killed node and watch its log catch up

### Network partition
1. Start a 5-node cluster and write a key
2. Select a node and click **Isolate**
3. Write from a node in the majority partition. Commits succeed
4. **Clear partition**. Logs converge across all nodes

### Persistence
1. Write several keys
2. Kill and restart nodes one at a time
3. Read keys from restarted nodes. Data survives via disk persistence

## Guided tours

Load a preset from the Advanced drawer:

| Tour | Teaches |
|---|---|
| Leader election | Forced re-election after leader kill |
| Leader failure | Kill + restart + catch-up |
| Network partition | Split-brain prevention via quorum |
| Log persistence | Survive node restarts |

Click **Run** after loading. Use **Pause** to freeze between steps.

## Under the hood

Each node is a real `ryanDB` process:
- HTTP API on port 8001+
- gRPC Raft RPCs on port 9001+
- Logs persisted under `logs/` (`.rlog`, `.meta`)
- Prometheus metrics at `/metrics` (disable with `--metrics=false`)

The playground observes nodes via `/status`, `/log`, and `/events`. Partition simulation uses `/simulate/block` to drop gRPC between peers without stopping processes.

For correctness guarantees, testing methodology, and benchmarks, see:
- [Testing guide](development/testing.md)
- [Performance report](../benchmarks/REPORT.md)
- [Internals guide](guide.md)
