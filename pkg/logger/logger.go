// Package logger builds a structured slog.Logger with trace-id enrichment.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// New returns a JSON slog.Logger at the given level (debug/info/warn/error).
// Logs are written to stderr so stdout stays clean for tooling.
func New(level string) *slog.Logger {
	return NewWithWriter(os.Stderr, level)
}

// NewWithWriter is like New but lets callers substitute the writer (useful in tests).
func NewWithWriter(w io.Writer, level string) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: parseLevel(level),
	})

	return slog.New(&traceHandler{Handler: h})
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// traceHandler injects trace_id and span_id from the OTel span in the context.
// This lets Grafana correlate logs to traces via the derivedFields regex.
type traceHandler struct {
	slog.Handler
}

// Handle injects trace_id and span_id from the OTel span in ctx, then delegates.
// slog.Handler's interface mandates passing slog.Record by value.
//
//nolint:gocritic // slog.Handler interface requires Record to be passed by value
func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}
