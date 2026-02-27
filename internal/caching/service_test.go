package caching

import "testing"

func TestCachingLifecycle(t *testing.T) {
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

	if !s.Invalidate("ws_1", "compiled_context:turn_1") {
		t.Fatalf("expected invalidation success")
	}
	stats = s.Stats("ws_1")
	if stats.Invalidations != 1 || stats.Entries != 0 {
		t.Fatalf("unexpected stats after invalidation: %#v", stats)
	}
}
