package fastpath_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/fastpath"
)

type mockRedisCache struct {
	store map[string]string
}

func newMockRedisCache() *mockRedisCache {
	return &mockRedisCache{store: map[string]string{}}
}

func (m *mockRedisCache) Get(_ context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", fmt.Errorf("cache miss")
	}
	return v, nil
}

func (m *mockRedisCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.store[key] = value
	return nil
}

func TestFastPathService_DefaultRoutes_MatchHello(t *testing.T) {
	t.Parallel()
	svc := fastpath.NewFastPathService()
	require.NoError(t, fastpath.SeedDefaultRoutes(svc))
	result, ok := svc.Match("hello")
	assert.True(t, ok)
	assert.NotEmpty(t, result.Response)
}

func TestFastPathService_DefaultRoutes_MatchThankYou(t *testing.T) {
	t.Parallel()
	svc := fastpath.NewFastPathService()
	require.NoError(t, fastpath.SeedDefaultRoutes(svc))
	result, ok := svc.Match("thank you!")
	assert.True(t, ok)
	assert.NotEmpty(t, result.Response)
}

func TestFastPathService_NoMatch_ReturnsFalse(t *testing.T) {
	t.Parallel()
	svc := fastpath.NewFastPathService()
	require.NoError(t, fastpath.SeedDefaultRoutes(svc))
	_, ok := svc.Match("book me a flight to Tokyo next Tuesday")
	assert.False(t, ok)
}

func TestFastPathService_RedisCache_WritesOnMatch(t *testing.T) {
	t.Parallel()
	svc := fastpath.NewFastPathService()
	require.NoError(t, fastpath.SeedDefaultRoutes(svc))
	cache := newMockRedisCache()
	svc.WithRedisCache(cache)
	_, ok := svc.Match("hi")
	assert.True(t, ok)
	assert.Greater(t, len(cache.store), 0, "cache should have at least one entry after match")
}

func TestFastPathService_RedisCache_ServesFromCache(t *testing.T) {
	t.Parallel()
	svc := fastpath.NewFastPathService()
	cache := newMockRedisCache()
	svc.WithRedisCache(cache)
	// First call: no routes registered → no match
	_, ok := svc.Match("hi")
	assert.False(t, ok)
	// Now add routes and match to populate cache
	require.NoError(t, fastpath.SeedDefaultRoutes(svc))
	result1, ok1 := svc.Match("hi")
	assert.True(t, ok1)
	// Second call: served from cache
	result2, ok2 := svc.Match("hi")
	assert.True(t, ok2)
	assert.True(t, result2.FromCache)
	assert.Equal(t, result1.Response, result2.Response)
}
