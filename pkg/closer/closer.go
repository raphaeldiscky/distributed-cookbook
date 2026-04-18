// Package closer provides helpers that close resources without losing
// observability when the Close error can't be returned to the caller.
package closer

import (
	"io"
	"log/slog"
)

// LogOnError calls c.Close() and, if it returns an error, logs it at
// warn level tagged with the given resource name. Use this in a `defer`
// when you can't return the Close error to the caller — e.g. process-exit
// cleanup in main:
//
//	defer closer.LogOnError(rdb, log, "redis")
//
// Preferred over `//nolint:errcheck` because it preserves observability:
// a silent Close failure during shutdown is exactly the kind of thing
// you'd want to see in logs when something goes wrong.
func LogOnError(c io.Closer, log *slog.Logger, resource string) {
	if err := c.Close(); err != nil {
		log.Warn("close failed",
			slog.String("resource", resource),
			slog.String("err", err.Error()),
		)
	}
}
