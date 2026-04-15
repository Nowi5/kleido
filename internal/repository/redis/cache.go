package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"kleido/internal/repository"

	"github.com/redis/go-redis/v9"
)

type cacheRepo struct {
	rdb *redis.Client
}

// NewCacheRepo returns a repository.CacheRepository backed by Redis.
func NewCacheRepo(rdb *redis.Client) repository.CacheRepository {
	return &cacheRepo{rdb: rdb}
}

// SetJSON marshals v to JSON and stores it under key with the given TTL.
func (c *cacheRepo) SetJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("cache: marshal: %w", err)
	}
	if err := c.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache: set: %w", err)
	}
	return nil
}

// GetJSON retrieves the JSON value stored at key and unmarshals it into dest.
// Returns redis.Nil unchanged when the key does not exist so callers can
// distinguish a cache miss from other errors.
func (c *cacheRepo) GetJSON(ctx context.Context, key string, dest any) error {
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err //nolint:wrapcheck // redis.Nil is returned as-is so callers can detect cache misses
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("cache: unmarshal: %w", err)
	}
	return nil
}

// Delete removes one or more keys.
func (c *cacheRepo) Delete(ctx context.Context, keys ...string) error {
	if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
		return fmt.Errorf("cache: delete: %w", err)
	}
	return nil
}

// Exists reports whether the key is present in the cache.
func (c *cacheRepo) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("cache: exists: %w", err)
	}
	return n > 0, nil
}
