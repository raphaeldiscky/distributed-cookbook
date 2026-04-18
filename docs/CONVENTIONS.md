# Conventions — the hard rules

This file is the **single source of truth** for things every recipe must
comply with. When ARCHITECTURE.md, CONTRIBUTING.md, or a RECIPE.md seem
to disagree with this file, this file wins. If you change a rule here,
search for stale references elsewhere.

---

## 1. Port allocation (living registry)

Each recipe's HTTP server claims the next unused port starting at 8081.
Edit this table in the same PR that adds the recipe — this is how we
prevent runtime collisions.

| Port | Recipe      | Status |
| ---: | ----------- | :----: |
| 8081 | `flashsale` |   ✅   |
| 8082 | _unclaimed_ |   —    |
| 8083 | _unclaimed_ |   —    |
| 8084 | _unclaimed_ |   —    |
| 8085 | _unclaimed_ |   —    |

Infrastructure ports (not per recipe):

|  Port | Service                     | Notes                                                         |
| ----: | --------------------------- | ------------------------------------------------------------- |
|  5433 | Postgres                    | remapped from 5432 to avoid collision with host Postgres      |
|  6379 | Redis                       |                                                               |
|  3000 | Grafana                     | admin/admin                                                   |
|  9090 | Prometheus                  |                                                               |
|  3100 | Loki                        |                                                               |
|  3200 | Tempo                       |                                                               |
|  4317 | Tempo OTLP gRPC             | direct (bypasses collector)                                   |
|  4318 | OTel Collector OTLP HTTP    | **apps send here**                                            |
|  8889 | OTel Collector → Prometheus | scraped                                                       |
| 12345 | Alloy                       | admin UI                                                      |
|  6565 | k6                          | Prometheus endpoint (when `--out=experimental-prometheus-rw`) |

If your recipe needs an infra service not yet in the repo (etcd, Envoy,
Consul…), add it here with its port.

---

## 2. Prometheus metric namespace

All recipe-emitted metrics must use:

```
Namespace: "cookbook"
Subsystem: "<recipe>"   (underscores, no hyphens)
Name:      "<metric_name>"
```

So a metric ends up as `cookbook_<recipe>_<metric_name>`, e.g.
`cookbook_flashsale_oversell_total`. This prevents collisions across
recipes sharing one Prometheus instance and keeps PromQL queries
predictable.

**At least one metric in every recipe must encode the recipe's
correctness claim** — the hero number on the Grafana dashboard. For
flashsale it's `oversell_total`.

Construction uses an injected `prometheus.Registerer` (no globals):

```go
// recipes/<name>/internal/metrics/metrics.go
func New(reg prometheus.Registerer) *Metrics {
    m := &Metrics{
        Thing: prometheus.NewCounter(prometheus.CounterOpts{
            Namespace: "cookbook",
            Subsystem: "<recipe>",
            Name:      "thing_total",
            Help:      "…",
        }),
    }
    reg.MustRegister(m.Thing)
    return m
}
```

---

## 3. Postgres schema

Each recipe owns a schema named after itself (underscored form if the
recipe name has hyphens):

```sql
-- recipes/<name>/migrations/000001_init.up.sql
CREATE SCHEMA IF NOT EXISTS <recipe>;

CREATE TABLE IF NOT EXISTS <recipe>.my_table ( … );
```

**Always schema-qualify table names** (`<recipe>.my_table`, not just
`my_table`) in both migrations and application queries. Do _not_ rely on
`SET search_path` inside the migration file — it only lasts for that
session.

One Postgres container is shared across all recipes. Do not create
per-recipe databases.

---

## 4. Env var naming

| Scope                | Pattern                 | Example                                                    |
| -------------------- | ----------------------- | ---------------------------------------------------------- |
| Shared (all recipes) | unprefixed              | `POSTGRES_DSN`, `REDIS_ADDR`, `OTLP_ENDPOINT`, `LOG_LEVEL` |
| Recipe-specific      | `RECIPE_<UPPER_NAME>_*` | `RECIPE_FLASHSALE_ADAPTER`, `RECIPE_FLASHSALE_PORT`        |

