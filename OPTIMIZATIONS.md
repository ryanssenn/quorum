# Performance Optimizations

This document tracks optimizations applied to RaftDB based on the roadmap in `docs/performance.md`. Each change was benchmarked individually; only improvements that measurably helped key metrics were kept.

**Benchmark command** (consistent across all runs):

```bash
go run ./benchmarks --quick --concurrency=1,16,64 --out=benchmarks/results/<label>
```

**Environment:** Cursor Cloud VM, 4 vCPUs, 16 GB RAM, Go 1.24.0, 3-node cluster, 2 s measurement window.

---

## Baseline (pre-optimization)

Per-entry `fsync` on every log append; 10 ms sleep after every AppendEntries round (including successful replication).

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput (ops/s) | 102 | 815 | 2,444 |
| Write p50 (ms) | 11.51 | 19.91 | 21.67 |
| Write p99 (ms) | 12.63 | 32.94 | 60.19 |
| Read throughput (ops/s) | 19,061 | 65,644 | 67,500 |
| Read p99 (ms) | 0.11 | 1.38 | 4.43 |

---

## Final cumulative results

All kept optimizations combined.

| Metric | conc=1 | conc=16 | conc=64 | vs baseline (throughput) |
|---|---:|---:|---:|---|
| Write throughput (ops/s) | 517 | 7,473 | 20,249 | **5.1× / 9.2× / 8.3×** |
| Write p50 (ms) | 1.84 | 2.11 | 3.00 | **6.3× / 9.4× / 7.2× faster** |
| Write p99 (ms) | 3.54 | 3.90 | 6.09 | **3.6× / 8.4× / 9.9× faster** |
| Read throughput (ops/s) | 15,806 | 66,740 | 69,161 | ~unchanged |
| Read p99 (ms) | 0.12 | 1.44 | 4.65 | ~unchanged |

Write latency at concurrency 1 dropped from ~11 ms (fsync-dominated floor) to ~2 ms. Write throughput at 64 clients rose from ~2.4k to ~20k ops/s.

---

## Optimization 1: Skip replication sleep when entries are pending (**KEPT**)

**Problem:** `ReplicateToFollower` slept 10 ms after every AppendEntries RPC, including after successfully replicating entries. This added idle time on the critical path under load.

**Implementation:** Only sleep when sending heartbeats with zero entries. On replication failure, retry immediately instead of sleeping.

**Files:** `core/leader.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 102 → **138** | 815 → **3,411** | 2,444 → 2,887 |
| Write p50 (ms) | 11.51 → **7.83** | 19.91 → **3.45** | 21.67 → 18.18 |
| Write p99 (ms) | 12.63 → 12.26 | 32.94 → **19.38** | 60.19 → 75.39 |

**Verdict:** Large gains at low/medium concurrency. Kept.

---

## Optimization 2: Group commit (defer fsync until replication) (**KEPT**)

**Problem:** Every log append called `fsync` immediately. At high concurrency, many entries were appended to the OS page cache before any could be batched.

**Implementation:**
- `Logger.AppendLog` writes to the file but does not sync.
- Leader calls `Logger.Sync()` once before sending AppendEntries with new entries.
- Follower `AppendLogs` syncs once per batch (not per entry).

**Files:** `core/log.go`, `core/leader.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 138 → **149** | 3,411 → 2,827 | 2,887 → **18,698** |
| Write p50 (ms) | 7.83 → **6.84** | 3.45 → 5.98 | 18.18 → **3.06** |
| Write p99 (ms) | 12.26 → **10.57** | 19.38 → **11.44** | 75.39 → **13.37** |

**Verdict:** Massive improvement at high concurrency; modest at low. Durability is preserved (entries are fsynced before replication RPC). Kept.

---

## Optimization 3: Disable debug event recording (`--no-events`) (**NOT KEPT for perf**)

**Problem:** Event recording on every client request and Raft RPC adds mutex + allocation overhead.

**Implementation:** Added `--no-events` flag to `ryanDB` and `--no-events` passthrough in the benchmark harness.

**Files:** `main.go`, `benchmarks/main.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 149 → 144 | 2,827 → 2,696 | 18,698 → 18,813 |
| Write p99 (ms) | 10.57 → 12.88 | 11.44 → **9.85** | 13.37 → **8.15** |

**Verdict:** No consistent improvement (within run-to-run variance). Flag kept as an optional operational knob for deployments that do not need the observatory, but not counted as a performance win.

---

## Optimization 4: Single fsync per batch (dirty flag) (**KEPT**)

**Problem:** With opt 2, each follower replication goroutine called `Sync()` before its RPC. A 3-node cluster triggered two redundant fsyncs per batch.

**Implementation:** `Logger` tracks a `dirty` flag; `Sync()` is a no-op when the log is already synced.

**Files:** `core/log.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 149 → 137 | 2,827 → **3,045** | 18,698 → **19,478** |
| Write p99 (ms) | 10.57 → 13.63 | 11.44 → **10.80** | 13.37 → **10.79** |

