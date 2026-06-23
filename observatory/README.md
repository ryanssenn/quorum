# Observatory

Grafana-first observability tool for ryanDB. Runs real Raft nodes, executes scenario scripts, and exposes live metrics.

## Quick start

```bash
go run ./observatory --no-browser --compose-up observatory/scenarios/leader-failure.json
```

Interactive mode:

```bash
go run ./observatory --no-browser
docker compose -f monitoring/docker-compose.yml up
```

Open http://localhost:8080. Configure a cluster, load a scenario, click **Run**, and watch Grafana panels update.

## CLI flags

| Flag | Default | Description |
|---|---|---|
| `--port` | 8080 | Observatory HTTP port |
| `--no-browser` | false | Skip opening browser |
| `--binary` | auto-build | Path to ryanDB binary |
| `--demo` | true | Compress scenario wait times |
| `--compose-up` | false | Start Prometheus + Grafana via docker compose |

Pass a scenario path as the first argument to auto-start cluster and run it:

```bash
go run ./observatory --no-browser observatory/scenarios/steady-writes.json
```

## Scenarios

| File | Demonstrates |
|---|---|
| `steady-writes.json` | Commit rate, low replication lag |
| `leader-failure.json` | Term spike, election rate, recovery |
| `partition.json` | Lag on minority, append failures |
| `recovery.json` | Lag decay after heal |
| `election.json` | Re-election after leader kill |

Scenario JSON schema: each step has exactly one action (`wait`, `put`, `get`, `kill`, `restart`, `partition`, `clear_partition`).

## API

| Method | Path | Description |
|---|---|---|
| GET | `/api/cluster/status` | Node status snapshot |
| POST | `/api/cluster/create` | `{"nodes": N}` |
| POST | `/api/cluster/start` | Start cluster |
| POST | `/api/cluster/stop` | Stop cluster |
| GET | `/api/scenario` | Scenario state |
| POST | `/api/scenario/load` | `{"path": "..."}` |
| POST | `/api/scenario/run` | Run loaded scenario |
| POST | `/api/scenario/pause` | Toggle pause |
| POST | `/api/scenario/reset` | Reset scenario |
| GET | `/metrics` | Cluster-level Prometheus metrics |

## Docs

- [Observability metrics guide](../docs/observability.md)
- [Monitoring stack](../monitoring/README.md)
- [Raft internals](../docs/guide.md)
