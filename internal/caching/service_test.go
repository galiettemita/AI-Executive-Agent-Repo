package caching

import (
	"testing"
	"time"
)

func TestCachingLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	policy := s.UpsertPolicy(Policy{
		WorkspaceID: "ws_1",
		CacheKey:    "compiled_context",
		TTLSeconds:  600,
		MaxBytes:    1048576,
		Enabled:     true,
	})
	if policy.ID == "" {
		t.Fatalf("expected policy id")
	}

	policies := s.ListPolicies("ws_1")
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}

	s.PutEntry("ws_1", "compiled_context:turn_1", "cached payload")
	s.RecordHit("ws_1")
	s.RecordMiss("ws_1")

	stats := s.Stats("ws_1")
	if stats.Entries != 1 {
		t.Fatalf("expected 1 cache entry, got %#v", stats)
	}
	if stats.HitRate != 0.5 {
		t.Fatalf("expected 0.5 hit rate, got %#v", stats)
	}
	if stats.L2Entries != 1 || stats.L3Entries != 1 {
		t.Fatalf("expected entries replicated across layers, got %#v", stats)
	}

	if !s.Invalidate("ws_1", "compiled_context:turn_1") {
		t.Fatalf("expected invalidation success")
	}
	stats = s.Stats("ws_1")
	if stats.Invalidations != 1 || stats.Entries != 0 {
		t.Fatalf("unexpected stats after invalidation: %#v", stats)
	}
}

func TestCachingLayerPromotionAndTTLExpiry(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertPolicy(Policy{WorkspaceID: "ws_cache", CacheKey: "tool_result", TTLSeconds: 60, MaxBytes: 2048, Enabled: true})
	baseTime := time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC)
	if err := s.PutEntryAt("ws_cache", "tool_result:action_1", "payload", baseTime); err != nil {
		t.Fatalf("put entry: %v", err)
	}

	value, ok := s.GetEntryAt("ws_cache", "tool_result:action_1", baseTime.Add(10*time.Second))
	if !ok || value != "payload" {
		t.Fatalf("expected layer hit before expiry, got value=%q ok=%v", value, ok)
	}

	_, ok = s.GetEntryAt("ws_cache", "tool_result:action_1", baseTime.Add(61*time.Second))
	if ok {
		t.Fatal("expected cache miss after TTL expiry")
	}
	stats := s.Stats("ws_cache")
	if stats.Misses == 0 {
		t.Fatalf("expected miss count after expiry, got %#v", stats)
	}
}

func TestCachingRejectsOversizedEntry(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertPolicy(Policy{WorkspaceID: "ws_limit", CacheKey: "prompt_embedding", TTLSeconds: 120, MaxBytes: 4, Enabled: true})
	if err := s.PutEntryAt("ws_limit", "prompt_embedding:req_1", "toolarge", time.Now().UTC()); err == nil {
		t.Fatal("expected CACHE_ENTRY_TOO_LARGE error")
	}
	stats := s.Stats("ws_limit")
	if stats.Entries != 0 {
		t.Fatalf("expected no entries after rejected write, got %#v", stats)
	}
}
