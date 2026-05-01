// Command user-service runs the universal user-service used by recipes
// across the cookbook. It serves a small in-memory user catalog:
//
//	GET /users        → list of users
//	GET /users/:id    → one user, 404 if not found
//	GET /healthz      → liveness
//	GET /metrics      → Prometheus
//
// Env vars:
//
//	SERVICE_PORT      listen port (default 8080)
//	LOG_LEVEL         debug|info|warn|error (default info)
//	OTLP_ENDPOINT     OTel collector OTLP/HTTP host:port (default localhost:4318)
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
	"github.com/raphaeldiscky/distributed-cookbook/pkg/telemetry"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/config"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/domain"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/handler"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/metrics"
	"github.com/raphaeldiscky/distributed-cookbook/services/user-service/internal/routes"
)

const serviceName = "user-service"

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fatal:", err)

		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()
	log := logger.New(cfg.Shared.LogLevel)
	log.Info("starting "+serviceName, slog.Int("port", cfg.Port))

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

	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := metrics.New(reg)

	usersHandler := handler.NewUsers(domain.SeedUsers(), m)

	e := httpserver.New(serviceName, reg, log)
	routes.Register(e, usersHandler)

	return httpserver.Run(ctx, e, fmt.Sprintf(":%d", cfg.Port), log)
}
