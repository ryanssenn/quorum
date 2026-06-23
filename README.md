# ryanDB

Go implementation of Raft with an observability demo on top. Start a real cluster, run a guided demo, and watch throughput and consensus metrics live.

This is for learning, not production. The Raft code is tested (unit + integration) and benchmarked.

## Try it

**Prerequisite:** [Docker Desktop](https://www.docker.com/products/docker-desktop/) must be running (for Prometheus).

```bash
go run ./observatory
```

This single command:

1. Starts Prometheus via Docker
2. Boots a 5-node cluster and runs the full demo scenario
3. Opens http://localhost:8080 with live cluster topology and native metrics charts

Metrics reference: [docs/observability.md](docs/observability.md)

## Benchmarks

3-node cluster on a single host ([full report](benchmarks/REPORT.md)):

| Metric | Result |
|---|---|
| Read throughput (peak) | ~72,000 ops/sec |
| Write throughput (64 clients) | ~19,500 ops/sec |
| Read latency, p99 (16 clients) | ~1.3 ms |
| Write latency, p99 (16 clients) | ~4 ms |
| Failover recovery after leader crash | ~327 ms |

## The Raft implementation

A Go implementation of the [Raft paper](https://raft.github.io/raft.pdf) with a small in-memory key-value store on top.

Read the code: [docs/guide.md](docs/guide.md)

## Tests

```bash
go test -race ./core
go test -v ./test
go test ./observatory/...
```

## Running a cluster manually

Each node needs an HTTP port (`--port`) and a gRPC port (in `--peers` as `id=host:port`). Start at least three nodes for a working cluster.

```bash
go build -o ryanDB .

./ryanDB \
  --id=node1 \
  --port=8001 \
  --peers=node1=127.0.0.1:9001,node2=127.0.0.1:9002,node3=127.0.0.1:9003 \
  --reset=true
```

Per-node Prometheus metrics are at `/metrics` (disable with `--metrics=false`).

## Not yet implemented

- Log compaction / snapshots
- Dynamic cluster membership
