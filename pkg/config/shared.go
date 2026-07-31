// Package config loads shared infrastructure settings from env vars.
// Recipe-specific settings live in each recipe's own config package.
package config

import (
	"os"
	"strings"
)

// Shared carries infra connection settings used by every recipe.
type Shared struct {
	PostgresDSN  string   // POSTGRES_DSN, default "postgres://cookbook:cookbook@localhost:5433/cookbook_db?sslmode=disable"
	RedisAddr    string   // REDIS_ADDR,    default "localhost:6379"
	KafkaBrokers []string // KAFKA_BROKERS, comma-separated, default "localhost:9094" (the broker's host listener)
	OTLPEndpoint string   // OTLP_ENDPOINT, default "localhost:4318" (collector OTLP HTTP)
	LogLevel     string   // LOG_LEVEL,     default "info"
	// TracingEnabled, TRACING_ENABLED, default true. Set false when load testing
	// without the collector running: the OTLP exporter retries every failed batch,
	// and at load-test rates those retries become the thing you are measuring.
	TracingEnabled bool
}

// LoadShared reads env vars with sensible local-dev defaults.
func LoadShared() Shared {
	return Shared{
		PostgresDSN: getenv(
			"POSTGRES_DSN",
			"postgres://cookbook:cookbook@localhost:5433/cookbook_db?sslmode=disable",
		),
		RedisAddr: getenv("REDIS_ADDR", "localhost:6379"),
		// 9094 is the broker's PLAINTEXT_HOST listener. Containers on the shared
		// bridge use kafka:9092 instead; recipes run as host binaries, so the
		// host listener is the right default.
		KafkaBrokers:   splitList(getenv("KAFKA_BROKERS", "localhost:9094")),
		OTLPEndpoint:   getenv("OTLP_ENDPOINT", "localhost:4318"),
		LogLevel:       getenv("LOG_LEVEL", "info"),
		TracingEnabled: getenv("TRACING_ENABLED", "true") != "false",
	}
}

// splitList turns a comma-separated env value into a trimmed, non-empty slice.
func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))

	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}

	return out
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	return def
}
