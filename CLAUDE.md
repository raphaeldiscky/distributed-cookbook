# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A cookbook of small, self-contained distributed-systems learning recipes
in Go. Each recipe lives under `recipes/<name>/`, demonstrates one
concept, is load-testable with k6, and is observable in Grafana. Today
there's one recipe: `flashsale` (atomic stock decrement under
concurrency, with three adapters: `naive`, `pg_cond`, `redis_lua`).

The guiding principle is **legibility over cleverness** — every pattern
is earning its keep by making one specific aspect easier to understand
or swap. Before making architectural changes, read `docs/DECISIONS.md`
— a dozen common "why aren't we using X?" questions are pre-answered
there (no Wire, no GORM, no Viper, single `go.mod`, etc.).

## Where the authoritative docs live

**Read these before making non-trivial changes:**

- `docs/ARCHITECTURE.md` — structure, shared-package rules, design
  patterns used (each pointer includes file + line number)
- `docs/CONVENTIONS.md` — **single source of truth** for the hard rules:
  port allocation registry, metric namespace, Postgres schema naming,
  env var prefix, file naming. When other docs disagree, this wins.
- `docs/DECISIONS.md` — lightweight ADR log of deliberate non-choices.
  Don't propose replacing a listed non-choice without editing this
  file first.
- `docs/CONTRIBUTING.md` — includes the `pkg/` vs recipe-local decision
  tree (three gates: identical · infra/primitive · stable). This is the
  #1 risk mitigation against `pkg/` drift.
- `docs/ROADMAP.md` — 30+ future recipes, grouped by topic.

## Commands

