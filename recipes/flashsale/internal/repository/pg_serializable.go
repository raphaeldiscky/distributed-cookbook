package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// serializationFailure is the SQLSTATE Postgres raises when it cannot order two
// SERIALIZABLE transactions. It is the retry signal, not a fault.
const serializationFailure = "40001"

// PgSerializableAdapter writes the same naive read-check-write and asks Postgres
// to make it safe by running at SERIALIZABLE isolation, retrying whenever the
// engine says the two transactions could not be ordered.
//
// It reaches the same place as PgOptimisticAdapter from the opposite direction.
// Optimistic locking makes the conflict visible in your own schema through a
// version column, and you decide what a lost race means; here the schema stays
// untouched and the engine detects the conflict for you. Both then pay the same
// bill, which is that under contention most attempts lose and retry.
//
// The detail that catches people: a serialization failure usually surfaces at
// COMMIT rather than at the offending statement, so a retry has to wrap the whole
// transaction. Retrying just the failed statement inside the same transaction
// cannot work, because that transaction is already doomed.
type PgSerializableAdapter struct {
	pool *pgxpool.Pool
}

// NewPgSerializable constructs the SERIALIZABLE-with-retry adapter.
func NewPgSerializable(pool *pgxpool.Pool) *PgSerializableAdapter {
	return &PgSerializableAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *PgSerializableAdapter) Kind() Kind { return KindPgSerializable }

// Decrement retries the whole transaction until it commits, the product runs out,
// or it exhausts optimisticMaxAttempts.
func (a *PgSerializableAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	for range optimisticMaxAttempts {
		res, err := a.attempt(ctx, productID, qty)
		if err == nil {
			return res, nil
		}

		if !isSerializationFailure(err) {
			return Result{}, err
		}
	}

	return Result{}, domain.ErrRetryExhausted
}

// isSerializationFailure reports whether Postgres refused to order this
// transaction against a concurrent one, which is retriable.
func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.SQLState() == serializationFailure
}

// attempt runs one whole SERIALIZABLE transaction, commit included.
//
// The body below is naive.go's read-check-write, on purpose. The only thing making
// it safe is the isolation level passed to BeginTx, and hiding that behind a shared
// helper would hide the entire point of the adapter.
//
//nolint:dupl // identical to naive.go by design, see above
func (a *PgSerializableAdapter) attempt(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, fmt.Errorf("pg_serializable: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	var current int

	err = tx.QueryRow(ctx,
		`SELECT stock FROM flashsale.products WHERE id = $1`, productID,
	).Scan(&current)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Result{}, domain.ErrProductNotFound
		}

		return Result{}, fmt.Errorf("pg_serializable: select: %w", err)
	}

	if current < qty {
		return Result{}, domain.ErrOutOfStock
	}

	// Plain arithmetic on a value read without any lock. At READ COMMITTED this
	// is the naive race; at SERIALIZABLE the engine tracks the read/write
	// dependency and refuses the commit instead of letting the update be lost.
	newStock := current - qty
	if _, err := tx.Exec(ctx,
		`UPDATE flashsale.products SET stock = $1 WHERE id = $2`,
		newStock, productID,
	); err != nil {
		return Result{}, fmt.Errorf("pg_serializable: update: %w", err)
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_serializable: insert order: %w", err)
	}

	// The conflict lands here more often than anywhere above.
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_serializable: commit: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: newStock}, nil
}

// Seed upserts the product row and truncates prior orders for this product.
func (a *PgSerializableAdapter) Seed(
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
		return fmt.Errorf("pg_serializable: seed: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock returns the current stock value from Postgres.
func (a *PgSerializableAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var s int

	err := a.pool.QueryRow(ctx, `SELECT stock FROM flashsale.products WHERE id = $1`, productID).
		Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrProductNotFound
	}

	return s, err
}
