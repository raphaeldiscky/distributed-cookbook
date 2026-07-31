package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// optimisticMaxAttempts bounds the retry loop. Unbounded retry under heavy
// contention is a livelock: a slow client can lose every race forever while the
// server burns round trips on its behalf. Giving up and surfacing
// ErrRetryExhausted turns that into a number on the dashboard.
const optimisticMaxAttempts = 5

// PgOptimisticAdapter reads (stock, version), then writes conditional on the
// version being unchanged. A losing writer updates zero rows and retries.
//
// This is what ORMs mean by optimistic locking, and it is correct. Its cost is
// shaped opposite to PgForUpdateAdapter's: nobody blocks, but under contention
// most attempts lose and re-read, so the wasted work grows with the number of
// concurrent buyers rather than with the amount of stock.
type PgOptimisticAdapter struct {
	pool *pgxpool.Pool
}

// NewPgOptimistic constructs the version-column optimistic-locking adapter.
func NewPgOptimistic(pool *pgxpool.Pool) *PgOptimisticAdapter {
	return &PgOptimisticAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *PgOptimisticAdapter) Kind() Kind { return KindPgOptimistic }

// Decrement retries a compare-and-set on (stock, version) until it wins, the
// product runs out, or it exhausts optimisticMaxAttempts.
func (a *PgOptimisticAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	for range optimisticMaxAttempts {
		res, err := a.attempt(ctx, productID, qty)
		if err == nil {
			return res, nil
		}

		// errVersionConflict means another buyer committed between our read and
		// our write. Nothing is wrong, we just lost, so read again and retry.
		if !errors.Is(err, errVersionConflict) {
			return Result{}, err
		}
	}

	return Result{}, domain.ErrRetryExhausted
}

// errVersionConflict signals a lost compare-and-set, which is retriable. It
// never escapes Decrement.
var errVersionConflict = errors.New("version conflict")

// attempt runs one read-then-conditional-write cycle.
func (a *PgOptimisticAdapter) attempt(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("pg_optimistic: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	var (
		current int
		version int
	)

	err = tx.QueryRow(ctx,
		`SELECT stock, version FROM flashsale.products WHERE id = $1`,
		productID,
	).Scan(&current, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, domain.ErrProductNotFound
		}

		return Result{}, fmt.Errorf("pg_optimistic: select: %w", err)
	}

	if current < qty {
		return Result{}, domain.ErrOutOfStock
	}

	// The compare-and-set. Matching on version is what makes the read above
	// safe to have taken without a lock: if anyone else committed in between,
	// version moved and this updates nothing.
	newStock := current - qty

	tag, err := tx.Exec(ctx, `
		UPDATE flashsale.products
		   SET stock = $1, version = version + 1
		 WHERE id = $2 AND version = $3
	`, newStock, productID, version)
	if err != nil {
		return Result{}, fmt.Errorf("pg_optimistic: update: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return Result{}, errVersionConflict
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_optimistic: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_optimistic: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row, resets its version, and truncates prior orders.
func (a *PgOptimisticAdapter) Seed(
	ctx context.Context,
	productID int64,
	name string,
	stock int,
) error {
	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.products (id, name, stock, version) VALUES ($1, $2, $3, 0)
		ON CONFLICT (id) DO UPDATE
		   SET name = EXCLUDED.name, stock = EXCLUDED.stock, version = 0
	`, productID, name, stock)
	if err != nil {
		return fmt.Errorf("pg_optimistic: seed: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the current stock value from Postgres.
func (a *PgOptimisticAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
