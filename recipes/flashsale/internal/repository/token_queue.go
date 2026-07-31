package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
)

// Kafka wiring for the async order writer.
const (
	ordersTopic    = "flashsale.orders"
	ordersGroup    = "flashsale-order-writer"
	batchMaxSize   = 500                   // rows per INSERT
	batchMaxWait   = 50 * time.Millisecond // flush even when the batch is short
	pollMaxRecords = 2000
	// produceTimeout caps how long a checkout waits for Kafka to accept the order
	// before it gives up and tells the caller the sale is unavailable.
	produceTimeout = 5 * time.Second
)

// orderMsg is the queued order. JSON rather than a binary codec because a reader
// should be able to `kafka-console-consumer` the topic and see what is going on.
type orderMsg struct {
	ProductID      int64  `json:"product_id"`
	Qty            int    `json:"qty"`
	IdempotencyKey string `json:"idempotency_key"`
}

// TokenQueueAdapter grants stock from an in-process quota, then records the order
// asynchronously through Kafka. This is the shape large sales actually run, and it
// is the fastest and least strict adapter in the recipe.
//
// It is fast because the sell/reject decision touches no shared row, no lock
// outside this process and no network: rejections cost almost nothing, which is
// what matters when most buyers lose. Postgres sees a smoothed stream of batched
// inserts instead of a spike of individual transactions.
//
// It is least strict for three reasons, all of which the numbers should show:
//
//   - Quota fragments across replicas. Each process is seeded a slice of the
//     stock, so a replica can sell out while another still holds units, and the
//     sale UNDERSELLS. Real deployments add refill or rebalancing; this one does
//     not, so the stranding is measurable.
//   - The order is only eventually in Postgres. HTTP 200 means durably queued in
//     Kafka, not committed, so a reader has to wait for consumer lag to reach
//     zero before counting orders.
//   - Redelivery is possible, which is why every message carries an idempotency
//     key and the consumer inserts with ON CONFLICT DO NOTHING.
//
// A system willing to return before the Kafka ack is faster still, at the cost of
// answering 200 for an order that exists nowhere durable. This adapter waits.
type TokenQueueAdapter struct {
	pool     *pgxpool.Pool
	producer *kgo.Client
	log      *slog.Logger

	mu    sync.Mutex
	quota map[int64]int // this replica's remaining share of the stock
}

// NewTokenQueue constructs the quota-plus-queue adapter, starting the consumer
// that drains Kafka into Postgres. The consumer runs until ctx is canceled.
func NewTokenQueue(
	ctx context.Context,
	pool *pgxpool.Pool,
	brokers []string,
	log *slog.Logger,
) (*TokenQueueAdapter, error) {
	producer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		// acks=1: the leader has the record before Decrement returns. Waiting for
		// all replicas would be meaningless on a single-broker dev stack.
		kgo.RequiredAcks(kgo.LeaderAck()),
		kgo.DisableIdempotentWrite(),
		kgo.ProducerLinger(5*time.Millisecond),
		// Bound the wait. Without these the client retries a record forever, so a
		// broker that is down turns every checkout into a hung request rather than
		// a failed one, and the caller never learns anything.
		kgo.RecordDeliveryTimeout(produceTimeout),
		kgo.RecordRetries(3),
	)
	if err != nil {
		return nil, fmt.Errorf("token_queue: producer: %w", err)
	}

	a := &TokenQueueAdapter{
		pool:     pool,
		producer: producer,
		log:      log,
		quota:    make(map[int64]int),
	}

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(ordersGroup),
		kgo.ConsumeTopics(ordersTopic),
		kgo.DisableAutoCommit(),
	)
	if err != nil {
		producer.Close()

		return nil, fmt.Errorf("token_queue: consumer: %w", err)
	}

	go a.consume(ctx, consumer)

	return a, nil
}

// Kind returns the repository kind used for metrics labels.
func (a *TokenQueueAdapter) Kind() Kind { return KindTokenQueue }

// grant takes qty from this replica's quota, reporting what is left. This is the
// entire hot path of the sell decision: no network, no shared row.
func (a *TokenQueueAdapter) grant(productID int64, qty int) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	current, ok := a.quota[productID]
	if !ok {
		return 0, domain.ErrProductNotFound
	}

	if current < qty {
		return 0, domain.ErrOutOfStock
	}

	a.quota[productID] = current - qty

	return current - qty, nil
}

// refund puts a grant back after a failure to queue the order.
func (a *TokenQueueAdapter) refund(productID int64, qty int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.quota[productID] += qty
}

