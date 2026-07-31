package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// stockUnknown is returned as StockRemaining by adapters that cannot answer
// "how many are left" without paying for it. See PgSkipLockedAdapter.
const stockUnknown = -1

// PgSkipLockedAdapter turns stock into one row per unit and claims a row with
// FOR UPDATE SKIP LOCKED. This is the claim pattern Postgres-backed job queues use
// (river, good_job, graphile-worker), pointed at inventory rather than at jobs.
//
// It is the only Postgres adapter here where buyers never wait for each other.
// Every other one funnels them onto a single row, so the second buyer blocks until
// the first commits; SKIP LOCKED tells a blocked reader to move to the next
// unlocked row instead. Contention stops being a queue and becomes a collision
// that is cheap to resolve.
//
// Two costs come with that. Storage and seeding are per unit, so a 50,000-unit
// sale is 50,000 rows to insert before the sale opens. And there is no cheap
// remaining count: "how many are left" is a scan of the unclaimed index rather
// than reading one integer, so Decrement reports stockUnknown rather than paying
// for a COUNT on every checkout.
type PgSkipLockedAdapter struct {
	pool *pgxpool.Pool
}

// NewPgSkipLocked constructs the ticket-claiming adapter.
func NewPgSkipLocked(pool *pgxpool.Pool) *PgSkipLockedAdapter {
	return &PgSkipLockedAdapter{pool: pool}
}

// Kind returns the repository kind used for metrics labels.
func (a *PgSkipLockedAdapter) Kind() Kind { return KindPgSkipLocked }

// Decrement claims qty unclaimed tickets, skipping any another transaction is
// already holding, then records the order.
func (a *PgSkipLockedAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	tx, err := a.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Result{}, fmt.Errorf("pg_skip_locked: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after successful commit is a no-op

	// SKIP LOCKED is the whole adapter. Without it this SELECT would block behind
	// whoever holds the first unclaimed ticket, which is the queue every other
	// Postgres adapter here forms. With it, a contended reader takes the next row.
	rows, err := tx.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM flashsale.stock_tickets
			 WHERE product_id = $1 AND NOT claimed
			 LIMIT $2
			 FOR UPDATE SKIP LOCKED
		)
		UPDATE flashsale.stock_tickets t
		   SET claimed = true
		  FROM claimed c
		 WHERE t.id = c.id
		RETURNING t.id
	`, productID, qty)
	if err != nil {
		return Result{}, fmt.Errorf("pg_skip_locked: claim: %w", err)
	}

	claimed := 0
	for rows.Next() {
		claimed++
	}

	rows.Close()

	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("pg_skip_locked: claim rows: %w", err)
	}

	// Fewer tickets than asked for means the sale is out, or that everything left
	// is locked by someone mid-checkout. Rolling back releases whatever partial
	// claim we did take, so no unit is stranded by a failed multi-unit buy.
	if claimed < qty {
		if claimed == 0 {
			return Result{}, a.classifyEmpty(ctx, tx, productID)
		}

		return Result{}, domain.ErrOutOfStock
	}

	var orderID int64

	err = tx.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("pg_skip_locked: insert order: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("pg_skip_locked: commit: %w", err)
	}

	// Deliberately not counting: see the type comment.
	return Result{OrderID: orderID, StockRemaining: stockUnknown}, nil
}

// classifyEmpty separates a missing product from a drained one.
func (a *PgSkipLockedAdapter) classifyEmpty(
	ctx context.Context,
	tx pgx.Tx,
	productID int64,
) error {
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM flashsale.products WHERE id = $1)`, productID,
	).Scan(&exists); err != nil {
		return fmt.Errorf("pg_skip_locked: exists: %w", err)
	}

	if !exists {
		return domain.ErrProductNotFound
	}

	return domain.ErrOutOfStock
}

// Seed upserts the product, replaces its ticket pile with one row per unit, and
// truncates prior orders. One INSERT builds every ticket, since 50,000 round trips
// would take longer than the sale being measured.
func (a *PgSkipLockedAdapter) Seed(
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
		return fmt.Errorf("pg_skip_locked: seed product: %w", err)
	}

	if _, err := a.pool.Exec(ctx,
		`DELETE FROM flashsale.stock_tickets WHERE product_id = $1`, productID,
	); err != nil {
		return fmt.Errorf("pg_skip_locked: clear tickets: %w", err)
	}

	if _, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.stock_tickets (product_id, claimed)
		SELECT $1, false FROM generate_series(1, $2)
	`, productID, stock); err != nil {
		return fmt.Errorf("pg_skip_locked: seed tickets: %w", err)
	}

	_, err = a.pool.Exec(ctx, `DELETE FROM flashsale.orders WHERE product_id = $1`, productID)

	return err
}

// Stock counts unclaimed tickets. This is the scan Decrement refuses to pay for on
// every checkout, which is fine here because GET /stock is not on the hot path.
func (a *PgSkipLockedAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	var exists bool

	err := a.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM flashsale.products WHERE id = $1)`, productID,
	).Scan(&exists)
	if err != nil {
		return 0, fmt.Errorf("pg_skip_locked: exists: %w", err)
	}

	if !exists {
		return 0, domain.ErrProductNotFound
	}

	var remaining int

	err = a.pool.QueryRow(ctx, `
		SELECT count(*) FROM flashsale.stock_tickets
		 WHERE product_id = $1 AND NOT claimed
	`, productID).Scan(&remaining)

	return remaining, err
}
