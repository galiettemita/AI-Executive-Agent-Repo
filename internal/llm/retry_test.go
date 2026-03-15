package llm

import (
	"testing"
	"time"
)

func TestRetryBackoff_ZeroAttempt(t *testing.T) {
	t.Parallel()
	if got := RetryBackoff(0, time.Second, 30*time.Second); got != 0 {
		t.Errorf("attempt 0: want 0, got %v", got)
	}
}

func TestRetryBackoff_InRange(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		got := RetryBackoff(1, time.Second, 30*time.Second)
		if got < 0 || got > 2*time.Second {
			t.Errorf("attempt 1: %v out of [0, 2s]", got)
		}
		got = RetryBackoff(3, time.Second, 30*time.Second)
		if got < 0 || got > 8*time.Second {
			t.Errorf("attempt 3: %v out of [0, 8s]", got)
		}
	}
}

func TestRetryBackoff_CapEnforced(t *testing.T) {
	t.Parallel()
	for i := 0; i < 100; i++ {
		if got := RetryBackoff(20, time.Second, 30*time.Second); got > 30*time.Second {
			t.Errorf("attempt 20: %v exceeds 30s cap", got)
		}
	}
}

func TestRetryAfterDuration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"30", 30 * time.Second},
		{"0.5", 500 * time.Millisecond},
		{"", 0},
		{"invalid", 0},
		{"-1", 0},
	}
	for _, tc := range cases {
		if got := retryAfterDuration(tc.in); got != tc.want {
			t.Errorf("retryAfterDuration(%q): want %v, got %v", tc.in, tc.want, got)
		}
	}
}
