// Package redisconn builds a single-node Redis client with OTel tracing.
package redisconn

import (
	"context"
	"fmt"
	"time"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	redis "github.com/redis/go-redis/v9"
)

// New creates a redis.Client against addr (e.g. "localhost:6379").
func New(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     25,
	})
	if err := redisotel.InstrumentTracing(client); err != nil {
		client.Close() //nolint:errcheck,gosec // cleanup path — construction error already dominates

		return nil, fmt.Errorf("redisconn: instrument tracing: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close() //nolint:errcheck,gosec // cleanup path — ping error already dominates

		return nil, fmt.Errorf("redisconn: ping: %w", err)
	}

	return client, nil
}
