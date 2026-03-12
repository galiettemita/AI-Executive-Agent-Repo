package llm

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/brevio/brevio/internal/cache"
	"github.com/redis/go-redis/v9"
)

func newTestLLMServiceWithRedis(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rc := cache.NewRedisClientFromRDB(rdb, 10*time.Minute)

	svc := NewService()
	svc.SetRedisCache(rc)
	return svc, mr
}

// TestServiceReplayCache_RedisHitPreventsSecondCall verifies that when Redis
// is configured, the second Generate call returns FromReplay=true from Redis.
func TestServiceReplayCache_RedisHitPreventsSecondCall(t *testing.T) {
	t.Parallel()
	svc, _ := newTestLLMServiceWithRedis(t)

	req := Request{
		WorkspaceID: "ws-replay-test",
		PromptKey:   "classify",
		Input:       "schedule a meeting",
		Tier:        "T1",
		ModelID:     "claude-haiku-4-5-20250929",
		ProviderID:  "anthropic",
	}

	// First call — cache miss, generates plan.
	resp1 := svc.Generate(req)
	if resp1.FromReplay {
		t.Fatal("expected first call to be a miss")
	}
	if resp1.RequestHash == "" {
		t.Fatal("expected non-empty request hash")
	}

	// Second call — same request, should hit Redis cache.
	resp2 := svc.Generate(req)
	if !resp2.FromReplay {
		t.Fatal("expected second call to be a replay HIT from Redis")
	}
	if resp2.RequestHash != resp1.RequestHash {
		t.Errorf("hash mismatch: %q vs %q", resp1.RequestHash, resp2.RequestHash)
	}
	if resp2.PlanJSON != resp1.PlanJSON {
		t.Error("plan JSON should be identical on replay")
	}

	if svc.ReplayHitCount() != 1 {
		t.Errorf("expected 1 replay hit, got %d", svc.ReplayHitCount())
	}

	t.Logf("Replay HIT confirmed: hash=%s", resp2.RequestHash)
}

// TestServiceReplayCache_FallbackToInMemoryWhenRedisDown verifies that if Redis
// goes down, the in-memory cache still works.
func TestServiceReplayCache_FallbackToInMemoryWhenRedisDown(t *testing.T) {
	t.Parallel()
	svc, mr := newTestLLMServiceWithRedis(t)

	req := Request{
		WorkspaceID: "ws-fallback",
		PromptKey:   "plan",
		Input:       "send email",
		Tier:        "T2",
		ModelID:     "claude-sonnet-4-20250514",
		ProviderID:  "anthropic",
	}

	// First call with Redis up — populates both Redis and in-memory.
	resp1 := svc.Generate(req)
	if resp1.FromReplay {
		t.Fatal("expected miss on first call")
	}

	// Shut down Redis.
	mr.Close()

	// Second call — Redis fails, but in-memory should still work.
	resp2 := svc.Generate(req)
	if !resp2.FromReplay {
		t.Fatal("expected replay hit from in-memory fallback")
	}
}

// TestServiceReplayCache_CrossInstanceIdempotency simulates two service
// instances sharing Redis — the second instance finds the first's cached entry.
func TestServiceReplayCache_CrossInstanceIdempotency(t *testing.T) {
	t.Parallel()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rc := cache.NewRedisClientFromRDB(rdb, 10*time.Minute)

	// Instance A stores.
	svcA := NewService()
	svcA.SetRedisCache(rc)

	// Instance B shares the same Redis.
	svcB := NewService()
	svcB.SetRedisCache(rc)

	req := Request{
		WorkspaceID: "ws-cross",
		PromptKey:   "classify",
		Input:       "hello world",
		Tier:        "T0",
		ModelID:     "claude-haiku-4-5-20250929",
		ProviderID:  "anthropic",
	}

	// Instance A generates.
	respA := svcA.Generate(req)
	if respA.FromReplay {
		t.Fatal("instance A should miss")
	}

	// Instance B should hit from Redis (its in-memory cache is empty).
	respB := svcB.Generate(req)
	if !respB.FromReplay {
		t.Fatal("instance B should hit Redis from instance A's cached entry")
	}
	if respB.PlanJSON != respA.PlanJSON {
		t.Error("plan JSON should match across instances")
	}
}
