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

Select one at startup:

```
RECIPE_FLASHSALE_ADAPTER=naive  task run RECIPE=flashsale
RECIPE_FLASHSALE_ADAPTER=pg_cond  task run RECIPE=flashsale
RECIPE_FLASHSALE_ADAPTER=redis_lua task run RECIPE=flashsale
```

## The demo

1. `task start_stack RECIPE=flashsale` — brings up Postgres, Redis, and the full observability stack.
2. `task migrate_up RECIPE=flashsale` — creates the `flashsale` schema.
3. `RECIPE_FLASHSALE_ADAPTER=naive task run RECIPE=flashsale` — start the server.
4. `task load_test RECIPE=flashsale CONCURRENT_USERS=5000 PRODUCT_COUNT=1 INITIAL_STOCK=200 DURATION=15s`.
5. Open <http://localhost:3000> → Recipes → "Flashsale — Oversell Demo".
   - **Oversell count** panel goes > 0. Stock gauge dips negative. You just oversold.
6. `^C`, then rerun with `RECIPE_FLASHSALE_ADAPTER=pg_cond` — oversell stays at 0.
7. Repeat with `redis_lua` — oversell stays at 0 and the latency panel shows Redis wins.

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
| POST | `/checkout` | `{product_id, qty}` | Try to buy. `200` with `stock_remaining`, or `409 out_of_stock`, or `404 product_not_found`. |
| POST | `/seed` | `{product_id, name?, stock}` | Reset stock for a product. Used by the k6 `setup()` hook. |
| GET | `/stock/:id` | — | Current stock (reads source-of-truth for active adapter). |
| GET | `/metrics` | — | Prometheus. |
| GET | `/healthz` | — | Liveness. |

## Metrics

- `cookbook_flashsale_oversell_total` — the hero metric. Must be zero for correct adapters.
- `cookbook_flashsale_checkout_attempts_total{adapter,outcome}` — rate per outcome.
- `cookbook_flashsale_checkout_latency_seconds` — histogram per adapter.
- `cookbook_flashsale_stock_remaining{product_id}` — gauge sampled after each checkout.
