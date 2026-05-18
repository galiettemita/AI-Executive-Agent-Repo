package executor

import "testing"

func TestRateCoordinatorAcquireAndRelease(t *testing.T) {
	t.Parallel()

	rc := NewRateCoordinator()
	rc.SetLimit("openai", RateLimitConfig{Provider: "openai", MaxConcurrent: 2, WindowMs: 1000})

	if !rc.Acquire("openai") {
		t.Fatal("expected first acquire to succeed")
	}
	if !rc.Acquire("openai") {
		t.Fatal("expected second acquire to succeed")
	}
	if rc.Acquire("openai") {
		t.Fatal("expected third acquire to fail (at capacity)")
	}

	rc.Release("openai")
	if !rc.Acquire("openai") {
		t.Fatal("expected acquire after release to succeed")
	}
}

func TestRateCoordinatorDefaultLimit(t *testing.T) {
	t.Parallel()

	rc := NewRateCoordinator()
	// No limit set, should use default of 10.
	if !rc.Acquire("unknown_provider") {
		t.Fatal("expected acquire with default limit to succeed")
	}
	if rc.CurrentUsage("unknown_provider") != 1 {
		t.Fatalf("expected usage 1, got %d", rc.CurrentUsage("unknown_provider"))
	}
}

func TestRateCoordinatorMultipleProviders(t *testing.T) {
	t.Parallel()

	rc := NewRateCoordinator()
	rc.SetLimit("openai", RateLimitConfig{MaxConcurrent: 1})
	rc.SetLimit("anthropic", RateLimitConfig{MaxConcurrent: 1})

	if !rc.Acquire("openai") {
		t.Fatal("expected openai acquire to succeed")
	}
	if rc.Acquire("openai") {
		t.Fatal("expected openai second acquire to fail")
	}
	if !rc.Acquire("anthropic") {
		t.Fatal("expected anthropic acquire to succeed (independent)")
	}
}

func TestRateCoordinatorReleaseUnknown(t *testing.T) {
	t.Parallel()

	rc := NewRateCoordinator()
	// Should not panic.
	rc.Release("nonexistent")
}

func TestRateCoordinatorCurrentUsage(t *testing.T) {
	t.Parallel()

	rc := NewRateCoordinator()
	rc.SetLimit("provider", RateLimitConfig{MaxConcurrent: 5})

	if rc.CurrentUsage("provider") != 0 {
		t.Fatalf("expected 0 usage, got %d", rc.CurrentUsage("provider"))
	}

	rc.Acquire("provider")
	rc.Acquire("provider")
	if rc.CurrentUsage("provider") != 2 {
		t.Fatalf("expected 2 usage, got %d", rc.CurrentUsage("provider"))
	}

	rc.Release("provider")
	if rc.CurrentUsage("provider") != 1 {
		t.Fatalf("expected 1 usage after release, got %d", rc.CurrentUsage("provider"))
	}
}
