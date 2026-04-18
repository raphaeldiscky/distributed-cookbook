// Package telemetry sets up OTel tracing and provides a typed handle for shutdown.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Telemetry is a handle returned by New. Keep it on the app struct so Shutdown
// can run at process exit.
type Telemetry struct {
	cfg Config
	tp  *sdktrace.TracerProvider
}

// New initializes the OTel tracer provider and registers it globally.
// If cfg.TracingEnabled is false, a no-op tracer is installed.
func New(ctx context.Context, cfg Config) (*Telemetry, error) {
	t := &Telemetry{cfg: cfg}
	if !cfg.TracingEnabled {
		return t, nil
	}

	tp, err := newTracerProvider(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("telemetry: init tracer: %w", err)
	}

	t.tp = tp

	return t, nil
}

// Tracer returns a tracer scoped to the service name.
func (t *Telemetry) Tracer() trace.Tracer {
	return otelTracer(t.cfg.ServiceName)
}
