# Monitoring

Prometheus + Grafana stack for live Raft observability.

## Quick start

Terminal 1: observatory (optionally starts monitoring stack)

```bash
go run ./observatory --no-browser --compose-up observatory/scenarios/leader-failure.json
```

Or interactively:

```bash
go run ./observatory --no-browser
```

Terminal 2 (if not using `--compose-up`):

```bash
docker compose -f monitoring/docker-compose.yml up
```

- Observatory UI: http://localhost:8080
- Grafana: http://localhost:3000/d/raft-observatory/raft-observatory
- Prometheus: http://localhost:9090

## Dynamic scrape targets

When you configure or start a cluster, the observatory writes `monitoring/targets.json`. Prometheus reloads this file every 5 seconds via file service discovery, so node count (3-9) is handled automatically.

## Metrics reference

See [docs/observability.md](../docs/observability.md) for the full metrics catalog, PromQL queries, dashboard layout, and alert rules.

## Verify manually

1. Start observatory and a 5-node cluster
2. Confirm Prometheus targets at http://localhost:9090/targets show all nodes up
3. Run a scenario and watch Grafana panels update (commit index, lag, election rate)
4. Kill the leader and confirm term spike and replication lag panels react

## Notes

- Disable per-node metrics when running ryanDB manually: `--metrics=false`
- Grafana anonymous access is enabled for local demos (Editor role for annotations)
- Observatory posts Grafana annotations at each scenario step boundary
