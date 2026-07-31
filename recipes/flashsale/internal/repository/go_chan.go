package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// chanOp is what the owner goroutine should do with a request. Routing seeds and
// reads through the same channel as claims is what keeps the stock map free of
// locks: one goroutine performs every access, whatever the caller wanted.
type chanOp int

const (
	opClaim chanOp = iota // decrement if enough stock
	opSet                 // set stock outright (seeding)
	opRead                // report stock, change nothing
)

// claimReq is one request to the owner goroutine.
type claimReq struct {
	op        chanOp
	productID int64
	qty       int
	reply     chan claimResp // buffered by the caller, so the owner never blocks
}

// claimResp is the owner's verdict plus the stock left after it.
type claimResp struct {
	remaining int
	err       error
}

// GoChanAdapter keeps stock in a map owned by exactly one goroutine. Every
// checkout sends a request down a channel and waits for that goroutine's answer,
// so no two claims are ever evaluated concurrently.
//
// This is what Redis does, moved in-process: correctness by single ownership
// rather than by locking or by a clever statement. It takes the storage round
// trip out of the decision entirely, which is why it belongs in the comparison,
// and it isolates how much of every other adapter's latency is coordination
// rather than storage.
//
// Two things make it a teaching contrast rather than something to ship. The
// state is not durable, so a restart forgets what it sold. And it is correct
// only inside one process: run two replicas and each sells the full stock, which
// is precisely the fragmentation problem quota-based designs have to solve.
type GoChanAdapter struct {
	pool  *pgxpool.Pool
	reqs  chan claimReq
	stock map[int64]int // owned by run(), never touched from outside it
}

// NewGoChan constructs the single-owner adapter and starts its owner goroutine,
// which lives until ctx is canceled.
func NewGoChan(ctx context.Context, pool *pgxpool.Pool) *GoChanAdapter {
	a := &GoChanAdapter{
		pool:  pool,
		reqs:  make(chan claimReq, 1024),
		stock: make(map[int64]int),
	}

	go a.run(ctx)

	return a
}

// Kind returns the repository kind used for metrics labels.
func (a *GoChanAdapter) Kind() Kind { return KindGoChan }

// run is the single owner of a.stock. Being the only reader and writer is what
// lets the map go without a mutex and stops the check-then-decrement below from
// interleaving with anyone else's.
func (a *GoChanAdapter) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-a.reqs:
			req.reply <- a.apply(req)
		}
	}
}

// apply handles one request. Only run() may call it.
func (a *GoChanAdapter) apply(req claimReq) claimResp {
	if req.op == opSet {
		a.stock[req.productID] = req.qty

		return claimResp{remaining: req.qty}
	}

	current, ok := a.stock[req.productID]
	if !ok {
		return claimResp{err: domain.ErrProductNotFound}
	}

	if req.op == opRead {
		return claimResp{remaining: current}
	}

	if current < req.qty {
		return claimResp{err: domain.ErrOutOfStock}
	}

	a.stock[req.productID] = current - req.qty

	return claimResp{remaining: current - req.qty}
}

// ask sends one request to the owner goroutine and waits for its answer.
func (a *GoChanAdapter) ask(ctx context.Context, req claimReq) (claimResp, error) {
	req.reply = make(chan claimResp, 1)

	select {
	case a.reqs <- req:
	case <-ctx.Done():
		return claimResp{}, fmt.Errorf("go_chan: enqueue: %w", ctx.Err())
	}

	select {
	case resp := <-req.reply:
		return resp, nil
	case <-ctx.Done():
		return claimResp{}, fmt.Errorf("go_chan: await: %w", ctx.Err())
	}
}

// Decrement asks the owner goroutine for a claim, then records the order.
func (a *GoChanAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	resp, err := a.ask(ctx, claimReq{op: opClaim, productID: productID, qty: qty})
	if err != nil {
		return Result{}, err
	}

	if resp.err != nil {
		return Result{}, resp.err
	}

	// The claim is already committed in memory, so this insert carries the same
	// dual-write gap as redis_lua, with an in-process counter instead of Redis.
	var orderID int64

	err = a.pool.QueryRow(ctx,
		`INSERT INTO flashsale.orders (product_id, qty) VALUES ($1, $2) RETURNING id`,
		productID, qty,
	).Scan(&orderID)
	if err != nil {
		return Result{}, fmt.Errorf("go_chan: insert order: %w", err)
	}

	return Result{OrderID: orderID, StockRemaining: resp.remaining}, nil
}

// Seed upserts the Postgres product row so GET /stock and the oversell tracker
// have something to read, clears prior orders, then hands the new stock to the
// owner goroutine.
func (a *GoChanAdapter) Seed(
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
		return fmt.Errorf("go_chan: seed product: %w", err)
	}

	if _, err := a.pool.Exec(ctx,
		`DELETE FROM flashsale.orders WHERE product_id = $1`, productID,
	); err != nil {
		return fmt.Errorf("go_chan: clear orders: %w", err)
	}

	if _, err := a.ask(ctx, claimReq{op: opSet, productID: productID, qty: stock}); err != nil {
		return err
	}

	return nil
}

// Stock returns the owner goroutine's current count.
func (a *GoChanAdapter) Stock(ctx context.Context, productID int64) (int, error) {
	resp, err := a.ask(ctx, claimReq{op: opRead, productID: productID})
	if err != nil {
		return 0, err
	}

	return resp.remaining, resp.err
}
