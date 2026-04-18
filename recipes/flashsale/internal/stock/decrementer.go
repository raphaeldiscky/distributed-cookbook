// Package stock provides swappable adapters for atomic stock decrement.
package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	redis "github.com/redis/go-redis/v9"
)

// Adapter identifies which decrement strategy is active.
type Adapter string

// The three stock-decrement strategies this recipe ships.
const (
	AdapterNaive    Adapter = "naive"
	AdapterPgCond   Adapter = "pg_cond"
	AdapterRedisLua Adapter = "redis_lua"
)

// Result carries the post-decrement state back to the handler.
type Result struct {
	OrderID        int64
	StockRemaining int
}

// Decrementer atomically reduces stock by qty for productID and records an order.
// Returns domain.ErrOutOfStock if qty > current stock.
// Returns domain.ErrProductNotFound if productID is unknown.
type Decrementer interface {
	Decrement(ctx context.Context, productID int64, qty int) (Result, error)
	Seed(ctx context.Context, productID int64, name string, stock int) error
	Stock(ctx context.Context, productID int64) (int, error)
	Name() Adapter
}

// New returns the adapter selected by name.
// redis_lua requires rdb; others may pass nil.
func New(name Adapter, pool *pgxpool.Pool, rdb *redis.Client) (Decrementer, error) {
	switch name {
	case AdapterNaive:
		return NewNaive(pool), nil
	case AdapterPgCond:
		return NewPgCond(pool), nil
	case AdapterRedisLua:
		if rdb == nil {
			return nil, errors.New("stock: redis_lua requires a non-nil redis client")
		}

		return NewRedisLua(pool, rdb), nil
	default:
		return nil, fmt.Errorf("stock: unknown adapter %q", name)
	}
}
