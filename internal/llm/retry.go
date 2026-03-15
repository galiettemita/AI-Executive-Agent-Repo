package llm

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryBackoff returns exponential backoff with full jitter.
// Formula: uniform(0, min(cap, base × 2^attempt))
// Prevents thundering herd on provider rate limits.
// attempt=0 → 0s, attempt=1 → [0,2s], attempt=2 → [0,4s], attempt=3 → [0,8s].
func RetryBackoff(attempt int, base, cap time.Duration) time.Duration {
	if attempt <= 0 {
		return 0
	}
	ceiling := time.Duration(math.Min(
		float64(cap),
		float64(base)*math.Pow(2, float64(attempt)),
	))
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(ceiling) + 1))
}

// retryAfterDuration parses a Retry-After header value (seconds as float).
// Returns 0 on any parse failure or non-positive value.
func retryAfterDuration(headerVal string) time.Duration {
	var secs float64
	if _, err := fmt.Sscanf(headerVal, "%f", &secs); err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

// isRetryable returns true for errors that warrant a retry or failover.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "context deadline exceeded")
}
