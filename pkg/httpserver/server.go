// Package httpserver wires Echo with OTel tracing + Prometheus /metrics + graceful shutdown.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
)

// New returns an Echo instance with tracing, recovery, access logging,
// and /metrics wired. Register your recipe handlers on the returned *echo.Echo.
func New(serviceName string, reg *prometheus.Registry, log *slog.Logger) *echo.Echo {
	e := echo.New()

	e.Use(middleware.Recover())
	e.Use(otelecho.Middleware(serviceName))
	e.Use(accessLog(log))

	// /metrics is handled outside the trace middleware to avoid self-tracing noise,
	// but Echo applies middleware in order; exposing via echo.WrapHandler is fine here.
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(reg, promhttp.HandlerOpts{})))
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	return e
}

// accessLog logs one structured line per request (method, path, status,
// duration, trace_id). Skips /metrics and /healthz to avoid scrape noise.
//
// trace_id comes from the OTel span installed by otelecho, which runs
// before this middleware — handy for correlating logs to traces in Grafana.
func accessLog(log *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if path == "/metrics" || path == "/healthz" {
				return next(c)
			}

			start := time.Now()
			err := next(c)
			dur := time.Since(start)

			req := c.Request()
			status := c.Response().Status
			// The traceHandler in pkg/logger will attach trace_id/span_id from ctx.
			log.LogAttrs(req.Context(), slog.LevelInfo, "http",
				slog.String("method", req.Method),
				slog.String("path", path),
				slog.Int("status", status),
				slog.Duration("duration", dur),
			)

			return err
		}
	}
}

// Run starts srv and blocks until ctx is canceled, then shuts down gracefully.
//
// The bind happens before the "listening" log, so a bind failure (port in
// use) fails fast and the log output doesn't lie about the listener state.
// After bind, we hand the listener to Echo via e.StartServer so Echo's
// startup banner (framework + version) prints on every air restart.
func Run(ctx context.Context, e *echo.Echo, addr string, log *slog.Logger) error {
	srv := &http.Server{
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
	}

	var lc net.ListenConfig

	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	log.Info("http server listening", slog.String("addr", ln.Addr().String()))

	// Handing Echo the already-bound listener lets StartServer skip its own
	// net.Listen call but still run configureServer — which is the code path
	// that prints the Echo banner and framework version.
	e.Listener = ln

	errCh := make(chan error, 1)

	go func() {
		if err := e.StartServer(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}

		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Info("http server shutting down")

		shutdownStart := time.Now()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := srv.Shutdown(shutdownCtx)

		log.Info("http server stopped", slog.Duration("grace", time.Since(shutdownStart)))

		return err
	case err := <-errCh:
		return err
	}
}
