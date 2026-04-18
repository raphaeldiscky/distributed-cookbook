package stock

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// PgCondAdapter decrements stock via a single conditional UPDATE.
// Correctness comes from Postgres row-level locking during UPDATE — the
// check (stock >= qty) and the decrement happen atomically in one statement.
type PgCondAdapter struct {
	pool *pgxpool.Pool
}

// NewPgCond constructs the Postgres conditional-UPDATE adapter.
func NewPgCond(pool *pgxpool.Pool) *PgCondAdapter {
	return &PgCondAdapter{pool: pool}
}

// Name returns the adapter label used for metrics.
func (a *PgCondAdapter) Name() Adapter { return AdapterPgCond }

// Decrement runs the atomic check-and-decrement via a single conditional UPDATE.
func (a *PgCondAdapter) Decrement(ctx context.Context, productID int64, qty int) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("pg_cond: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	// The atomic check-and-decrement. If WHERE clause fails, no row is updated.
	var newStock int

	err = tx.QueryRow(ctx, `
		UPDATE flashsale.products
		   SET stock = stock - $1
		 WHERE id = $2 AND stock >= $1
		RETURNING stock
	`, qty, productID).Scan(&newStock)
	if err != nil {
		return Result{}, classifyUpdateErr(ctx, tx, productID, err)
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_cond: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_cond: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row and truncates prior orders for this product.
func (a *PgCondAdapter) Seed(ctx context.Context, productID int64, name string, stock int) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, stock = EXCLUDED.stock
	`, productID, name, stock)
	if err != nil {
		return fmt.Errorf("pg_cond: seed: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// classifyUpdateErr decides whether an UPDATE...WHERE returning no rows was
// caused by a missing product or by insufficient stock, and wraps real errors.
func classifyUpdateErr(ctx context.Context, tx pgx.Tx, productID int64, err error) error {
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("pg_cond: update: %w", err)
	}

	var exists bool
	if e := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM flashsale.products WHERE id = $1)`,
		productID,
	).Scan(&exists); e != nil {
		return fmt.Errorf("pg_cond: exists: %w", e)
	}

	if !exists {
		return domain.ErrProductNotFound
	}

	return domain.ErrOutOfStock
}

// Stock returns the current stock value from Postgres.
func (a *PgCondAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
