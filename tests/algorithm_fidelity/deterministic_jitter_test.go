package algorithm_fidelity

import (
	"fmt"
	"testing"
	"time"

	temporal "github.com/brevio/brevio/internal/temporal"
)

func TestDeterministicJitter_FormulaCompliance(t *testing.T) {
	cfg := temporal.DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     60 * time.Second,
		JitterWindowMs: 1000,
	}

	// Same inputs must always produce same output (determinism)
	for i := 0; i < 100; i++ {
		b1 := temporal.ComputeDeterministicBackoff(cfg, "wf-test", "activity-test", 3)
		b2 := temporal.ComputeDeterministicBackoff(cfg, "wf-test", "activity-test", 3)
		if b1 != b2 {
			t.Fatalf("iteration %d: non-deterministic result %v != %v", i, b1, b2)
		}
	}
}

func TestDeterministicJitter_ExponentialGrowth(t *testing.T) {
	cfg := temporal.DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     120 * time.Second,
		JitterWindowMs: 0, // no jitter to test pure exponential
	}

	b1 := temporal.ComputeDeterministicBackoff(cfg, "wf", "act", 1)
	b2 := temporal.ComputeDeterministicBackoff(cfg, "wf", "act", 2)
	b3 := temporal.ComputeDeterministicBackoff(cfg, "wf", "act", 3)

	if b1 != time.Second {
		t.Errorf("attempt 1: expected 1s, got %v", b1)
	}
	if b2 != 2*time.Second {
		t.Errorf("attempt 2: expected 2s, got %v", b2)
	}
	if b3 != 4*time.Second {
		t.Errorf("attempt 3: expected 4s, got %v", b3)
	}
}

func TestDeterministicJitter_MaxClamp(t *testing.T) {
	cfg := temporal.DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     5 * time.Second,
		JitterWindowMs: 500,
	}

	for attempt := 1; attempt <= 30; attempt++ {
		b := temporal.ComputeDeterministicBackoff(cfg, "wf", "act", attempt)
		if b > cfg.MaxBackoff {
			t.Fatalf("attempt %d: backoff %v exceeds max %v", attempt, b, cfg.MaxBackoff)
		}
	}
}

func TestDeterministicJitter_SeedVariation(t *testing.T) {
	cfg := temporal.DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     60 * time.Second,
		JitterWindowMs: 10000, // large window to see variation
	}

	// Different workflow IDs should produce different jitter
	results := make(map[time.Duration]bool)
	for i := 0; i < 50; i++ {
		wfID := fmt.Sprintf("wf-%d", i)
		b := temporal.ComputeDeterministicBackoff(cfg, wfID, "act", 1)
		results[b] = true
	}

	// With 50 different seeds and 10s jitter window, we should get multiple distinct values
	if len(results) < 5 {
		t.Errorf("expected more variation across seeds, got only %d distinct values", len(results))
	}
}
