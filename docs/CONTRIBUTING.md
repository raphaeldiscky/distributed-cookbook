# Contributing a new recipe

This doc is the concrete checklist. Read
[ARCHITECTURE.md](./ARCHITECTURE.md) for _why_, and
[CONVENTIONS.md](./CONVENTIONS.md) for the hard rules you must follow.

---

## Required files

Given a new recipe `foo`:

```
recipes/foo/
├── RECIPE.md                                # the lesson: problem + demo
├── cmd/server/main.go                       # entrypoint
├── internal/…                               # your code
├── migrations/000001_init.{up,down}.sql     # schema for 'foo' namespace
├── grafana/foo.json                         # dashboard with uid:"foo"
└── loadtest/*.js                            # k6 script(s)

deployments/docker-compose/recipes/
└── foo.stack.yaml                           # include: the infra atoms foo needs
```

If `foo` needs an infra service the repo doesn't yet ship (e.g., etcd):

```
deployments/docker-compose/etcd.yaml         # new atomic infra file
```

Then `foo.stack.yaml` `include:`s it. Nothing else changes.

---

## Where does new code live?

This is the single most important decision you make when adding a recipe.
Follow this tree; it prevents `pkg/` from becoming a "utils" dumping
ground as the number of recipes grows.

```
          Is this code used by 2+ recipes?
                    │
            ┌───────┴───────┐
            NO              YES
            │               │
            ▼        Would it be byte-for-byte identical
  recipes/<name>/     when imported from each recipe?
  internal/...              │
  (default home)    ┌───────┴───────┐
                    NO              YES
                    │               │
                    ▼       Is it infrastructure/primitive
       recipes/<name>/         (not domain logic)?
       internal/... or             │
       a helper repo       ┌───────┴───────┐
                           NO              YES
                           │               │
                           ▼               ▼
                 recipes/<name>/     Is it stable — won't
                 internal/...          change per recipe?
                                          │
                                   ┌──────┴──────┐
                                   NO            YES
                                   │             │
                                   ▼             ▼
                          recipes/<name>/     pkg/<thing>/
                          internal/...       (shared package)
```

**All three gates must pass for `pkg/`.** If any fails, the code stays
recipe-local.

### Examples

| Scenario                                | Destination                                                                                                                                | Why                                                        |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------- |
| A pgx pool builder used by every recipe | `pkg/pgconn`                                                                                                                               | identical + infra + stable ✅                              |
| An OTel tracer setup                    | `pkg/telemetry`                                                                                                                            | identical + infra + stable ✅                              |
| The `Decrementer` interface             | `recipes/flashsale/internal/stock`                                                                                                         | domain-specific — will differ per recipe ❌ (wrong gate 2) |
| A `Stock` struct with fields            | `recipes/flashsale/internal/domain`                                                                                                        | domain type ❌                                             |
| A shared idempotency-key middleware     | `recipes/idempotency/internal/...` until a _second_ recipe genuinely needs it                                                              | speculative sharing wastes the abstraction budget          |
| Error-response JSON helper              | either recipe-local or `pkg/httperror` — borderline. Keep recipe-local first, promote only if the second recipe needs the exact same shape |

**Rule of thumb:** when uncertain, keep it recipe-local. Promoting to
`pkg/` later costs ~15 minutes. Demoting a leaky shared abstraction
later costs hours.

---

## Naming

All naming rules live in **[CONVENTIONS.md](./CONVENTIONS.md)** — ports,
env vars, Postgres schemas, metric namespaces, Go packages, dashboard
UIDs. Consult that file before you name anything.

Short version: recipe name is lowercase kebab-case (`service-mesh-cilium`);
everything derived from it swaps hyphens for underscores where the target
language/system requires (`service_mesh_cilium`).

---

## Port allocation

Claim the next unused port in
**[CONVENTIONS.md → Port allocation](./CONVENTIONS.md#1-port-allocation-living-registry)**.
Update that table in the same PR that adds your recipe.

---

## Metrics + dashboard

See **[CONVENTIONS.md § 2 and § 6](./CONVENTIONS.md#2-prometheus-metric-namespace)**
for the namespace/subsystem rule and dashboard placement.

Non-negotiable: your recipe must emit **at least one metric that encodes
the correctness claim** and that metric must be the hero panel on the
dashboard. For flashsale it's `cookbook_flashsale_oversell_total`. A
recipe without a correctness metric has no way to prove its lesson
landed.

---

## k6 script

- Read knobs from `__ENV`: `CONCURRENT_USERS`, `DURATION`, `RPS_CAP`,
  plus any recipe-specific ones.
- Provide a `setup()` that prepares the world (`POST /seed` etc).
- Document the "hero scenario" (the combination of knobs that demonstrates
  the concept most clearly) at the top of `RECIPE.md`.

---

## Taskfile

No edits needed for the common case — `task run RECIPE=foo`,
`task migrate_up RECIPE=foo`, `task start_stack RECIPE=foo`, and
`task load_test RECIPE=foo` all resolve via the `RECIPE=` variable.

If your recipe has exotic commands (e.g., leader election requires
launching 3 binaries), add recipe-specific sub-targets under
`taskfile.yml` using your recipe name as the prefix
(`leader_election_run_3`, etc.).

---

## Pull-request checklist

- [ ] `go build ./...` passes.
- [ ] `task lint` passes.
- [ ] Recipe runs locally end-to-end: `start_stack` → `migrate_up` →
      `run` + seed → `load_test` → dashboard shows the correctness
      metric changing.
- [ ] Naming follows **[CONVENTIONS.md](./CONVENTIONS.md)**.
- [ ] New `pkg/` code passes all three gates in the decision tree above.
      If not, it stays recipe-local.
- [ ] **[CONVENTIONS.md → Port allocation](./CONVENTIONS.md#1-port-allocation-living-registry)**
      table updated to claim your port.
- [ ] **[docs/README.md → Current recipes](./README.md)** table gains a row.
- [ ] **[docs/ROADMAP.md](./ROADMAP.md)** entry (if one existed) is
      marked complete.
- [ ] If you introduced a deliberate deviation from an ADR in
      **[DECISIONS.md](./DECISIONS.md)**, that file is updated with a
      "Revisit trigger" note first.