The project uses [go-task](https://taskfile.dev). All common actions
are `task <target>`:

```bash
# First-time setup
task install_tools                            # installs golang-migrate, k6 (hint), linters

# Infrastructure (granular — reusable across recipes)
task start_network                            # create shared bridge
task start_postgres | start_redis | start_kafka
task start_monitoring                         # LGTM + Alloy + OTel Collector
task stop_all                                 # tear everything down

# Recipe-aware composites (require RECIPE= variable)
task start_stack    RECIPE=flashsale          # spins up everything the recipe's stack.yaml includes
task migrate_up     RECIPE=flashsale          # runs golang-migrate against the recipe's migrations/
task migrate_down   RECIPE=flashsale
task run            RECIPE=flashsale          # hot-reload loop via air (watches recipes/<name>/ + pkg/)
task stop_server    [PORT=8081]               # kill whoever is listening on the recipe's port
task load_test      RECIPE=flashsale          # k6 run recipes/flashsale/loadtest/checkout.js
task stop_stack     RECIPE=flashsale

# Quality
task lint                                     # gofumpt + goimports + golangci-lint --fix (chained via deps)
task test                                     # go test ./... -v
task deadcode                                 # dead-code detection
task security                                 # govulncheck

# Run a single Go test
go test ./recipes/flashsale/internal/stock -run TestPgCondAdapter_Decrement -v
```

**Recipe-specific env vars** use the prefix `RECIPE_<UPPER_NAME>_*`
(e.g. `RECIPE_FLASHSALE_ADAPTER=pg_cond`). Shared vars are unprefixed
(`POSTGRES_DSN`, `REDIS_ADDR`, `OTLP_ENDPOINT`, `LOG_LEVEL`). See
`.env.example`.

## High-level architecture

```
pkg/                    # Shared across every recipe. Only infra primitives live here.
  config, logger, telemetry, pgconn, redisconn, httpserver

recipes/<name>/         # One directory per learning concept.
  cmd/server/main.go    # Composition root — ~40 lines of explicit wiring, no DI framework.
  internal/…            # Recipe-local (Go enforces visibility). Typical layout:
    config/   domain/   <operation>/   metrics/   handler/
  migrations/           # golang-migrate SQL; own the schema `<name>`
  grafana/<name>.json   # dashboard auto-discovered via scripts/sync-dashboards.sh
  loadtest/*.js         # k6 scenario(s)

deployments/docker-compose/
  network.yaml, postgres.yaml, redis.yaml, kafka.yaml, monitoring.yaml
                        # Atomic files — ONE service each. Self-contained.
  monitoring/config/    # prometheus.yml, loki-config.yaml, tempo-config.yaml,
                        # otel-collector-config.yaml, alloy-config.river
  monitoring/grafana/provisioning/
                        # datasources + dashboards provisioning YAMLs
  recipes/<name>.stack.yaml
                        # Recipe's infra combo via Compose `include:` directive.
                        # This is the ONLY glue between recipes and infra.
```

**Key architectural patterns** (see `docs/ARCHITECTURE.md § Design
patterns used` for line-number pointers into real code):

- **Hexagonal / Ports & Adapters** — e.g. `stock.Decrementer` with three
  adapters selected by the `stock.New` factory. Recipes compare
  multiple implementations behind one interface — this is _the_ pattern
  the cookbook teaches.
- **Manual Composition Root** — all wiring in `cmd/server/main.go`, no
  DI framework. Learners see every dependency.
- **No-Globals Metrics** — every Prometheus metric constructed against
  an injected `prometheus.Registerer` (enforced by `gochecknoglobals` lint).
- **Variants pattern** — for comparison-style recipes, every
  variant-sensitive metric must carry an `adapter` label so Grafana
  panels split correctly across sequential runs.

## Critical conventions (will bite you)

1. **Postgres listens on host port `5433`** (not 5432) to avoid
   collision with a host-level Postgres. `POSTGRES_DSN` default in
   `pkg/config/shared.go` reflects this.
2. **On Linux, Prometheus reaches host apps via `host.docker.internal`
   with `extra_hosts: ["host.docker.internal:host-gateway"]`** — already
   set in `monitoring.yaml` for Prometheus. Without it, the
   `recipe-app` scrape target fails.
3. **Schema-per-recipe** inside one shared `cookbook_db`. Migrations
   `CREATE SCHEMA IF NOT EXISTS <recipe>` and all table references MUST
   be schema-qualified (`flashsale.products`, not `products`). Do not
   rely on `SET search_path` in migrations — the session scope is lost.
4. **Prometheus metric names** must be `cookbook_<recipe>_<name>`
   (namespace `cookbook`, subsystem `<recipe>`). Recipes that share one
   Prometheus instance collide otherwise.
5. **Recipe Compose stack files do NOT include `network.yaml`** — the
   external bridge is created by `task start_network` (a dep of
   `start_stack`) via plain `docker network create`. Including it
   triggers a "networks.X conflicts with imported resource" error.
6. **`scripts/sync-dashboards.sh` is a dep of `start_stack` and
   `start_monitoring`** — it copies `recipes/*/grafana/*.json` into the
   provisioned dashboard directory. If Grafana doesn't show a
   dashboard, run that script (or the Task target that depends on it).
7. **Docker images are pinned deliberately** (see `docs/DECISIONS.md
§ 8`). Do not switch to `:latest` tags.
8. **`pkg/` inclusion requires ALL THREE gates** (`docs/CONTRIBUTING.md`):
   (a) 2+ recipes import the exact same code, (b) it's
   infra/primitive, not domain, (c) it's stable. Default to
   recipe-local; promoting later is cheap, demoting is expensive.
9. **Graceful shutdown is load-bearing** — `recipes/<name>/cmd/server/main.go`
   installs a `signal.NotifyContext` on SIGINT/SIGTERM;
   `pkg/httpserver.Run` calls `srv.Shutdown(ctx)` with a 10s timeout;
   main's LIFO defers close Redis → Postgres → OTel last. Air's
   `kill_delay=3s` deliberately undercuts the 10s server timeout for
   dev iteration speed. **Do not remove the `http server stopped` log
   line** — it's the visible signal that graceful drain completed,
   especially when watching air rebuild cycles.

## When adding a new recipe

Follow `docs/CONTRIBUTING.md` — it has the required-files checklist,
naming rubric, port claim process, and PR checklist. Key moves:

1. Create `recipes/<name>/{RECIPE.md, cmd/server/main.go,
internal/..., migrations/, grafana/<name>.json, loadtest/*.js}`.
2. Create `deployments/docker-compose/recipes/<name>.stack.yaml` that
   `include:`s the infra atoms the recipe needs (postgres, redis,
   monitoring, kafka — add new atomic files for any new infra).
3. Claim a port in `docs/CONVENTIONS.md § 1` (starting at `:8081`).
4. Update `docs/README.md → Current recipes` table.
5. `task lint` + end-to-end smoke test must pass.

## Lint expectations

`golangci-lint` is strict (see `.golangci.yml`). Common gotchas:

- **`revive`** requires godoc comments on every exported symbol
  (functions, methods, constants, types).
- **`gochecknoglobals`** forbids package-level `var` declarations —
  pass things via constructors.
- **`nolintlint`** requires _rationale_ in every nolint directive:
  `//nolint:errcheck,gosec // reason here`.
- **`errcheck` + `gosec G104`** both flag unchecked errors. Instead of
  `nolint`, prefer one of two real-handling patterns:
  - **Deferred cleanup with a logger** — use `pkg/closer.LogOnError`:
    `defer closer.LogOnError(rdb, log, "redis")`. Preserves
    observability; no nolint needed.
  - **Cleanup inside a constructor error path** (no logger available) —
    use `errors.Join` to merge the Close error with the dominant one:
    `return nil, errors.Join(fmt.Errorf("...: %w", err), client.Close())`.
    See `pkg/redisconn/redisconn.go` for an example.

<!-- rtk-instructions v2 -->

# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:

```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)

```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (60-99% savings)

```bash
rtk cargo test          # Cargo test failures only (90%)
rtk go test             # Go test failures only (90%)
rtk jest                # Jest failures only (99.5%)
rtk vitest              # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk pytest              # Python test failures only (90%)
rtk rake test           # Ruby test failures only (90%)
rtk rspec               # RSpec test failures only (60%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)

```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)

```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)

```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
```

### Files & Search (60-75% savings)

```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%)
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)

```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)

```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)

```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands

```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category         | Commands                       | Typical Savings |
| ---------------- | ------------------------------ | --------------- |
| Tests            | vitest, playwright, cargo test | 90-99%          |
| Build            | next, tsc, lint, prettier      | 70-87%          |
| Git              | status, log, diff, add, commit | 59-80%          |
| GitHub           | gh pr, gh run, gh issue        | 26-87%          |
| Package Managers | pnpm, npm, npx                 | 70-90%          |
| Files            | ls, read, grep, find           | 60-75%          |
| Infrastructure   | docker, kubectl                | 85%             |
| Network          | curl, wget                     | 65-70%          |

Overall average: **60-90% token reduction** on common development operations.

<!-- /rtk-instructions -->
