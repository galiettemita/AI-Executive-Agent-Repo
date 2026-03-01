package determinism

import (
	"testing"
	"time"
)

func TestDeterministicJitterIsStableAndBounded(t *testing.T) {
	t.Parallel()

	j1 := DeterministicJitterSeconds("ws_123", "memory_consolidation")
	j2 := DeterministicJitterSeconds("ws_123", "memory_consolidation")
	if j1 != j2 {
		t.Fatalf("expected stable jitter, got %d vs %d", j1, j2)
	}
	if j1 < 0 || j1 > 50 {
		t.Fatalf("expected jitter range [0,50], got %d", j1)
	}
}

func TestApplyDeterministicJitter(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	jittered := ApplyDeterministicJitter(base, "ws_123", "token_refresh")
	diff := jittered.Sub(base)
	if diff < 0 || diff > 50*time.Second {
		t.Fatalf("expected jitter diff <=50s, got %s", diff)
	}
}
