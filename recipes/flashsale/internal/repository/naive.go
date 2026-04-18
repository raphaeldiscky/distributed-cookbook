package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// NaiveAdapter reads stock, checks it in Go, then writes the computed new value.
// This is intentionally racy: two goroutines can both read stock=1, both compute
// new=0, both write — two orders created when only one should be. This is the
// "before" picture that the other adapters fix.
type NaiveAdapter struct {
	pool *pgxpool.Pool
}

// NewNaive constructs the intentionally-racy adapter used for the "before" demo.
func NewNaive(pool *pgxpool.Pool) *NaiveAdapter {
	return &NaiveAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *NaiveAdapter) Kind() Kind { return KindNaive }

// Decrement reads stock, checks in Go, then writes the computed new value — racy on purpose.
func (a *NaiveAdapter) Decrement(ctx context.Context, productID int64, qty int) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("naive: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	var current int

	err = tx.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, domain.ErrProductNotFound
		}

		return Result{}, fmt.Errorf("naive: select: %w", err)
	}

	if current < qty {
		return Result{}, domain.ErrOutOfStock
	}

	// THE RACE: newStock is computed from `current`, which another goroutine may
	// also have read. Both write the same value, overwriting each other — lost update.
	newStock := current - qty
	if _, err := tx.Exec(ctx,
		`UPDATE flashsale.products SET stock = $1 WHERE id = $2`,
		newStock, productID,
	); err != nil {
		return Result{}, fmt.Errorf("naive: update: %w", err)
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("naive: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("naive: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row and truncates prior orders for this product.
func (a *NaiveAdapter) Seed(ctx context.Context, productID int64, name string, stock int) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, stock = EXCLUDED.stock
	`, productID, name, stock)
	if err != nil {
		return fmt.Errorf("naive: seed: %w", err)
	}
	// Truncate orders so oversells from a previous run don't leak into this one.
	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the current stock value from Postgres.
func (a *NaiveAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
