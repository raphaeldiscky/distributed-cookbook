# Architecture

Distributed-cookbook is a **monorepo of self-contained distributed-systems
learning recipes**. Each recipe is a small Go program that demonstrates one
concept, is load-testable with k6, and is observable in Grafana.

## Design principles

1. **Small and focused.** A learner should be able to read one recipe in
   30 minutes and understand the lesson.
2. **No frameworks or magic.** Manual dependency wiring, raw SQL, stdlib
   env loading. Learners see every mechanism.
3. **Add-files, don't edit.** Adding a new recipe creates files. It does
   not modify anything under another recipe, and edits a shared file only
   when introducing genuinely new infrastructure.
4. **Observable by default.** Every recipe ships its own Grafana dashboard
   and at least one Prometheus metric that encodes its correctness claim.
5. **Reusable infrastructure.** One Postgres, one Redis, one monitoring
   stack, shared across recipes.

## Top-level layout

```
distributed-cookbook/
├── pkg/              # Shared Go libraries (telemetry, pg, redis, http, config, logger)
├── services/         # Universal HTTP services reused across recipes
│   └── <name>/
│       ├── README.md
│       ├── Dockerfile
│       ├── cmd/server/main.go
│       └── internal/…
├── recipes/          # One directory per recipe
│   └── <name>/
│       ├── RECIPE.md
│       ├── cmd/server/main.go     # only when the recipe's own server IS the lesson
│       ├── internal/…              # recipe-local code (when needed)
│       ├── migrations/             # only when the recipe owns a Postgres schema
│       ├── grafana/<name>.json
│       └── loadtest/*.js
├── deployments/
│   ├── docker-compose/             # Mode A infra (compose recipes)
│   │   ├── network.yaml
│   │   ├── postgres.yaml, redis.yaml, kafka.yaml, …
│   │   ├── monitoring.yaml + monitoring/config/…
│   │   └── recipes/<name>.stack.yaml
│   ├── k8s/                        # Mode B infra (K8s recipes)
│   │   ├── services/<name>/        # reusable workload bases (kustomize)
│   │   └── recipes/<name>/         # recipe-specific gateways/meshes/etc.
│   └── helm/values/                # Helm values for K8s controllers
├── Tiltfile                         # Local-K8s orchestrator (Mode B)
├── docs/             # THIS dir
├── scripts/
└── taskfile.yml      # task runner targets for both modes
```

## Shared packages (`pkg/`)

