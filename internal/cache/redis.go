// Package cache provides a Redis-backed caching layer for LLM replay,
// provider rate limiting, and feature flag storage.
package cache

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultReplayTTL is the default time-to-live for LLM replay cache entries.
const DefaultReplayTTL = 24 * time.Hour

// RedisClient wraps go-redis and exposes the minimal interface needed by
// the LLM service, rate limiter, and feature flags subsystems.
type RedisClient struct {
	rdb      *redis.Client
	replayTTL time.Duration
}

// RedisConfig holds the configuration for connecting to Redis.
type RedisConfig struct {
	// URL is the Redis connection string (e.g. redis://host:port/db).
	URL string
	// PoolSize overrides the default connection pool size.
	PoolSize int
	// DialTimeout overrides the default dial timeout.
	DialTimeout time.Duration
	// ReadTimeout overrides the default read timeout.
	ReadTimeout time.Duration
	// WriteTimeout overrides the default write timeout.
	WriteTimeout time.Duration
	// ReplayTTL configures the TTL for LLM replay cache entries.
	// Defaults to DefaultReplayTTL if zero.
	ReplayTTL time.Duration
}

// ParseRedisURL parses a Redis URL into go-redis Options.
// Supports redis:// and rediss:// schemes.
func ParseRedisURL(rawURL string) (*redis.Options, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return nil, fmt.Errorf("cache: empty REDIS_URL")
	}

	// go-redis has its own URL parser; use it directly.
	opts, err := redis.ParseURL(trimmed)
	if err != nil {
		// Fallback: try to parse as a simple host:port.
		parsed, parseErr := url.Parse(trimmed)
		if parseErr != nil || parsed.Host == "" {
			return nil, fmt.Errorf("cache: invalid REDIS_URL: %w", err)
		}
		host := parsed.Host
		if !strings.Contains(host, ":") {
			host += ":6379"
		}
		opts = &redis.Options{Addr: host}
		if parsed.User != nil {
			opts.Password, _ = parsed.User.Password()
		}
	}
	return opts, nil
}

// NewRedisClient creates a new RedisClient from a RedisConfig.
func NewRedisClient(cfg RedisConfig) (*RedisClient, error) {
	opts, err := ParseRedisURL(cfg.URL)
	if err != nil {
		return nil, err
	}

	if cfg.PoolSize > 0 {
		opts.PoolSize = cfg.PoolSize
	}
	if cfg.DialTimeout > 0 {
		opts.DialTimeout = cfg.DialTimeout
	} else {
		opts.DialTimeout = 5 * time.Second
	}
	if cfg.ReadTimeout > 0 {
		opts.ReadTimeout = cfg.ReadTimeout
	} else {
		opts.ReadTimeout = 3 * time.Second
	}
	if cfg.WriteTimeout > 0 {
		opts.WriteTimeout = cfg.WriteTimeout
	} else {
		opts.WriteTimeout = 3 * time.Second
	}

	replayTTL := cfg.ReplayTTL
	if replayTTL <= 0 {
		replayTTL = DefaultReplayTTL
	}

	rdb := redis.NewClient(opts)
	return &RedisClient{rdb: rdb, replayTTL: replayTTL}, nil
}

// NewRedisClientFromRDB wraps an existing go-redis client (useful for tests with miniredis).
func NewRedisClientFromRDB(rdb *redis.Client, replayTTL time.Duration) *RedisClient {
	if replayTTL <= 0 {
		replayTTL = DefaultReplayTTL
	}
	return &RedisClient{rdb: rdb, replayTTL: replayTTL}
}

// Ping validates that the Redis connection is alive.
func (c *RedisClient) Ping(ctx context.Context) error {
	result, err := c.rdb.Ping(ctx).Result()
	if err != nil {
		return fmt.Errorf("cache: redis ping failed: %w", err)
	}
	if result != "PONG" {
		return fmt.Errorf("cache: unexpected ping response: %s", result)
	}
	return nil
}

// Close closes the underlying Redis connection.
func (c *RedisClient) Close() error {
	return c.rdb.Close()
}

// --- Replay cache (LLM idempotency) ---

// ReplayGet retrieves a cached LLM response by request hash.
// Returns the cached value and true if found, empty string and false on miss.
func (c *RedisClient) ReplayGet(ctx context.Context, hash string) (string, bool, error) {
	key := "replay:" + hash
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cache: replay get: %w", err)
	}
	return val, true, nil
}

