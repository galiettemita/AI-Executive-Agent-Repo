package cache

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisClient(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rc := NewRedisClientFromRDB(rdb, 10*time.Minute)
	return rc, mr
}

// ---------------------------------------------------------------------------
// URL Parsing
// ---------------------------------------------------------------------------

func TestParseRedisURL_Valid(t *testing.T) {
	t.Parallel()

	cases := []struct {
		url      string
		wantAddr string
	}{
		{"redis://localhost:6379", "localhost:6379"},
		{"redis://localhost:6379/0", "localhost:6379"},
		{"redis://:password@host:6380/2", "host:6380"},
		{"rediss://secure.host:6381", "secure.host:6381"},
	}

	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			opts, err := ParseRedisURL(tc.url)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if opts.Addr != tc.wantAddr {
				t.Errorf("expected addr %q, got %q", tc.wantAddr, opts.Addr)
			}
		})
	}
}

func TestParseRedisURL_Empty(t *testing.T) {
	t.Parallel()
	_, err := ParseRedisURL("")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestParseRedisURL_Invalid(t *testing.T) {
	t.Parallel()
	_, err := ParseRedisURL("://")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

// ---------------------------------------------------------------------------
// Ping
// ---------------------------------------------------------------------------

func TestPing_Success(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)
	if err := rc.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
}

func TestPing_Failure(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rc := NewRedisClientFromRDB(rdb, time.Minute)
	mr.Close()
	if err := rc.Ping(context.Background()); err == nil {
		t.Fatal("expected ping error after close")
	}
}

// ---------------------------------------------------------------------------
// Replay Cache
// ---------------------------------------------------------------------------

func TestReplayCache_MissThenHit(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	hash := "abc123"

	// Miss.
	val, ok, err := rc.ReplayGet(ctx, hash)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if ok {
		t.Fatal("expected miss on empty cache")
	}
	if val != "" {
		t.Errorf("expected empty value, got %q", val)
	}

	// Set.
	plan := `{"plan_id":"plan-001","provider_id":"anthropic","created_at":"2026-03-12T00:00:00Z"}`
	if err := rc.ReplaySet(ctx, hash, plan); err != nil {
		t.Fatalf("set error: %v", err)
	}

	// Hit.
	val, ok, err = rc.ReplayGet(ctx, hash)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !ok {
		t.Fatal("expected hit after set")
	}
	if val != plan {
		t.Errorf("expected %q, got %q", plan, val)
	}
}

func TestReplayCache_TTLExpiry(t *testing.T) {
	t.Parallel()
	rc, mr := newTestRedisClient(t)

	ctx := context.Background()
	hash := "expire-test"
	if err := rc.ReplaySet(ctx, hash, "data"); err != nil {
		t.Fatalf("set error: %v", err)
	}

	// Fast-forward past TTL.
	mr.FastForward(rc.ReplayTTL() + time.Second)

	_, ok, err := rc.ReplayGet(ctx, hash)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestReplayCache_PreventsSecondProviderCall(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	hash := "idempotent-001"
	plan := `{"plan":"cached_result"}`

	// Simulate first call: miss → store.
	_, ok, _ := rc.ReplayGet(ctx, hash)
	if ok {
		t.Fatal("unexpected hit on first call")
	}
	rc.ReplaySet(ctx, hash, plan)

	// Simulate second call (Activity retry): should hit cache.
	val, ok, _ := rc.ReplayGet(ctx, hash)
	if !ok {
		t.Fatal("expected cache hit on retry — second provider call would happen")
	}
	if val != plan {
		t.Errorf("expected cached plan, got %q", val)
	}
	t.Logf("Replay HIT confirmed: second provider call prevented (key=replay:%s)", hash)
}

// ---------------------------------------------------------------------------
// Rate Limiting
// ---------------------------------------------------------------------------

func TestRateLimit_RequestsPerMinute(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	limit := 3

	// First 3 requests should pass.
	for i := 0; i < limit; i++ {
		err := rc.CheckRateLimit(ctx, "anthropic", "ws-001", limit, 0, 0)
		if err != nil {
			t.Fatalf("request %d should be allowed: %v", i+1, err)
		}
	}

	// 4th request should be denied.
	err := rc.CheckRateLimit(ctx, "anthropic", "ws-001", limit, 0, 0)
	if err == nil {
		t.Fatal("expected rate limit error on 4th request")
	}
	rlErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}
	if rlErr.LimitType != "requests" {
		t.Errorf("expected limit type 'requests', got %q", rlErr.LimitType)
	}
	if rlErr.ProviderID != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", rlErr.ProviderID)
	}
}