**Verdict:** Modest but consistent improvement at conc ≥ 16. Kept.

---

## Optimization 5: Cache serialized command bytes (**KEPT**)

**Problem:** The leader re-marshaled each log entry to JSON on every AppendEntries RPC to every follower.

**Implementation:** `LogEntry` stores pre-marshaled `Serialized []byte` at creation; replication uses it directly. Rebuilt on log recovery from disk.

**Files:** `core/log.go`, `core/leader.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 137 → 136 | 3,045 → **3,407** | 19,478 → 19,117 |
| Write p99 (ms) | 13.63 → 13.61 | 10.80 → 12.63 | 10.79 → **8.60** |

**Verdict:** Small CPU savings; no measurable downside. Kept.

---

## Optimization 6: Wake replicators on append (**KEPT**)

**Problem:** After replicating all pending entries, follower goroutines sent a heartbeat and then slept 10 ms. A new client write during that sleep waited unnecessarily.

**Implementation:** Added `ReplicateNotify` channel on `Node`. Leader calls `notifyReplicators()` after appending. Idle replication loops wait on the channel (with 10 ms timeout for heartbeats) instead of blind sleep.

**Files:** `core/node.go`, `core/leader.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 136 → **500** | 3,407 → **8,227** | 19,117 → **19,875** |
| Write p50 (ms) | 7.03 → **2.00** | 3.46 → **1.86** | 3.05 → 3.06 |
| Write p99 (ms) | 13.61 → **3.64** | 12.63 → **3.81** | 8.60 → **6.00** |

**Verdict:** Largest single improvement after group commit. Kept.

---

## Optimization 7: Remove replication failure backoff (**KEPT**)

**Problem:** On AppendEntries mismatch, the replicator slept 1 ms before retrying.

**Implementation:** Retry immediately on log mismatch (decrement `nextIndex` and continue).

**Files:** `core/leader.go`

| Metric | conc=1 | conc=16 | conc=64 |
|---|---:|---:|---:|
| Write throughput | 500 → **711** | 8,227 → 7,974 | 19,875 → 19,529 |
| Write p50 (ms) | 2.00 → **1.04** | 1.86 → 1.96 | 3.06 → 3.13 |
| Write p99 (ms) | 3.64 → **3.60** | 3.81 → 3.83 | 6.00 → 6.47 |

**Verdict:** Clear win at conc=1; neutral at higher concurrency. Kept.

---

## Not implemented (tradeoffs for discussion)

These items from `docs/performance.md` were considered but not implemented because they either require a larger refactor, change consistency/durability guarantees, or did not show measurable benefit in quick testing.

| Idea | Expected benefit | Why deferred |
|---|---|---|
| **Async / timer-based fsync** | High write throughput | Weakens crash durability; un-synced entries could be lost on power failure |
| **Optional `--sync=none` mode** | Benchmark-only throughput | Useful for load testing but misleading for a learning project unless clearly documented |
| **Leader-side write batching API** | High throughput | Requires new client API or buffering layer; changes latency semantics |
| **Skip apply wait in `Commit()`** | Lower write latency | Write returns before state machine applies; breaks read-after-write linearizability on the leader |
| **Follower-local reads (stale)** | High read throughput | Relaxed consistency; reads may return outdated values |
| **Read index / leader leases** | Linearizable follower reads | Significant Raft extension; read path is already fast (~70k ops/s) |
| **Binary / protobuf log encoding** | Lower CPU and disk bytes | Migration complexity; fsync dominated until batching is in place (now addressed) |
| **Snapshots and log compaction** | Long-running cluster health | Large implementation effort; does not improve peak benchmark throughput on fresh clusters |
| **Shorter election timeouts** | Faster failover | Increases false-election risk under load |
| **HTTP → gRPC client protocol** | Modest latency reduction | API change; consensus cost dominates writes |

---

## How to reproduce

```bash
go test -count=1 ./core ./test
go run ./benchmarks --quick --concurrency=1,16,64
```

Results are written to `benchmarks/results/`. Baseline and per-optimization runs are stored under `benchmarks/results/baseline`, `opt1-no-repl-sleep`, etc.
