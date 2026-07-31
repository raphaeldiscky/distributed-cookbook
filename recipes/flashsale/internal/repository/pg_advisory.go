// This file is naive.go with one extra statement, an advisory lock, which is what
// makes the identical read-check-write below safe. Both bodies are kept whole so
// that single statement is visible when the two files are read side by side.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// PgAdvisoryAdapter serializes buyers on a Postgres advisory lock rather than on
// the product row, then runs the same read-check-write as NaiveAdapter.
//
// The interesting property is where the contention moves to. PgForUpdateAdapter
// queues buyers on the row's own lock, so the row is both the data and the
// gatekeeper; here the gate is an entry in Postgres' lock manager keyed by an
// arbitrary integer, and the row is touched only once the caller already holds it.
//
// That indirection is also the footgun. The advisory key space is a single global
// int64 shared by the whole database, so an unrelated feature that locks the same
// number will block checkouts, and nothing in the schema records that the number
// is taken.
type PgAdvisoryAdapter struct {
	pool *pgxpool.Pool
}

// NewPgAdvisory constructs the advisory-lock adapter.
func NewPgAdvisory(pool *pgxpool.Pool) *PgAdvisoryAdapter {
	return &PgAdvisoryAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *PgAdvisoryAdapter) Kind() Kind { return KindPgAdvisory }

// Decrement takes a transaction-scoped advisory lock on the product, then reads,
// checks and writes behind it.
func (a *PgAdvisoryAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("pg_advisory: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	// _xact_ scoping is what makes this safe to use here: the lock releases on
	// commit or rollback, so a panic between here and the commit cannot leak it.
	// The session-scoped variant would need an explicit unlock on every path.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, productID); err != nil {
		return Result{}, fmt.Errorf("pg_advisory: lock: %w", err)
	}

	var current int

	err = tx.QueryRow(ctx,
		`SELECT stock FROM flashsale.products WHERE id = $1`, productID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, domain.ErrProductNotFound
		}

		return Result{}, fmt.Errorf("pg_advisory: select: %w", err)
	}

	if current < qty {
		return Result{}, domain.ErrOutOfStock
	}

	newStock := current - qty
	if _, err := tx.Exec(ctx,
		`UPDATE flashsale.products SET stock = $1 WHERE id = $2`,
		newStock, productID,
	); err != nil {
		return Result{}, fmt.Errorf("pg_advisory: update: %w", err)
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_advisory: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_advisory: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row and truncates prior orders for this product.
func (a *PgAdvisoryAdapter) Seed(
	ctx context.Context,
	productID int64,
	name string,
	stock int,
) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, stock = EXCLUDED.stock
	`, productID, name, stock)
	if err != nil {
		return fmt.Errorf("pg_advisory: seed: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the current stock value from Postgres.
func (a *PgAdvisoryAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
