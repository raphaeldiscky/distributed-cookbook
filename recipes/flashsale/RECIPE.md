# Recipe: Flash Sale (Prevent Overselling)

## The problem

200 units in stock. Thousands of buyers click "buy" within seconds.
Your naïve code records **250 orders**. You've oversold by 50 units.

This recipe shows *why* that happens and *three* ways to fix it — with a
Grafana dashboard so you can watch the bug appear and disappear.

## The setup

| Adapter | Mechanism | Correctness | Latency (expected) |
|---|---|---|---|
| `naive` | `SELECT` → check in Go → `UPDATE` with literal value | **broken** — two goroutines can read the same stock and both write the same new value | medium |
| `pg_cond` | `UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1` | correct — the WHERE clause and the write are atomic in one statement | medium |
| `redis_lua` | Lua `EVAL` with atomic `GET`+`DECRBY` inside Redis' single-threaded loop | correct — Redis guarantees atomicity | lowest |

All three adapters are **live on the same server**. Route per-request
via URL path — no restart needed to switch between them:

```bash
curl -X POST localhost:8081/checkout/naive     -d '{"product_id":1,"qty":1}' -H 'Content-Type: application/json'
curl -X POST localhost:8081/checkout/pg_cond   -d '{"product_id":1,"qty":1}' -H 'Content-Type: application/json'
curl -X POST localhost:8081/checkout/redis_lua -d '{"product_id":1,"qty":1}' -H 'Content-Type: application/json'
```

`POST /checkout` without a suffix uses whichever adapter
`RECIPE_FLASHSALE_ADAPTER` selects (default `pg_cond`).

> `task run` uses [air](https://github.com/air-verse/air) — edit any
> `.go` file under `recipes/flashsale/` or `pkg/`, save, and the server
> rebuilds and restarts (gracefully: in-flight requests drain first).

## The demo

1. `task start_stack RECIPE=flashsale` — brings up Postgres, Redis, and the full observability stack.
2. `task migrate_up RECIPE=flashsale` — creates the `flashsale` schema.
3. `task run RECIPE=flashsale` — start the server (one, stays running).
4. Load-test each adapter in turn (server keeps running between runs):

   ```bash
   ADAPTER=naive     CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
   ADAPTER=pg_cond   CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
   ADAPTER=redis_lua CONCURRENT_USERS=5000 INITIAL_STOCK=200 DURATION=15s task load_test RECIPE=flashsale
   ```

   Sequencing the runs (instead of running all three in parallel)
   gives each adapter its own clean Postgres/Redis pool bandwidth for
   an honest p99 comparison.

5. Open <http://localhost:3000> → Recipes → "Flashsale — Oversell Demo".
   - **Three hero tiles** (Oversells · naive / pg_cond / redis_lua) are
     visible from the moment the server starts — all green at 0 —
     because the server pre-touches every `(adapter, outcome)` label
     combination at startup. After the naive load test the naive tile
     goes red with thousands; pg_cond and redis_lua stay green at 0.
   - **Stock remaining** bar-gauge shows one horizontal bar per
     `(adapter, product)` combo — reds when oversold, greens when healthy.
   - **Latency panels** (p50/p95/p99) split by adapter — three
     coloured lines in each chart, redis_lua visibly lowest.
   - **k6 client RPS** draws the load-test curve (k6 remote_writes to
     Prometheus via the task's `--out experimental-prometheus-rw`).
   - **Recent checkouts** panel (TraceQL from Tempo) shows one span
     per checkout tagged with the `adapter` and `outcome` attributes.

## Why `redis_lua` is the fastest, but not always the best

The Redis Lua adapter keeps the authoritative stock counter in Redis. After
the decrement succeeds we `INSERT` the order row into Postgres — a separate
write that can fail *after* Redis has already decremented. This is the
**dual-write problem**.

If the Postgres insert fails, Redis has already given away the unit but no
order exists. That is exactly the problem the next recipe — `outbox` — fixes:
it writes the state change and the outbound event in a single Postgres
transaction, then a separate process publishes the event.

## HTTP surface

| Method | Path | Body | Purpose |
|---|---|---|---|
| POST | `/checkout/:adapter` | `{product_id, qty}` | Try to buy via the named adapter. `200` / `409 out_of_stock` / `404 product_not_found`. |
| POST | `/checkout` | same | Uses the default adapter (env `RECIPE_FLASHSALE_ADAPTER`, default `pg_cond`). |
| POST | `/seed` | `{product_id, name?, stock}` | Primes ALL adapters' state (PG row + Redis key). One call per product. |
| GET | `/stock/:adapter/:id` | — | Current stock via the named adapter's source of truth. |
| GET | `/stock/:id` | — | Via the default adapter. |
| GET | `/metrics` | — | Prometheus. |
| GET | `/healthz` | — | Liveness. |

## Metrics

Every metric carries an `adapter` label so Grafana panels split cleanly.

- `cookbook_flashsale_oversell_total{adapter}` — the hero metric. Must be zero for correct adapters.
- `cookbook_flashsale_checkout_attempts_total{adapter, outcome}` — rate per (adapter, outcome).
- `cookbook_flashsale_checkout_latency_seconds{adapter}` — histogram per adapter.
- `cookbook_flashsale_stock_remaining{adapter, product_id}` — gauge sampled after each successful checkout.
