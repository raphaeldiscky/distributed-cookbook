// Package config loads the catalog-service settings.
//
// The service is universal — it doesn't know which recipe is using it.
// Only env vars common to every cookbook service: the listen port,
// shared infra settings, and an OTel endpoint.
package config

import (
	"os"
	"strconv"

	pkgconfig "github.com/raphaeldiscky/distributed-cookbook/pkg/config"
)

// Config combines shared infra settings with this service's port.
type Config struct {
	Shared pkgconfig.Shared
	Port   int // SERVICE_PORT, default 8080
}

// Load pulls env vars and applies defaults.
func Load() Config {
	return Config{
		Shared: pkgconfig.LoadShared(),
		Port:   getint("SERVICE_PORT", 8080),
	}
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