Hyphens in the recipe name become underscores in the env-var form:
`leader-election` → `RECIPE_LEADER_ELECTION_*`.

Config loaded via stdlib `os.Getenv` with defaults in
`recipes/<name>/internal/config/config.go`. Do **not** introduce Viper or
envconfig (see [DECISIONS.md](./DECISIONS.md)).

---

## 5. Naming: directories, Go packages, files

| Thing                          | Form                              | Example                                                                                  |
| ------------------------------ | --------------------------------- | ---------------------------------------------------------------------------------------- |
| Recipe directory               | lowercase kebab-case              | `recipes/service-mesh-cilium/`                                                           |
| Go package name                | lowercase, underscores allowed    | `package service_mesh_cilium`                                                            |
| Import path                    | kebab-case preserved              | `github.com/raphaeldiscky/distributed-cookbook/recipes/service-mesh-cilium/internal/...` |
| Env var prefix                 | `RECIPE_` + UPPER_UNDERSCORE      | `RECIPE_SERVICE_MESH_CILIUM_*`                                                           |
| Postgres schema                | lowercase underscores             | `service_mesh_cilium`                                                                    |
| Prometheus subsystem           | lowercase underscores             | `service_mesh_cilium`                                                                    |
| Grafana dashboard file         | `<recipe>.json` (kebab preserved) | `service-mesh-cilium.json`                                                               |
| Grafana dashboard `uid`        | matches filename basename         | `"uid": "service-mesh-cilium"`                                                           |
| Docker Compose stack file      | `<recipe>.stack.yaml`             | `recipes/service-mesh-cilium.stack.yaml`                                                 |
| Container names (shared infra) | `cookbook-<service>`              | `cookbook-postgres`, `cookbook-redis`                                                    |

---

## 6. Grafana dashboard

- One JSON per recipe at `recipes/<name>/grafana/<name>.json`.
- `"uid": "<name>"` matches the filename basename.
- **Panel 1** should hero-visualize the recipe's correctness claim
  (stat or gauge). For flashsale that's `cookbook_flashsale_oversell_total`.
- `scripts/sync-dashboards.sh` (runs as a dep of `start_stack` and
  `start_monitoring`) copies every `recipes/*/grafana/*.json` into the
  provisioned dashboards dir. Grafana polls that dir every 10s.

---

## 7. Recipe Compose stack file

Every recipe has `deployments/docker-compose/recipes/<name>.stack.yaml`
using Compose's `include:` directive to reference the atomic infra files
it needs:

```yaml
# deployments/docker-compose/recipes/<name>.stack.yaml
include:
  - path: ../postgres.yaml
  - path: ../redis.yaml
  - path: ../monitoring.yaml
  # add ../kafka.yaml, ../etcd.yaml, etc. as needed
```

**Do not** include `../network.yaml` — the shared bridge network is
created by `task start_network` (a dep of `start_stack`) via plain
`docker network create`. Including it conflicts with other files that
declare the network as external.

---

## 8. Docker network

All containers attach to the external bridge network
`distributed-cookbook`. It's created once by:

```bash
docker network create distributed-cookbook
# or
task start_network
```

Each infra file declares it as `external: true` at the top level; each
service references it in `networks:`.

---

## 9. Go package layout inside a recipe

Rigid on purpose. Deviations should have a reason documented in RECIPE.md.

```
recipes/<name>/
├── RECIPE.md
├── cmd/server/main.go              # composition root only — no logic here
├── internal/
│   ├── config/config.go            # stdlib env loader
│   ├── domain/                     # domain types and typed errors
│   ├── <operation>/                # the port + adapters (e.g. stock/, ratelimit/)
│   ├── metrics/metrics.go          # constructor-injected registry
│   └── handler/<name>.go           # thin HTTP binding
├── migrations/000001_init.up.sql
├── grafana/<name>.json
└── loadtest/*.js
```
