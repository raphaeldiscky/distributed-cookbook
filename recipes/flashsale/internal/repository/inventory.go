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

// The Inventory implementations this recipe ships.
//
// Ordered by how much they contend: the first six funnel every buyer onto one
// shared product row, pg_skip_locked turns stock into a row per unit so buyers
// never wait, and the last four keep the authoritative counter outside Postgres.
// Strictness runs roughly the other way, which is the comparison this recipe
// exists to make.
const (
	KindNaive          Kind = "naive"
	KindPgCond         Kind = "pg_cond"
	KindPgForUpdate    Kind = "pg_for_update"
	KindPgAdvisory     Kind = "pg_advisory"
	KindPgOptimistic   Kind = "pg_optimistic"
	KindPgSerializable Kind = "pg_serializable"
	KindPgSkipLocked   Kind = "pg_skip_locked"
	KindRedisLua       Kind = "redis_lua"
	KindRedisAtomic    Kind = "redis_atomic"
	KindGoChan         Kind = "go_chan"
	KindTokenQueue     Kind = "token_queue"
)

// AllKinds is the canonical enumeration, used by the composition root to build
// every adapter and pre-touch its metric series. Adding a Kind above without
// adding it here would leave the adapter unreachable, so keep them together.
//
//nolint:gochecknoglobals // canonical enumeration, read-only
var AllKinds = []Kind{
	KindNaive,
	KindPgCond,
	KindPgForUpdate,
	KindPgAdvisory,
	KindPgOptimistic,
	KindPgSerializable,
	KindPgSkipLocked,
	KindRedisLua,
	KindRedisAtomic,
	KindGoChan,
	KindTokenQueue,
}