Only infrastructure primitives live here. Rule: if code would be
byte-for-byte identical when imported from two recipes, it goes in
`pkg/`. Domain-specific code never appears here. See
[CONTRIBUTING.md → Where does new code live?](./CONTRIBUTING.md#where-does-new-code-live)
for the decision tree.

## Universal services (`services/`)

Reusable HTTP workloads that recipes compose to demonstrate concepts
*around* them (gateway routing, mesh policy, distributed tracing, …).
A workload qualifies for `services/` when **2+ recipes can use it
as-is**; otherwise it stays under `recipes/<name>/cmd/`. See
[services/README.md](../services/README.md) for the contract every
universal service follows (env vars, endpoints, metric namespace).

| Package          | What it gives you                                                         |
| ---------------- | ------------------------------------------------------------------------- |
| `pkg/config`     | `LoadShared()` → `Shared{PostgresDSN, RedisAddr, OTLPEndpoint, LogLevel}` |
| `pkg/logger`     | `slog.Logger` with trace-id enrichment from the current OTel span         |
| `pkg/telemetry`  | OTel tracer provider, OTLP HTTP exporter, graceful shutdown               |
| `pkg/pgconn`     | pgxpool constructor wired with `otelpgx` tracing                          |
| `pkg/redisconn`  | single-node Redis client with OTel hooks                                  |
| `pkg/httpserver` | Echo server pre-wired with `otelecho` + `/metrics` + `/healthz`           |

## Atomic, composable infrastructure

Each infra service is **one self-contained Compose file**. Recipes combine
them via Compose's `include:` directive:

```yaml
# deployments/docker-compose/recipes/flashsale.stack.yaml
include:
  - path: ../postgres.yaml
  - path: ../redis.yaml
  - path: ../monitoring.yaml
```

Running `task start_stack RECIPE=flashsale` boots exactly what the recipe
needs. A future recipe that wants Postgres + Kafka + monitoring ships its
own `<name>.stack.yaml` with different `include:` paths. **No shared file
grows** when a new recipe is added.

### Adding new infrastructure

If a future recipe needs etcd, Envoy, Consul, Vault, or any service we
don't ship yet, the pattern is:

1. Write a new `deployments/docker-compose/<service>.yaml` (one service,
   its volumes, healthcheck, attach to `distributed-cookbook` network).
2. Reference it from the recipe's `*.stack.yaml` via `include:`.

Every shared monitoring asset — Prometheus scrape config, OTel collector
pipeline, Grafana datasources — is already provisioned, so the new infra
usually gets observability for free.

## Recipe conventions

The hard rules (port allocation, metric namespace, schema naming, env var
prefix, file naming) live in **[CONVENTIONS.md](./CONVENTIONS.md)**. The
short version:

- **Recipe name = recognizable concept or scenario** (`flashsale`,
  `outbox`, `saga`, `service-mesh-cilium`), not the technique name.
- Each recipe owns its port, Postgres schema, metric subsystem, env var
  prefix, and Grafana dashboard UID — all derived from the recipe name.
- `scripts/sync-dashboards.sh` copies every `recipes/*/grafana/*.json`
  into the provisioned dashboards dir; Grafana auto-discovers them.

## Deployment modes

The cookbook supports two deployment modes; each recipe picks one.

### Mode A — host Go binary + docker-compose (default)

Used by `flashsale` and most planned recipes. The Go server runs on the
host (via `task run RECIPE=<name>`, hot-reloaded by air) and connects
to infra running in docker-compose (Postgres, Redis, the LGTM monitoring
stack, …). Composition root is `recipes/<name>/cmd/server/main.go`.

### Mode B — kind cluster + Helm + Tilt

Used by `api-gateway` (the first such recipe) and planned for
`service-mesh-cilium`, `service-mesh-istio`, `operators-controllers`,
`kubernetes-basics`, `autoscaling`, `helm-kustomize`,
`chaos-engineering`. The recipe's workloads are containerised and
deployed to a local kind cluster. Gateway/mesh controllers and
observability are installed via Helm; manifests live under
`deployments/k8s/recipes/<name>/` (kustomize) and
`deployments/helm/values/`. A repo-root `Tiltfile` orchestrates
everything. Monitoring runs **inside** the kind cluster
(`kube-prometheus-stack`).

Mode-B recipes typically include reusable bases from
`deployments/k8s/services/<name>/` rather than ship their own backend
manifests — see "Universal services" above.

Each RECIPE.md declares which mode it uses up front.

## Design patterns used

Every pattern below is earning its keep on a specific problem. The
mapping between pattern → code makes it easy to learn the pattern _by
reading the code that implements it_.

### Hexagonal / Ports & Adapters

The domain defines a narrow interface (the "port"); one or more structs
implement it (the "adapters"). The handler depends only on the port.
This is _why_ we can swap strategies with an env var.

- Port: `type Decrementer interface` at `recipes/flashsale/internal/stock/decrementer.go:30`
- Adapters: `NaiveAdapter`, `PgCondAdapter`, `RedisLuaAdapter` in the same
  package (`naive.go`, `pg_cond.go`, `redis_lua.go`)
- Handler depends on the port, not the concrete:
  `recipes/flashsale/internal/handler/checkout.go` takes
  `stock.Decrementer` in its constructor

### Strategy Pattern (selected by Factory)

The three adapters are interchangeable strategies for the same operation.
A factory function picks the strategy at startup from config string.

- Factory: `stock.New(name, pool, rdb)` at
  `recipes/flashsale/internal/stock/decrementer.go:39`
- Selection wired in `recipes/flashsale/cmd/server/main.go`

### Manual Composition Root

All dependency wiring happens in one `main.go`. No DI framework — every
constructor takes its deps explicitly, and the root is the single place
that knows about concrete types.

- Composition root: `recipes/flashsale/cmd/server/main.go` (~40 lines, top-to-bottom)
- Shared helpers it composes: `pkg/config`, `pkg/logger`, `pkg/telemetry`,
  `pkg/pgconn`, `pkg/redisconn`, `pkg/httpserver`

### Typed Domain Errors

Sentinel errors (`domain.ErrOutOfStock`, `domain.ErrProductNotFound`) let
the handler branch on exact failure modes via `errors.Is`, without
reflection or error-message parsing. Idiomatic Go.

- Definitions: `recipes/flashsale/internal/domain/errors.go:7`
- Use site: `recipes/flashsale/internal/handler/checkout.go` in the
  `switch` on the decrement result — each branch maps a domain error to
  an HTTP status.

### No-Globals Metrics (constructor-injected Registry)

Every Prometheus counter/histogram is created inside a constructor that
receives a `prometheus.Registerer`. No package-level `var` declarations
— required by the `gochecknoglobals` lint, but also keeps tests isolated.

- Constructor: `metrics.New(reg)` at
  `recipes/flashsale/internal/metrics/metrics.go:17`
- Call site: `main.go` passes `prometheus.NewRegistry()` then hands the
  registry to both `metrics.New` and `promhttp.HandlerFor`

### Facade over the OTel SDK

The OpenTelemetry Go SDK has a lot of surface. `pkg/telemetry` hides the
ceremony behind two methods: `New(ctx, cfg)` and `Shutdown(ctx)`. After
`New`, the global tracer provider is installed and all instrumentation
libraries (`otelecho`, `otelpgx`, `otelredis`) pick it up transparently.

- Facade: `pkg/telemetry/telemetry.go:21` (`New`)
- Shutdown: `pkg/telemetry/shutdown.go`
- Exporter + resource + sampler wiring: `pkg/telemetry/tracing.go`

### Decorator Middleware Chain

Echo's middleware stack is a pure decorator pattern: each middleware
wraps the next handler. We use three decorators: `otelecho.Middleware`
(per-request span), `middleware.Recover` (panic-to-500), and an
ad-hoc Prometheus-latency decorator registered per-route.

- Chain: `pkg/httpserver/server.go:20` (`New`)

### Trace-aware Slog Handler

A small custom `slog.Handler` reads the OTel span from the request
context and injects `trace_id`/`span_id` into every log line. This is
what lets Grafana's derivedFields regex auto-link a log entry to its
trace in Tempo.

- Custom handler: `pkg/logger/logger.go:43` (`traceHandler`)
- Datasource wiring that consumes it:
  `deployments/docker-compose/monitoring/grafana/provisioning/datasources/datasources.yaml`

### Convention over Configuration (for discovery)

No registry file. No plugin manifest. The filesystem _is_ the registry.

- `task run RECIPE=foo` resolves because `recipes/foo/cmd/server/main.go`
  exists. No other step required.
- Grafana dashboard for recipe `foo` loads because
  `scripts/sync-dashboards.sh` copies `recipes/foo/grafana/foo.json`
  into the provisioned dir.

## Scalability notes

The architecture is designed for ~10–20 recipes. The three drift risks
below all have documented mitigations.

1. **`pkg/` drifting into a "utils" dumping ground.**
   Every new recipe will tempt contributors to "just promote this helper"
   into `pkg/`. Mitigation: the decision tree in
   [CONTRIBUTING.md → Where does new code live?](./CONTRIBUTING.md#where-does-new-code-live)
   requires all three gates (identical · infra/primitive · stable) to be
   met. When in doubt, keep it recipe-local.

2. **Silent collisions in ports, metric names, schemas.**
   Two recipes picking the same port, or `cookbook_<x>_foo_total` without
   a unique recipe subsystem, fail loudly at runtime but not at code
   review. Mitigation: the living registry in
   [CONVENTIONS.md](./CONVENTIONS.md) is the single source of truth.

3. **`cmd/server/main.go` boilerplate repeats.**
   Today ~40 lines of near-identical wiring appear per recipe. That's
   acceptable pedagogy — learners see every dep. If it becomes truly
   painful at 15+ recipes, factor a minimal `pkg/bootstrap.Run(cfg,
serviceName, register func(*echo.Echo))` **then**, not before.

## What makes a good recipe

- **One concept.** If you're teaching two things, split into two recipes.
- **A correctness claim encoded as a metric.** For flashsale, it's
  `oversell_total`. Without this, the demo doesn't prove anything.
- **A dramatic k6 scenario.** The "stock=200, 5000 users, 15 seconds"
  setup is the money shot — make sure there's one for your recipe.
- **A broken adapter alongside correct ones**, where it helps the lesson.
  Flashsale ships a `naive` adapter specifically so Grafana can visibly
  show the bug.
