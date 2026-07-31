// This file is naive.go plus FOR UPDATE on the SELECT, and that single clause is
// the whole lesson. Sharing a body between the two would hide the one difference
// a reader opened both files to compare, so the dupl check is waived here.
//
//nolint:dupl // near-identical to naive.go on purpose, see above
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// PgForUpdateAdapter takes a pessimistic row lock, checks stock in Go, then
// writes. This is the same read-check-write shape as NaiveAdapter, made correct
// by holding the row lock across all three steps.
//
// It is the fix most codebases actually contain, and its cost is the reason
// PgCondAdapter exists: the lock is held for a full application round trip, so
// every other buyer of that product queues behind it. Contention shows up here
// as latency rather than as wrong answers.
type PgForUpdateAdapter struct {
	pool *pgxpool.Pool
}

// NewPgForUpdate constructs the pessimistic row-lock adapter.
func NewPgForUpdate(pool *pgxpool.Pool) *PgForUpdateAdapter {
	return &PgForUpdateAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *PgForUpdateAdapter) Kind() Kind { return KindPgForUpdate }

// Decrement locks the product row, checks stock, then writes the new value.
func (a *PgForUpdateAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("pg_for_update: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	// FOR UPDATE blocks any other transaction wanting this row until we commit.
	// That is what makes the check-then-write below safe, and also what
	// serializes every concurrent buyer of this product.
	var current int

	err = tx.QueryRow(ctx,
		`SELECT stock FROM flashsale.products WHERE id = $1 FOR UPDATE`,
		productID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, domain.ErrProductNotFound
		}

		return Result{}, fmt.Errorf("pg_for_update: select: %w", err)
	}

	if current < qty {
		return Result{}, domain.ErrOutOfStock
	}

	newStock := current - qty
	if _, err := tx.Exec(ctx,
		`UPDATE flashsale.products SET stock = $1 WHERE id = $2`,
		newStock, productID,
	); err != nil {
		return Result{}, fmt.Errorf("pg_for_update: update: %w", err)
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_for_update: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_for_update: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row and truncates prior orders for this product.
func (a *PgForUpdateAdapter) Seed(
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
		return fmt.Errorf("pg_for_update: seed: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the current stock value from Postgres.
func (a *PgForUpdateAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