// ReplaySet stores an LLM response in the replay cache with the configured TTL.
func (c *RedisClient) ReplaySet(ctx context.Context, hash, value string) error {
	key := "replay:" + hash
	return c.rdb.Set(ctx, key, value, c.replayTTL).Err()
}

// ReplayTTL returns the configured TTL for replay cache entries.
func (c *RedisClient) ReplayTTL() time.Duration {
	return c.replayTTL
}

// --- Rate limiting (fixed-window counters) ---
//
// Key format follows existing patterns in internal/llm/providers.go:
//   rl:llm:{provider_id}:{workspace_id}:req:{window}
//   rl:llm:{provider_id}:{workspace_id}:tok:{window}

// RateLimitError is returned when a rate limit is exceeded.
// It is typed so Temporal retry policies can classify it as retryable.
type RateLimitError struct {
	ProviderID  string
	WorkspaceID string
	LimitType   string // "requests" or "tokens"
	Limit       int
	Current     int64
	WindowKey   string
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("cache: rate limit exceeded for %s/%s: %s limit %d (current %d, window %s)",
		e.ProviderID, e.WorkspaceID, e.LimitType, e.Limit, e.Current, e.WindowKey)
}

// CheckRateLimit atomically increments a fixed-window counter and checks
// whether the limit has been exceeded. The window resets every minute.
// Returns nil if the request is allowed, or a *RateLimitError if not.
func (c *RedisClient) CheckRateLimit(ctx context.Context, providerID, workspaceID string, requestsPerMin, tokensPerMin, tokenCount int) error {
	now := time.Now().UTC()
	window := now.Format("2006-01-02T15:04") // 1-minute window

	// Check requests/minute.
	reqKey := fmt.Sprintf("rl:llm:%s:%s:req:%s", providerID, workspaceID, window)
	reqCount, err := c.rdb.Incr(ctx, reqKey).Result()
	if err != nil {
		return fmt.Errorf("cache: rate limit incr: %w", err)
	}
	// Set TTL on first increment.
	if reqCount == 1 {
		c.rdb.Expire(ctx, reqKey, 2*time.Minute)
	}
	if requestsPerMin > 0 && reqCount > int64(requestsPerMin) {
		return &RateLimitError{
			ProviderID:  providerID,
			WorkspaceID: workspaceID,
			LimitType:   "requests",
			Limit:       requestsPerMin,
			Current:     reqCount,
			WindowKey:   window,
		}
	}

	// Check tokens/minute.
	if tokenCount > 0 && tokensPerMin > 0 {
		tokKey := fmt.Sprintf("rl:llm:%s:%s:tok:%s", providerID, workspaceID, window)
		tokCount, err := c.rdb.IncrBy(ctx, tokKey, int64(tokenCount)).Result()
		if err != nil {
			return fmt.Errorf("cache: rate limit incrby: %w", err)
		}
		if tokCount == int64(tokenCount) {
			c.rdb.Expire(ctx, tokKey, 2*time.Minute)
		}
		if tokCount > int64(tokensPerMin) {
			return &RateLimitError{
				ProviderID:  providerID,
				WorkspaceID: workspaceID,
				LimitType:   "tokens",
				Limit:       tokensPerMin,
				Current:     tokCount,
				WindowKey:   window,
			}
		}
	}

	return nil
}

// --- Feature flags ---
//
// Key format: ff:{flag_key}  (stores JSON-encoded flag state)
// Key format: ff:rules:{flag_key} (stores JSON-encoded rules)

// FeatureFlagGet retrieves a feature flag value from Redis.
func (c *RedisClient) FeatureFlagGet(ctx context.Context, flagKey string) (string, bool, error) {
	key := "ff:" + flagKey
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("cache: feature flag get: %w", err)
	}
	return val, true, nil
}

// FeatureFlagSet stores a feature flag value in Redis with a TTL.
func (c *RedisClient) FeatureFlagSet(ctx context.Context, flagKey, value string, ttl time.Duration) error {
	key := "ff:" + flagKey
	return c.rdb.Set(ctx, key, value, ttl).Err()
}

// FeatureFlagDel removes a feature flag from Redis.
func (c *RedisClient) FeatureFlagDel(ctx context.Context, flagKey string) error {
	key := "ff:" + flagKey
	return c.rdb.Del(ctx, key).Err()
}

// RDB returns the underlying go-redis client for advanced operations.
func (c *RedisClient) RDB() *redis.Client {
	return c.rdb
}
