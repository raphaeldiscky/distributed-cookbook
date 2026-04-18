# Decisions — what we chose _not_ to use, and why

This is a lightweight ADR (Architecture Decision Record) log. Each entry
names a popular option we **deliberately didn't adopt**, and the reason
it loses to our current choice at this repo's scale and goals.

The goal is to short-circuit the inevitable drive-by PR that says "why
aren't we using X?" — every major non-choice has an answer here.

Format per entry:

- **Chose:** what we do
- **Rejected:** the obvious alternative
- **Reason:** one paragraph, honest

If you disagree with an entry, please open a PR editing this file _first_
(with rationale). Changing the underlying implementation without updating
this file is how architectural drift happens.

---

## 1. DI: manual wiring, not Wire or Fx

- **Chose:** explicit constructors in `recipes/<name>/cmd/server/main.go`
- **Rejected:** `google/wire` (code gen), `uber-go/fx` (reflection DI)
- **Reason:** our composition roots are ~40 lines each. A learner can
  read them top-to-bottom and see every dependency in five seconds.
  Wire adds a build step and a `wire.go` to maintain; Fx adds reflective
  magic at runtime. Both hide the very mechanism we're trying to teach.

## 2. Database access: pgx directly, not GORM / ent / sqlc

- **Chose:** `jackc/pgx/v5` with raw SQL strings in repository methods
- **Rejected:** GORM (ORM), ent (schema-first), sqlc (codegen)
- **Reason:** raw SQL is **pedagogically the whole point** in several
  recipes. The fix in flashsale is literally the SQL statement
  `UPDATE ... WHERE stock >= qty`; if that's hidden inside a query
  builder or generated code, the lesson evaporates. Recipes teaching
  consistency patterns (outbox, CDC, saga) will benefit from the same
  visibility. pgx also exposes features (transactions with isolation
  levels, COPY, notifications) that ORMs abstract away.

## 3. Config: stdlib `os.Getenv`, not Viper or envconfig

- **Chose:** ~20 lines in `pkg/config/shared.go` using `os.LookupEnv`
- **Rejected:** `spf13/viper`, `kelseyhightower/envconfig`
- **Reason:** we have six env vars at the shared level and a handful
  more per recipe. Viper is a 300-line dependency for something solvable
  in 20 lines. envconfig is lighter but still adds tags-as-DSL. At
  50+ config keys it would be worth revisiting — we're nowhere near
  that.

## 4. Logging: stdlib `log/slog`, not zap / zerolog / logrus

- **Chose:** stdlib `slog` with a custom `traceHandler` in `pkg/logger`
- **Rejected:** `uber-go/zap`, `rs/zerolog`, `sirupsen/logrus`
- **Reason:** slog is the standard library's structured logger since
  Go 1.21 — no dependency, good enough performance for a learning repo,
  and our custom handler for trace-id enrichment is 20 lines. Picking
  a third-party logger would require explaining _why_ and introduce a
  transitive dep every recipe inherits.

## 5. HTTP framework: Echo v4, not Gin / stdlib / chi

- **Chose:** `labstack/echo/v4`
- **Rejected:** Gin (more popular), stdlib `net/http` (no dep), chi
- **Reason:** honestly, all four would work. Echo won on two axes:
  (a) the reference repo `go-micro-commerce` uses it, so patterns
  transfer between repos; (b) `otelecho` middleware is polished and
  routes trace context correctly. Gin is roughly equivalent in
  popularity but requires switching to `otelgin`; stdlib with Go 1.22's
  `ServeMux` is tempting and may become the default in a later refactor,
  but Echo's bind/validate/error-handler story is currently friendlier.

## 6. Module structure: single `go.mod`, not multi-module workspace

- **Chose:** one `go.mod` at repo root, everything inside
- **Rejected:** `go.work` with `pkg/` and each `recipes/<name>/` as
  separate modules
- **Reason:** multi-module workspaces earn their keep when different
  modules have different dependency sets or release cadences. Our
  recipes all share the same deps (pgx, echo, otel…) and ship together
  with the repo itself. `go get`, `go build ./...`, and IDE tooling are
  simpler with a single module. Go's `internal/` visibility still gives
  us the isolation we need between recipes.

## 7. Compose layout: atomic files with `include:`, not one mega `docker-compose.yml`

- **Chose:** one service per Compose file, recipes combine via `include:`
- **Rejected:** a single `docker-compose.yml` with profiles; per-recipe
  full compose files that duplicate infra
- **Reason:** atomic files let a new recipe add only what it needs
  (`recipes/outbox.stack.yaml` pulls postgres + kafka + monitoring).
  Profiles would force every recipe's profile labels to be declared in
  one shared file — violating add-files-don't-edit. The `include:`
  directive (Compose v2.20+) gives us the composition we want without
  duplication.

## 8. Image versions: pinned, not `:latest`

- **Chose:** explicit tags (`postgres:18.3-alpine3.22`,
  `grafana/grafana:13.0.1`, …)
- **Rejected:** `:latest` or minor-only pins
- **Reason:** a learning repo has to be reproducible. If a learner runs
  `task start_stack` six months from now and Grafana changed its
  provisioning schema, the dashboards stop loading and the demo breaks
  silently. Pinning costs a PR every few months to bump; `:latest` costs
  mysterious failures.

## 9. Testing: `pgxmock` + `miniredis` for unit, testcontainers only if needed

- **Chose:** in-memory fakes for unit tests; real Compose for end-to-end
- **Rejected:** testcontainers for every test
- **Reason:** unit tests should be millisecond-fast so learners can
  iterate. `pgxmock` and `miniredis` give deterministic hermetic tests
  without spinning up containers. End-to-end coverage is provided by the
  k6 demo against the real stack — that's where the "does it actually
  work" question gets answered.

## 10. Grafana dashboards: file-provisioned, not HTTP API

- **Chose:** `scripts/sync-dashboards.sh` copies JSON into a provisioned
  directory Grafana polls every 10s
- **Rejected:** calling Grafana's HTTP API to upload dashboards
- **Reason:** provisioning is declarative and survives Grafana restarts
  with no extra work. The HTTP API requires auth, ordering
  (datasources-before-dashboards), and failure handling. A
  three-line shell script beats all of that.

## 11. Load testing: k6, not Locust / Gatling / Vegeta

- **Chose:** Grafana k6 with `--out experimental-prometheus-rw`
- **Rejected:** Locust (Python), Gatling (Scala/JVM), Vegeta (Go lib)
- **Reason:** k6 writes directly to Prometheus via remote_write, so
  client-side metrics show up next to server-side metrics on the same
  Grafana dashboard. It's also Grafana Labs' own tool — first-class
  integration. JS scripting is more readable than Vegeta's attack-config
  files for ramping scenarios. Gatling and Locust are great tools but
  bring heavier runtimes into a Go-first repo.

## 12. Postgres: schema-per-recipe, not database-per-recipe

- **Chose:** one database `cookbook_db`, each recipe owns a schema
  `<recipe>`
- **Rejected:** spawning a new Postgres container or `CREATE DATABASE`
  per recipe
- **Reason:** container-per-recipe doesn't scale (ports, memory) and
  obscures the learning point. Database-per-recipe inside one container
  fragments migrations tooling. Schema-per-recipe gives us namespace
  isolation with one `golang-migrate` command and one connection string.

---

## When to revise a decision

If a recipe genuinely needs something on this "rejected" list, update
the relevant entry with a "Revisit trigger" note explaining what would
change our mind. The rule is: the decision isn't immutable, but changing
it requires updating the doc first.
