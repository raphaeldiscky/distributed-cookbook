<h1 align="center">Distributed Cookbook</h1>

<p align="center">
A hands-on cookbook of distributed-systems learning recipes in Go.
Each recipe is a small runnable program with a Grafana dashboard and a k6 load test.
</p>

## Why

System-design blog posts are dense. Production codebases are tangled. This
repo sits in the middle: **one concept per recipe**, enough code to run and
watch, enough observability to see _why_ the concept matters.

Recipe 1, `flashsale`, shows the classic overselling race condition: 200
units in stock, thousands of buyers, and three ways to avoid recording
250 orders. You run it, press the dashboard refresh button, and watch
`oversell_total` drop from 50 to 0 when you swap a single env var.

## Recipes

| Recipe                                       | Concept                                                                                                              |
| -------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| [`flashsale`](./recipes/flashsale/RECIPE.md) | Atomic stock decrement under high concurrency. Naive read-check-write vs. Postgres conditional UPDATE vs. Redis Lua. |

Full catalogue of planned recipes: **[docs/ROADMAP.md](./docs/ROADMAP.md)**.
How the cookbook is structured: **[docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)**.
How to contribute a recipe: **[docs/CONTRIBUTING.md](./docs/CONTRIBUTING.md)**.

## Quick start

Prerequisites: Docker, Go 1.25+, `task` (`brew install go-task/tap/go-task`).

```bash
# One-time
task install_tools                           # golang-migrate, formatters, k6 hint
cp .env.example .env                         # optional overrides

# Run recipe 1 end-to-end
task start_stack RECIPE=flashsale            # postgres + redis + LGTM stack
task migrate_up RECIPE=flashsale             # creates flashsale.* tables

RECIPE_FLASHSALE_ADAPTER=pg_cond \
  task run RECIPE=flashsale                  # starts HTTP server on :8081

# In another terminal, seed and load-test:
task load_test RECIPE=flashsale              # k6 against :8081

# Open Grafana at http://localhost:3000  (admin/admin)
```

Switch adapter and repeat to compare:

```bash
RECIPE_FLASHSALE_ADAPTER=naive    task run RECIPE=flashsale  # shows oversell > 0
RECIPE_FLASHSALE_ADAPTER=redis_lua task run RECIPE=flashsale # fastest
```

## Observability (out of the box)

- **Prometheus** (`:9090`) — metrics
- **Grafana** (`:3000`) — dashboards auto-provisioned from every recipe
- **Loki** (`:3100`) — logs (Alloy scrapes Docker stdout)
- **Tempo** (`:3200`) — traces
- **OTel Collector** (`:4318` OTLP HTTP) — ingests traces/metrics/logs from apps

Your Go app sends OTLP to `localhost:4318`. The collector routes traces to
Tempo, metrics to Prometheus, logs to Loki. Grafana has all three
datasources provisioned, with trace-to-logs and trace-to-metrics linking.

## Layout

```
distributed-cookbook/
├── recipes/                     # each recipe lives in its own subdir
│   └── flashsale/               # recipe 1
├── pkg/                         # shared infra packages (telemetry, pg, redis, …)
├── deployments/docker-compose/  # atomic infra files + stack files per recipe
├── docs/                        # ARCHITECTURE, CONTRIBUTING, ROADMAP
├── scripts/
└── taskfile.yml
```

See **[docs/ARCHITECTURE.md](./docs/ARCHITECTURE.md)** for the full tour.

## Technologies

- **Go 1.25+** · Echo v4 · pgx/v5 · go-redis/v9 · slog · Prometheus client
- **OpenTelemetry** (`otelecho`, `otelpgx`, `otelredis`)
- **Postgres 18 · Redis 8 · Kafka (recipe 2+)**
- **Grafana LGTM stack + Alloy**
- **k6** for load testing

## License

MIT — see `LICENSE`.

# distributed-cookbook
