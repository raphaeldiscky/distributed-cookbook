// Package service holds the flashsale recipe's application services —
// thin orchestration layers between the HTTP handlers and the repositories.
//
// This is where logic that isn't transport (handler's job) and isn't
// persistence (repository's job) lives. For the flashsale recipe that
// means: routing per-request to the right Inventory implementation,
// timing metrics, and the in-memory oversell-detection invariant
// (comparing orderCount against initialStock per (kind, product)).
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/metrics"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/repository"
)

// Checkout is the application service that handlers talk to.
// It owns the per-(kind, product) invariants and metric bookkeeping that
// don't belong in the HTTP layer or the repository layer.
type Checkout struct {
	inventories map[repository.Kind]repository.Inventory
	defaultKind repository.Kind
	metrics     *metrics.Metrics
	tracer      trace.Tracer
	log         *slog.Logger

	// Oversell detection: /seed records initial stock per (kind, product).
	// Every successful /checkout increments orderCount for the same key.
	// If orderCount > initialStock, that checkout is an oversell.
	mu           sync.Mutex
	initialStock map[trackKey]int
	orderCount   map[trackKey]int
}

// trackKey is the composite lookup key for the oversell tracker.
type trackKey struct {
	kind repository.Kind
	pid  int64
}

// Result is the post-checkout summary returned to the handler.
type Result struct {
	OrderID        int64
	StockRemaining int
	Kind           repository.Kind
}

// NewCheckout constructs a Checkout service wired to every Inventory
// implementation, the metrics registry, and an OTel tracer. defaultKind
// is used when a caller omits the kind (e.g. POST /checkout without
// :adapter). The tracer is typically `tel.Tracer()` from pkg/telemetry —
// it creates a `flashsale.checkout` span per call that pairs nicely with
// the outer `otelecho` HTTP span and the inner `otelpgx`/`otelredis`
// spans in Grafana Tempo.
func NewCheckout(
	inventories map[repository.Kind]repository.Inventory,
	defaultKind repository.Kind,
	m *metrics.Metrics,
	tracer trace.Tracer,
	log *slog.Logger,
) *Checkout {
	return &Checkout{
		inventories:  inventories,
		defaultKind:  defaultKind,
		metrics:      m,
		tracer:       tracer,
		log:          log,
		initialStock: make(map[trackKey]int),
		orderCount:   make(map[trackKey]int),
	}
}

// resolveKind picks an Inventory impl by kind; empty kind falls back to default.
func (s *Checkout) resolveKind(
	kind repository.Kind,
) (repository.Kind, repository.Inventory, error) {
	if kind == "" {
		kind = s.defaultKind
	}

	inv, ok := s.inventories[kind]
	if !ok {
		return "", nil, fmt.Errorf("unknown kind %q", kind)
	}

	return kind, inv, nil
}

