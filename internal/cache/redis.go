package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/at/smartcdn/internal/config"
	"github.com/redis/go-redis/v9"
)

// ErrMiss indicates the requested key was not found in cache.
var ErrMiss = errors.New("cache miss")

// Cache wraps a Redis client for storing processed image variants.
type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

// NewCache creates a new Cache connected to Redis using the provided config.
func NewCache(cfg *config.Config) *Cache {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	return &Cache{
		client: client,
		ttl:    time.Duration(cfg.CacheTTLSeconds) * time.Second,
	}
}

// BuildKey returns a cache key in the format "smartcdn:{imageID}:{deviceClass}:{format}".
func BuildKey(imageID, deviceClass, format string) string {
	return fmt.Sprintf("smartcdn:%s:%s:%s", imageID, deviceClass, format)
}

// Get retrieves cached image bytes by key. Returns ErrMiss if the key does not exist.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	if err != nil {
		return nil, fmt.Errorf("cache get %q: %w", key, err)
	}
	return data, nil
}

// Set stores image bytes in the cache with the configured TTL.
func (c *Cache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("cache set %q: %w", key, err)
	}
	return nil
}

// DefaultTTL returns the configured default TTL for cache entries.
func (c *Cache) DefaultTTL() time.Duration {
	return c.ttl
}

// Ping checks Redis connectivity.
func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
