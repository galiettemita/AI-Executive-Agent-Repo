package temporal

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"time"
)

// DeterministicJitterConfig holds parameters for deterministic retry jitter.
type DeterministicJitterConfig struct {
	BaseBackoff    time.Duration
	MaxBackoff     time.Duration
	JitterWindowMs int64
}

// DefaultJitterConfig returns the default jitter configuration.
func DefaultJitterConfig() DeterministicJitterConfig {
	return DeterministicJitterConfig{
		BaseBackoff:    time.Second,
		MaxBackoff:     60 * time.Second,
		JitterWindowMs: 1000,
	}
}

// ComputeDeterministicBackoff computes a deterministic backoff duration using
// fnv1a64 seeded with workflow_id|activity|attempt.
//
// Formula:
//
//	seed = workflow_id | activity | attempt
//	jitter_ms = fnv1a64(seed) % jitter_window_ms
//	backoff_ms = base_backoff_ms * 2^(attempt-1) + jitter_ms
//	clamped to max_backoff_ms
func ComputeDeterministicBackoff(cfg DeterministicJitterConfig, workflowID, activityName string, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	seed := fmt.Sprintf("%s|%s|%d", workflowID, activityName, attempt)
	h := fnv.New64a()
	_, _ = h.Write([]byte(seed))
	hashVal := h.Sum64()

	jitterMs := int64(0)
	if cfg.JitterWindowMs > 0 {
		jitterMs = int64(hashVal % uint64(cfg.JitterWindowMs))
	}

	// base_backoff_ms * 2^(attempt-1)
	baseMs := cfg.BaseBackoff.Milliseconds()
	shift := attempt - 1
	if shift > 30 {
		shift = 30 // prevent overflow
	}
	backoffMs := baseMs << uint(shift)
	backoffMs += jitterMs

	maxMs := cfg.MaxBackoff.Milliseconds()
	if backoffMs > maxMs {
		backoffMs = maxMs
	}

	return time.Duration(backoffMs) * time.Millisecond
}

// FNV1a64 computes the FNV-1a 64-bit hash of a byte slice.
// Exported for verification in tests.
func FNV1a64(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

// FNV1a64Bytes returns the FNV-1a 64-bit hash as a big-endian byte slice.
func FNV1a64Bytes(data []byte) []byte {
	val := FNV1a64(data)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, val)
	return buf
}
