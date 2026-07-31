package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	redis "github.com/redis/go-redis/v9"
)

// errRedisRequired is returned when a Redis-backed kind is built without a client.
var errRedisRequired = errors.New("requires a non-nil redis client")

// errBrokersRequired is returned when a Kafka-backed kind is built without brokers.
var errBrokersRequired = errors.New("requires at least one kafka broker")

// Deps carries everything any Inventory implementation might need. A struct
// rather than a growing parameter list: the composition root fills it once and
// every kind takes only the fields it uses.
type Deps struct {
	Pool    *pgxpool.Pool
	Redis   *redis.Client
	Brokers []string
	Log     *slog.Logger
}

// New returns the Inventory implementation matching kind.
//
// ctx is retained by the kinds that own background goroutines (KindGoChan's
// owner, KindTokenQueue's consumer), so callers pass the server's lifetime
// context rather than a per-request one.
func New(ctx context.Context, kind Kind, deps Deps) (Inventory, error) {
	switch kind {
	case KindNaive:
		return NewNaive(deps.Pool), nil
	case KindPgCond:
		return NewPgCond(deps.Pool), nil
	case KindPgForUpdate:
		return NewPgForUpdate(deps.Pool), nil
	case KindPgAdvisory:
		return NewPgAdvisory(deps.Pool), nil
	case KindPgOptimistic:
		return NewPgOptimistic(deps.Pool), nil
	case KindPgSerializable:
		return NewPgSerializable(deps.Pool), nil
	case KindPgSkipLocked:
		return NewPgSkipLocked(deps.Pool), nil
	case KindRedisLua:
		if deps.Redis == nil {
			return nil, fmt.Errorf("repository %s: %w", kind, errRedisRequired)
		}

		return NewRedisLua(deps.Pool, deps.Redis), nil
	case KindRedisAtomic:
		if deps.Redis == nil {
			return nil, fmt.Errorf("repository %s: %w", kind, errRedisRequired)
		}

		return NewRedisAtomic(deps.Pool, deps.Redis), nil
	case KindGoChan:
		return NewGoChan(ctx, deps.Pool), nil
	case KindTokenQueue:
		if len(deps.Brokers) == 0 {
			return nil, fmt.Errorf("repository %s: %w", kind, errBrokersRequired)
		}

		return NewTokenQueue(ctx, deps.Pool, deps.Brokers, deps.Log)
	default:
		return nil, fmt.Errorf("repository: unknown kind %q", kind)
	}
}
