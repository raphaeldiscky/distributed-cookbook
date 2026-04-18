// Command flashsale-server runs the flashsale recipe's HTTP server.
//
// Usage:
//
//	RECIPE_FLASHSALE_ADAPTER=pg_cond go run ./recipes/flashsale/cmd/server
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/raphaeldiscky/distributed-cookbook/pkg/closer"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/httpserver"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/logger"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/pgconn"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/redisconn"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/telemetry"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/config"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/handler"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/metrics"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/repository"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/routes"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/service"
)

const serviceName = "flashsale"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	log := logger.New(cfg.Shared.LogLevel)
	log.Info("starting flashsale",
		slog.String("default_kind", string(cfg.DefaultKind)),
		slog.Int("port", cfg.Port),
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.New(ctx, telemetry.DefaultConfig(serviceName, cfg.Shared.OTLPEndpoint))
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}

	defer func() {
		if err := tel.Shutdown(context.Background()); err != nil {
			log.Error("telemetry shutdown", slog.String("err", err.Error()))
		}
	}()

	pool, err := pgconn.New(ctx, cfg.Shared.PostgresDSN)
	if err != nil {
		return fmt.Errorf("postgres: %w", err)
	}
	defer pool.Close()

	rdb, err := redisconn.New(ctx, cfg.Shared.RedisAddr)
	if err != nil {
		return fmt.Errorf("redis: %w", err)
	}

	defer closer.LogOnError(rdb, log, "redis")

	// Build every Inventory implementation up front. The server holds all
	// three and routes per-request via POST /checkout/:adapter. cfg.DefaultKind
	// is the fallback when the client omits the :adapter path segment.
	inventories := make(map[repository.Kind]repository.Inventory, 3)

	for _, kind := range []repository.Kind{repository.KindNaive, repository.KindPgCond, repository.KindRedisLua} {
		inv, err := repository.New(kind, pool, rdb)
		if err != nil {
			return fmt.Errorf("inventory %q: %w", kind, err)
		}

		inventories[kind] = inv
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := metrics.New(reg)

	// Pre-touch every (kind, outcome) label combination so the series
	// exist at value 0 from the moment Prometheus scrapes, before any
	// traffic. Dashboard panels render all three kinds from second one.
	for kind := range inventories {
		k := string(kind)
		m.OversellTotal.WithLabelValues(k)
		m.CheckoutLatency.WithLabelValues(k)

		for _, outcome := range metrics.AllOutcomes {
			m.CheckoutAttempts.WithLabelValues(k, string(outcome))
		}
	}

	svc := service.NewCheckout(inventories, cfg.DefaultKind, m, log)
	checkoutHandler := handler.NewCheckout(svc, log)

	e := httpserver.New(serviceName, reg, log)
	routes.Register(e, checkoutHandler)

	return httpserver.Run(ctx, e, fmt.Sprintf(":%d", cfg.Port), log)
}
