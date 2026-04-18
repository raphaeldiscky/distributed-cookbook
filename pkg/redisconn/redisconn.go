// Package redisconn builds a single-node Redis client with OTel tracing.
package redisconn

import (
	"context"
	"errors"
	"fmt"
	"time"

	redisotel "github.com/redis/go-redis/extra/redisotel/v9"
	redis "github.com/redis/go-redis/v9"
)

// New creates a redis.Client against addr (e.g. "localhost:6379").
// On failure, any Close error from the partially-constructed client is
// joined into the returned error via errors.Join — callers can inspect
// both via errors.Is/As without losing either.
func New(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     25,
	})
	if err := redisotel.InstrumentTracing(client); err != nil {
		return nil, errors.Join(
			fmt.Errorf("redisconn: instrument tracing: %w", err),
			client.Close(),
		)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, errors.Join(
			fmt.Errorf("redisconn: ping: %w", err),
			client.Close(),
		)
	}

	return client, nil
}