func TestRateLimit_TokensPerMinute(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	tokLimit := 1000

	// First call with 600 tokens — should pass.
	err := rc.CheckRateLimit(ctx, "openai", "ws-002", 0, tokLimit, 600)
	if err != nil {
		t.Fatalf("first batch should pass: %v", err)
	}

	// Second call with 500 tokens — exceeds 1000, should fail.
	err = rc.CheckRateLimit(ctx, "openai", "ws-002", 0, tokLimit, 500)
	if err == nil {
		t.Fatal("expected token rate limit error")
	}
	rlErr, ok := err.(*RateLimitError)
	if !ok {
		t.Fatalf("expected *RateLimitError, got %T", err)
	}
	if rlErr.LimitType != "tokens" {
		t.Errorf("expected limit type 'tokens', got %q", rlErr.LimitType)
	}
}

func TestRateLimit_DifferentWorkspacesIndependent(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	limit := 2

	// ws-A uses its full limit.
	for i := 0; i < limit; i++ {
		rc.CheckRateLimit(ctx, "anthropic", "ws-A", limit, 0, 0)
	}
	// ws-A should be limited.
	err := rc.CheckRateLimit(ctx, "anthropic", "ws-A", limit, 0, 0)
	if err == nil {
		t.Fatal("expected ws-A to be rate limited")
	}

	// ws-B should still be allowed.
	err = rc.CheckRateLimit(ctx, "anthropic", "ws-B", limit, 0, 0)
	if err != nil {
		t.Fatalf("ws-B should be independent: %v", err)
	}
}

func TestRateLimit_ConcurrencySafe(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()
	limit := 100
	goroutines := 50
	requestsPerGoroutine := 3 // 50*3=150 total, limit=100

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	denied := 0

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < requestsPerGoroutine; r++ {
				err := rc.CheckRateLimit(ctx, "anthropic", "ws-concurrent", limit, 0, 0)
				mu.Lock()
				if err == nil {
					allowed++
				} else {
					denied++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed > limit {
		t.Errorf("allowed %d requests, expected max %d", allowed, limit)
	}
	if denied == 0 {
		t.Error("expected some requests to be denied")
	}
	t.Logf("Concurrency test: %d allowed, %d denied (limit=%d)", allowed, denied, limit)
}

func TestRateLimit_WindowExpiry(t *testing.T) {
	t.Parallel()
	rc, mr := newTestRedisClient(t)

	ctx := context.Background()
	limit := 1

	// Use limit.
	rc.CheckRateLimit(ctx, "anthropic", "ws-expire", limit, 0, 0)
	err := rc.CheckRateLimit(ctx, "anthropic", "ws-expire", limit, 0, 0)
	if err == nil {
		t.Fatal("expected rate limit after 1 request")
	}

	// Fast-forward past the 2-minute key TTL.
	mr.FastForward(3 * time.Minute)

	// Should be allowed again in the new window.
	err = rc.CheckRateLimit(ctx, "anthropic", "ws-expire", limit, 0, 0)
	if err != nil {
		t.Fatalf("expected request allowed after window expiry: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Feature Flags
// ---------------------------------------------------------------------------

func TestFeatureFlag_SetGetDel(t *testing.T) {
	t.Parallel()
	rc, _ := newTestRedisClient(t)

	ctx := context.Background()

	// Miss.
	_, ok, err := rc.FeatureFlagGet(ctx, "test_flag")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if ok {
		t.Fatal("expected miss")
	}

	// Set.
	if err := rc.FeatureFlagSet(ctx, "test_flag", `{"enabled":true}`, 5*time.Minute); err != nil {
		t.Fatalf("set error: %v", err)
	}

	// Hit.
	val, ok, err := rc.FeatureFlagGet(ctx, "test_flag")
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !ok || val != `{"enabled":true}` {
		t.Errorf("unexpected: ok=%v val=%q", ok, val)
	}

	// Delete.
	if err := rc.FeatureFlagDel(ctx, "test_flag"); err != nil {
		t.Fatalf("del error: %v", err)
	}

	_, ok, _ = rc.FeatureFlagGet(ctx, "test_flag")
	if ok {
		t.Fatal("expected miss after delete")
	}
}

// ---------------------------------------------------------------------------
// NewRedisClient from config
// ---------------------------------------------------------------------------

func TestNewRedisClient_FromConfig(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	rc, err := NewRedisClient(RedisConfig{
		URL:       "redis://" + mr.Addr(),
		PoolSize:  5,
		ReplayTTL: 30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	if err := rc.Ping(context.Background()); err != nil {
		t.Fatalf("ping failed: %v", err)
	}
	if rc.ReplayTTL() != 30*time.Minute {
		t.Errorf("expected replay TTL 30m, got %v", rc.ReplayTTL())
	}
}

func TestNewRedisClient_DefaultReplayTTL(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)

	rc, err := NewRedisClient(RedisConfig{URL: "redis://" + mr.Addr()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	if rc.ReplayTTL() != DefaultReplayTTL {
		t.Errorf("expected default TTL %v, got %v", DefaultReplayTTL, rc.ReplayTTL())
	}
}
