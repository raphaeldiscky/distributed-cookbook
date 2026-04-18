package stock

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	redis "github.com/redis/go-redis/v9"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// The Lua script runs atomically in Redis's single-threaded command loop.
// No other command can interleave between GET and DECRBY.
//
//	KEYS[1] = stock key (e.g. "stock:1")
//	ARGV[1] = qty
//	Returns: new stock on success, -1 for out-of-stock, -2 for missing key.
const decrementLua = `
local cur = redis.call("GET", KEYS[1])
if not cur then return -2 end
local n = tonumber(cur)
local qty = tonumber(ARGV[1])
if n < qty then return -1 end
return redis.call("DECRBY", KEYS[1], qty)
`

// RedisLuaAdapter keeps the authoritative stock counter in Redis. The decrement
// is one round-trip; order rows are appended to Postgres after success.
//
// Dual-write caveat: the Postgres insert can fail after Redis has already
// decremented. That gap is exactly the problem the `outbox` recipe addresses.
type RedisLuaAdapter struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	script *redis.Script
}

// NewRedisLua constructs the Redis-Lua atomic-decrement adapter.
func NewRedisLua(pool *pgxpool.Pool, rdb *redis.Client) *RedisLuaAdapter {
	return &RedisLuaAdapter{
		pool:   pool,
		rdb:    rdb,
		script: redis.NewScript(decrementLua),
	}
}

// Name returns the adapter label used for metrics.
func (a *RedisLuaAdapter) Name() Adapter { return AdapterRedisLua }

// Decrement runs the atomic Lua script in Redis; the order row is a
// best-effort post-write into Postgres (the dual-write problem).
func (a *RedisLuaAdapter) Decrement(ctx context.Context, productID int64, qty int) (Result, error) {
	key := stockKey(productID)

	res, err := a.script.Run(ctx, a.rdb, []string{key}, qty).Int()
	if err != nil {
		return Result{}, fmt.Errorf("redis_lua: eval: %w", err)
	}

	switch res {
	case -2:
		return Result{}, domain.ErrProductNotFound
	case -1:
		return Result{}, domain.ErrOutOfStock
	default:
		// Positive return value = new stock after successful decrement; fall through
		// to the order insert below.
	}

	// Best-effort order insert. If this fails, Redis already decremented — the
	// dual-write problem. The `outbox` recipe shows how to close this gap.
	var orderID int64

	err = a.pool.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("redis_lua: insert order: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: res}, nil
}

// Seed upserts the product row in Postgres AND sets the authoritative stock key in Redis.
func (a *RedisLuaAdapter) Seed(ctx context.Context, productID int64, name string, stock int) error {
	// Redis is the source of truth for stock under this adapter. Postgres still
	// stores the product row (for name) and receives order inserts.
	if _, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, stock = EXCLUDED.stock
	`, productID, name, stock); err != nil {
		return fmt.Errorf("redis_lua: pg seed: %w", err)
	}

	if _, err := a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID); err != nil {
		return fmt.Errorf("redis_lua: pg reset orders: %w", err)
	}

	if err := a.rdb.Set(ctx, stockKey(productID), strconv.Itoa(stock), 0).Err(); err != nil {
		return fmt.Errorf("redis_lua: redis seed: %w", err)
	}

	return nil
}

// Stock returns the authoritative stock value from Redis (source of truth for this adapter).
func (a *RedisLuaAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	v, err := a.rdb.Get(ctx, stockKey(productID)).Int()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, domain.ErrProductNotFound
		}

		return 0, err
	}

	return v, nil
}

func stockKey(id int64) string { return fmt.Sprintf("stock:%d", id) }
