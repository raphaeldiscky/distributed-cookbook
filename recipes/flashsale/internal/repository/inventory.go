// Package repository holds the flashsale recipe's persistence abstractions
// and their concrete implementations.
//
// The `Inventory` interface is DDD-shaped: it's a Repository for an
// aggregate (Product + Order) rather than two separate repositories. The
// coupling is intentional — atomic stock-decrement AND order-insert are
// what this recipe teaches, and splitting them into two repositories
// would destroy that atomicity (letting the application service
// coordinate two writes is the dual-write anti-pattern that the later
// `outbox` recipe exists to fix).
//
// Three implementations ship: Naive (racy, buggy on purpose), PgCond
// (atomic via Postgres conditional UPDATE), RedisLua (atomic via Lua
// EVAL on Redis). Select one by passing its `Kind` to `New`.
package repository

import "context"

// Inventory atomically reduces stock by qty for a product and records an
// order. Returns domain.ErrOutOfStock if qty > current stock and
// domain.ErrProductNotFound when the product is unknown.
type Inventory interface {
	Decrement(ctx context.Context, productID int64, qty int) (Result, error)
	Seed(ctx context.Context, productID int64, name string, stock int) error
	Stock(ctx context.Context, productID int64) (int, error)
	Kind() Kind
}

// Result carries the post-decrement state back to the service.
type Result struct {
	OrderID        int64
	StockRemaining int
}

// Kind identifies which Inventory implementation is active. Values here
// are the recipe's public contract — they appear in URL path segments,
// the `adapter` Prometheus label, and env-var values.
type Kind string

// The three Inventory implementations this recipe ships.
const (
	KindNaive    Kind = "naive"
	KindPgCond   Kind = "pg_cond"
	KindRedisLua Kind = "redis_lua"
)
