package telemetry

import "context"

// Shutdown flushes spans and stops the tracer provider.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if t.tp == nil {
		return nil
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, t.cfg.ShutdownWait)
	defer cancel()

	return t.tp.Shutdown(shutdownCtx)
}