// Checkout performs the atomic stock decrement for the given kind. Metrics
// and oversell tracking happen here. Callers should treat the returned
// error as HTTP-mappable via errors.Is on domain.ErrOutOfStock /
// domain.ErrProductNotFound.
func (s *Checkout) Checkout(
	ctx context.Context,
	kind repository.Kind,
	productID int64,
	qty int,
) (Result, error) {
	resolvedKind, inv, err := s.resolveKind(kind)
	if err != nil {
		return Result{}, err
	}

	kindStr := string(resolvedKind)

	// Manual span: between the outer otelecho HTTP span and the inner
	// otelpgx / otelredis spans. Attributes make a checkout's adapter,
	// product, and qty visible at a glance in Tempo.
	ctx, span := s.tracer.Start(ctx, "flashsale.checkout",
		trace.WithAttributes(
			attribute.String("adapter", kindStr),
			attribute.Int64("product_id", productID),
			attribute.Int("qty", qty),
		),
	)
	defer span.End()

	start := time.Now()

	res, decErr := inv.Decrement(ctx, productID, qty)

	s.metrics.CheckoutLatency.WithLabelValues(kindStr).Observe(time.Since(start).Seconds())

	switch {
	case decErr == nil:
		s.metrics.CheckoutAttempts.WithLabelValues(kindStr, string(metrics.OutcomeOK)).Inc()
		s.metrics.StockRemaining.
			WithLabelValues(kindStr, strconv.FormatInt(productID, 10)).
			Set(float64(res.StockRemaining))

		s.recordOversell(resolvedKind, productID, qty)

		span.SetAttributes(
			attribute.String("outcome", string(metrics.OutcomeOK)),
			attribute.Int64("order_id", res.OrderID),
			attribute.Int("stock_remaining", res.StockRemaining),
		)

		return Result{
			OrderID:        res.OrderID,
			StockRemaining: res.StockRemaining,
			Kind:           resolvedKind,
		}, nil

	case errors.Is(decErr, domain.ErrOutOfStock):
		s.metrics.CheckoutAttempts.WithLabelValues(kindStr, string(metrics.OutcomeOutOfStock)).Inc()
		span.SetAttributes(attribute.String("outcome", string(metrics.OutcomeOutOfStock)))

		return Result{}, decErr

	case errors.Is(decErr, domain.ErrProductNotFound):
		s.metrics.CheckoutAttempts.WithLabelValues(kindStr, string(metrics.OutcomeNotFound)).Inc()
		span.SetAttributes(attribute.String("outcome", string(metrics.OutcomeNotFound)))

		return Result{}, decErr

	default:
		s.metrics.CheckoutAttempts.WithLabelValues(kindStr, string(metrics.OutcomeError)).Inc()
		s.log.ErrorContext(ctx, "checkout failed",
			slog.String("kind", kindStr),
			slog.String("err", decErr.Error()),
		)
		// Mark the span as a real failure (not a business-logic 409/404).
		span.SetStatus(codes.Error, decErr.Error())
		span.SetAttributes(attribute.String("outcome", string(metrics.OutcomeError)))

		return Result{}, decErr
	}
}

// recordOversell increments the per-kind orderCount and bumps the
// oversell counter when it exceeds the seeded initial stock.
func (s *Checkout) recordOversell(kind repository.Kind, productID int64, qty int) {
	key := trackKey{kind: kind, pid: productID}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.orderCount[key] += qty
	if init, ok := s.initialStock[key]; ok && s.orderCount[key] > init {
		s.metrics.OversellTotal.WithLabelValues(string(kind)).Inc()
	}
}

// Seed primes every Inventory implementation's storage with the given
// product and stock, and resets oversell tracking for each (kind,
// product) pair.
func (s *Checkout) Seed(ctx context.Context, productID int64, name string, stock int) (int, error) {
	for k, inv := range s.inventories {
		if err := inv.Seed(ctx, productID, name, stock); err != nil {
			s.log.ErrorContext(ctx, "seed failed",
				slog.String("kind", string(k)),
				slog.String("err", err.Error()),
			)

			return 0, fmt.Errorf("seed %s: %w", k, err)
		}
	}

	s.mu.Lock()

	for k := range s.inventories {
		key := trackKey{kind: k, pid: productID}
		s.initialStock[key] = stock
		s.orderCount[key] = 0
		s.metrics.StockRemaining.
			WithLabelValues(string(k), strconv.FormatInt(productID, 10)).
			Set(float64(stock))
	}

	s.mu.Unlock()

	return len(s.inventories), nil
}

// Stock reads the current stock from the given Inventory's source of
// truth. The returned kind tells callers which one was consulted
// (differs from the input when kind was empty).
func (s *Checkout) Stock(
	ctx context.Context,
	kind repository.Kind,
	productID int64,
) (int, repository.Kind, error) {
	resolvedKind, inv, err := s.resolveKind(kind)
	if err != nil {
		return 0, "", err
	}

	v, err := inv.Stock(ctx, productID)
	if err != nil {
		return 0, resolvedKind, err
	}

	return v, resolvedKind, nil
}
