// Package config loads the flashsale-specific settings.
package config

import (
	"os"
	"strconv"

	pkgconfig "github.com/raphaeldiscky/distributed-cookbook/pkg/config"
	"github.com/raphaeldiscky/distributed-cookbook/recipes/flashsale/internal/repository"
)

// Config is the flashsale recipe's full config, combining shared + recipe-local settings.
type Config struct {
	Shared      pkgconfig.Shared
	DefaultKind repository.Kind // RECIPE_FLASHSALE_ADAPTER: naive|pg_cond|redis_lua
	Port        int             // RECIPE_FLASHSALE_PORT, default 8081
}

// Load pulls env vars and applies defaults.
func Load() Config {
	return Config{
		Shared: pkgconfig.LoadShared(),
		DefaultKind: repository.Kind(
			getenv("RECIPE_FLASHSALE_ADAPTER", string(repository.KindPgCond)),
		),
		Port: getint("RECIPE_FLASHSALE_PORT", 8081),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	return def
}

func getint(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}

	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}

	return n
}
