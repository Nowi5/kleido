// Package redis provides go-redis backed implementations of the repository interfaces.
package redis

import (
	"context"
	"fmt"

	"github.com/nowi5/kleido/internal/config"
	"github.com/redis/go-redis/v9"
)

// NewClient creates a go-redis client configured from cfg and verifies
// connectivity with a PING. Returns an error if the ping fails.
func NewClient(cfg config.RedisConfig) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		_ = rdb.Close() //nolint:errcheck // best-effort cleanup on ping failure
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	return rdb, nil
}
