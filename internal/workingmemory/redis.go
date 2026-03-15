package workingmemory

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisClient is the minimal Redis interface required by this package.
type RedisClient interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error) // returns nil, nil on miss
	Del(ctx context.Context, keys ...string) error
	Scan(ctx context.Context, pattern string) ([]string, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
}

// GoRedisAdapter adapts *redis.Client (go-redis/v9) to RedisClient.
type GoRedisAdapter struct {
	c *goredis.Client
}

func NewGoRedisAdapter(c *goredis.Client) *GoRedisAdapter {
	return &GoRedisAdapter{c: c}
}

func (a *GoRedisAdapter) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return a.c.Set(ctx, key, value, ttl).Err()
}

func (a *GoRedisAdapter) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := a.c.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, nil
	}
	return val, err
}

func (a *GoRedisAdapter) Del(ctx context.Context, keys ...string) error {
	return a.c.Del(ctx, keys...).Err()
}

func (a *GoRedisAdapter) Scan(ctx context.Context, pattern string) ([]string, error) {
	var cursor uint64
	var all []string
	for {
		keys, nextCursor, err := a.c.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		all = append(all, keys...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return all, nil
}

func (a *GoRedisAdapter) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return a.c.Expire(ctx, key, ttl).Err()
}
