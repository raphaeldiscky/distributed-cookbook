package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	redis "github.com/redis/go-redis/v9"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// RedisAtomicAdapter claims stock with a bare DECRBY and compensates with
// INCRBY when the result goes negative.
//
// DECRBY is atomic, so the total number of successful claims is bounded exactly
// and this adapter never oversells. The lesson is that an atomic primitive is
// not an atomic operation: Redis has no decrement-if-at-least-n, so the
// condition has to be checked after the fact and undone, which leaves a window
// RedisLuaAdapter does not have.
//
// Two costs follow from that window. A crash between the DECRBY and the INCRBY
// leaks the units permanently, so the sale undersells. And while the
// compensation is in flight the counter reads negative, so concurrent buyers see
// a value that was never true. Lua exists to collapse both steps into one.
type RedisAtomicAdapter struct {
	pool *pgxpool.Pool
	rdb  *redis.Client
}

// NewRedisAtomic constructs the DECRBY-with-compensation adapter.
func NewRedisAtomic(pool *pgxpool.Pool, rdb *redis.Client) *RedisAtomicAdapter {
	return &RedisAtomicAdapter{pool: pool, rdb: rdb}
}

// Kind returns the repository kind used for metrics labels.
func (a *RedisAtomicAdapter) Kind() Kind { return KindRedisAtomic }

// Decrement claims with DECRBY and gives the units back if it overshot.
func (a *RedisAtomicAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	key := stockKey(productID)

	// Distinguishing a missing product from a zero-stock one needs its own look,
	// because DECRBY happily creates the key at -qty if it does not exist.
	exists, err := a.rdb.Exists(ctx, key).Result()
	if err != nil {
		return Result{}, fmt.Errorf("redis_atomic: exists: %w", err)
	}

	if exists == 0 {
		return Result{}, domain.ErrProductNotFound
	}

	remaining, err := a.rdb.DecrBy(ctx, key, int64(qty)).Result()
	if err != nil {
		return Result{}, fmt.Errorf("redis_atomic: decrby: %w", err)
	}

	if remaining < 0 {
		// We took units that were not there. Put them back. A crash here loses
		// them for good, which is this adapter's whole point.
		if _, err := a.rdb.IncrBy(ctx, key, int64(qty)).Result(); err != nil {
			return Result{}, fmt.Errorf("redis_atomic: compensating incrby: %w", err)
		}

		return Result{}, domain.ErrOutOfStock
	}

	// Same dual-write gap as redis_lua: Redis has already committed the claim.
	var orderID int64

	err = a.pool.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("redis_atomic: insert order: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: int(remaining)}, nil
}

// Seed writes the Redis counter, upserts the Postgres product row so the
// GET /stock and oversell bookkeeping have something to read, and clears prior
// orders for this product.
func (a *RedisAtomicAdapter) Seed(
	ctx context.Context,
	productID int64,
	name string,
	stock int,
) error {
	if err := a.rdb.Set(ctx, stockKey(productID), stock, 0).Err(); err != nil {
		return fmt.Errorf("redis_atomic: set: %w", err)
	}

	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, stock = EXCLUDED.stock
	`, productID, name, stock)
	if err != nil {
		return fmt.Errorf("redis_atomic: seed product: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the authoritative counter from Redis.
func (a *RedisAtomicAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	v, err := a.rdb.Get(ctx, stockKey(productID)).Int()
	if errors.Is(err, redis.Nil) {
		return 0, domain.ErrProductNotFound
	}

	if err != nil {
		return 0, fmt.Errorf("redis_atomic: get: %w", err)
	}

	return v, nil
}
