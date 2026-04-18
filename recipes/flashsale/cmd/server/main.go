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

	"github.com/raphaeldiscky/distributed-cookbook/pkg/httpserver"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/logger"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/pgconn"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/redisconn"
	"github.com/raphaeldiscky/distributed-cookbook/pkg/telemetry"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/config"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/handler"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/metrics"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/stock"
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
		slog.String("adapter", string(cfg.Adapter)),
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

	defer func() { rdb.Close() }() //nolint:errcheck,gosec // process-exit cleanup; error cannot be acted on

	dec, err := stock.New(cfg.Adapter, pool, rdb)
	if err != nil {
		return fmt.Errorf("stock adapter: %w", err)
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := metrics.New(reg)

	e := httpserver.New(serviceName, reg)
	handler.NewCheckout(dec, m, log).Register(e)

	return httpserver.Run(ctx, e, fmt.Sprintf(":%d", cfg.Port), log)
}
