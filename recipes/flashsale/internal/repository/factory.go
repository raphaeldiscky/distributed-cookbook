package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	redis "github.com/redis/go-redis/v9"
)

// New returns the Inventory implementation matching kind.
//
// KindRedisLua requires rdb; the other two accept nil.
func New(kind Kind, pool *pgxpool.Pool, rdb *redis.Client) (Inventory, error) {
	switch kind {
	case KindNaive:
		return NewNaive(pool), nil
	case KindPgCond:
		return NewPgCond(pool), nil
	case KindRedisLua:
		if rdb == nil {
			return nil, errors.New("repository: redis_lua requires a non-nil redis client")
		}

		return NewRedisLua(pool, rdb), nil
	default:
		return nil, fmt.Errorf("repository: unknown kind %q", kind)
	}
}