// Decrement grants from the local quota, then queues the order for the async
// writer. Returning without an order ID is deliberate: no row exists yet.
func (a *TokenQueueAdapter) Decrement(
	ctx context.Context,
	productID int64,
	qty int,
) (Result, error) {
	remaining, err := a.grant(productID, qty)
	if err != nil {
		return Result{}, err
	}

	payload, err := json.Marshal(orderMsg{
		ProductID:      productID,
		Qty:            qty,
		IdempotencyKey: uuid.NewString(),
	})
	if err != nil {
		a.refund(productID, qty)

		return Result{}, fmt.Errorf("token_queue: marshal: %w", err)
	}

	// Keyed by product so one product's orders land in one partition and stay in
	// order. Synchronous because a 200 should mean the order is durably queued.
	rec := &kgo.Record{
		Topic: ordersTopic,
		Key:   []byte(strconv.FormatInt(productID, 10)),
		Value: payload,
	}

	if err := a.producer.ProduceSync(ctx, rec).FirstErr(); err != nil {
		// The grant happened but the order did not queue, so give the unit back.
		// Crash between those two lines and it leaks, exactly like redis_atomic.
		a.refund(productID, qty)

		// The buyer's request was fine; our broker is not. Wrapping ErrUnavailable
		// is what turns this into a 503 rather than blaming the caller with a 4xx.
		return Result{}, fmt.Errorf("token_queue: produce: %w: %w", domain.ErrUnavailable, err)
	}

	// OrderID 0: the row does not exist yet, and inventing a number here would
	// imply a durability this adapter has not achieved.
	return Result{OrderID: 0, StockRemaining: remaining}, nil
}

// consume drains the topic into Postgres in batches until ctx is canceled.
//
// Each poll carries its own batchMaxWait deadline. That deadline is what makes
// the size-or-age flush below reachable: PollRecords blocks until records arrive,
// so a poll with no deadline would park forever holding a part-full batch, and
// the last few orders of a sale would never reach Postgres.
func (a *TokenQueueAdapter) consume(ctx context.Context, client *kgo.Client) {
	defer client.Close()

	pending := make([]orderMsg, 0, batchMaxSize)
	lastFlush := time.Now()

	flush := func() {
		if len(pending) == 0 {
			return
		}

		if err := a.insertBatch(ctx, pending); err != nil {
			// Not committing means Kafka redelivers, which is what the
			// idempotency key is for. Dropping the batch silently would be the bug.
			a.log.ErrorContext(ctx, "token_queue: batch insert failed",
				slog.Int("rows", len(pending)), slog.String("err", err.Error()))

			return
		}

		if err := client.CommitUncommittedOffsets(ctx); err != nil {
			a.log.ErrorContext(ctx, "token_queue: commit offsets failed",
				slog.String("err", err.Error()))

			return
		}

		pending = pending[:0]
		lastFlush = time.Now()
	}

	for ctx.Err() == nil {
		pollCtx, cancel := context.WithTimeout(ctx, batchMaxWait)
		fetches := client.PollRecords(pollCtx, pollMaxRecords)

		cancel()

		if fetches.IsClientClosed() {
			return
		}

		fetches.EachError(func(t string, p int32, err error) {
			// An expired poll deadline is the normal idle path, not a failure.
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				return
			}

			a.log.ErrorContext(ctx, "token_queue: fetch error",
				slog.String("topic", t), slog.Int("partition", int(p)),
				slog.String("err", err.Error()))
		})

		fetches.EachRecord(func(rec *kgo.Record) {
			var msg orderMsg
			if err := json.Unmarshal(rec.Value, &msg); err != nil {
				// A poison message must not stall its partition forever.
				a.log.ErrorContext(ctx, "token_queue: bad message, dropping",
					slog.String("err", err.Error()))

				return
			}

			pending = append(pending, msg)
		})

		if len(pending) >= batchMaxSize || time.Since(lastFlush) >= batchMaxWait {
			flush()
		}
	}
}

// insertBatch writes one multi-row INSERT, skipping keys already present so a
// redelivered message cannot double-book a unit.
func (a *TokenQueueAdapter) insertBatch(ctx context.Context, msgs []orderMsg) error {
	productIDs := make([]int64, len(msgs))
	quantities := make([]int32, len(msgs))
	keys := make([]string, len(msgs))

	for i, m := range msgs {
		productIDs[i] = m.ProductID
		quantities[i] = int32(m.Qty) //nolint:gosec // qty is a small positive request field
		keys[i] = m.IdempotencyKey
	}

	_, err := a.pool.Exec(ctx, `
		INSERT INTO flashsale.orders (product_id, qty, idempotency_key)
		SELECT * FROM unnest($1::bigint[], $2::int[], $3::uuid[])
		ON CONFLICT (idempotency_key) DO NOTHING
	`, productIDs, quantities, keys)
	if err != nil {
		return fmt.Errorf("token_queue: insert batch: %w", err)
	}

	return nil
}

// Seed upserts the product row, clears prior orders, and sets this replica's
// quota. Callers running several replicas seed each one with its own share.
func (a *TokenQueueAdapter) Seed(
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
		return fmt.Errorf("token_queue: seed product: %w", err)
	}

	if _, err := a.pool.Exec(ctx,
		`DELETE FROM flashsale.orders WHERE product_id = $1`, productID,
	); err != nil {
		return fmt.Errorf("token_queue: clear orders: %w", err)
	}

	a.mu.Lock()
	a.quota[productID] = stock
	a.mu.Unlock()

	return nil
}

// Stock returns this replica's remaining quota, which is not the sale's remaining
// stock when more than one replica is running.
func (a *TokenQueueAdapter) Stock(_ context.Context, productID int64) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	v, ok := a.quota[productID]
	if !ok {
		return 0, domain.ErrProductNotFound
	}

	return v, nil
}
