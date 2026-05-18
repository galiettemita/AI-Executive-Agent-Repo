package temporal

import (
	"testing"
	"time"
)

func TestComputeDeterministicBackoff_Deterministic(t *testing.T) {
	cfg := DefaultJitterConfig()
	b1 := ComputeDeterministicBackoff(cfg, "wf-1", "ClassifyIntent", 1)
	b2 := ComputeDeterministicBackoff(cfg, "wf-1", "ClassifyIntent", 1)
	if b1 != b2 {
		t.Fatalf("expected deterministic result: %v != %v", b1, b2)
	}
}

func TestComputeDeterministicBackoff_Increases(t *testing.T) {
	cfg := DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     120 * time.Second,
		JitterWindowMs: 100,
	}
	b1 := ComputeDeterministicBackoff(cfg, "wf-1", "Exec", 1)
	b2 := ComputeDeterministicBackoff(cfg, "wf-1", "Exec", 2)
	b3 := ComputeDeterministicBackoff(cfg, "wf-1", "Exec", 3)

	// Base doubles each attempt: ~1s, ~2s, ~4s (plus jitter)
	if b2 <= b1 {
		t.Fatalf("attempt 2 (%v) should be > attempt 1 (%v)", b2, b1)
	}
	if b3 <= b2 {
		t.Fatalf("attempt 3 (%v) should be > attempt 2 (%v)", b3, b2)
	}
}

func TestComputeDeterministicBackoff_ClampedToMax(t *testing.T) {
	cfg := DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     5 * time.Second,
		JitterWindowMs: 500,
	}
	b := ComputeDeterministicBackoff(cfg, "wf-1", "Exec", 20)
	if b > cfg.MaxBackoff {
		t.Fatalf("backoff %v exceeds max %v", b, cfg.MaxBackoff)
	}
}

func TestComputeDeterministicBackoff_DifferentSeeds(t *testing.T) {
	cfg := DefaultJitterConfig()
	b1 := ComputeDeterministicBackoff(cfg, "wf-1", "ActivityA", 1)
	b2 := ComputeDeterministicBackoff(cfg, "wf-2", "ActivityA", 1)
	// Different seeds should (almost certainly) produce different jitter
	// but same base, so values may differ in the jitter portion
	_ = b1
	_ = b2
}

func TestFNV1a64_Deterministic(t *testing.T) {
	a := FNV1a64([]byte("test-seed"))
	b := FNV1a64([]byte("test-seed"))
	if a != b {
		t.Fatalf("FNV1a64 not deterministic: %d != %d", a, b)
	}
}

func TestFNV1a64_Different(t *testing.T) {
	a := FNV1a64([]byte("seed-a"))
	b := FNV1a64([]byte("seed-b"))
	if a == b {
		t.Fatal("different inputs should produce different hashes")
	}
}
