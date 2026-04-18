// Package config loads shared infrastructure settings from env vars.
// Recipe-specific settings live in each recipe's own config package.
package config

import "os"

// Shared carries infra connection settings used by every recipe.
type Shared struct {
	PostgresDSN  string // POSTGRES_DSN, default "postgres://cookbook:cookbook@localhost:5433/cookbook_db?sslmode=disable"
	RedisAddr    string // REDIS_ADDR,    default "localhost:6379"
	OTLPEndpoint string // OTLP_ENDPOINT, default "localhost:4318" (collector OTLP HTTP)
	LogLevel     string // LOG_LEVEL,     default "info"
}

// LoadShared reads env vars with sensible local-dev defaults.
func LoadShared() Shared {
	return Shared{
		PostgresDSN: getenv(
			"POSTGRES_DSN",
			"postgres://cookbook:cookbook@localhost:5433/cookbook_db?sslmode=disable",
		),
		RedisAddr:    getenv("REDIS_ADDR", "localhost:6379"),
		OTLPEndpoint: getenv("OTLP_ENDPOINT", "localhost:4318"),
		LogLevel:     getenv("LOG_LEVEL", "info"),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	return def
}
