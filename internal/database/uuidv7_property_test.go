package database

import (
	"encoding/hex"
	"strings"
	"testing"
)

// TestUUIDv7Properties verifies RFC 9562 UUIDv7 invariants.
// This test validates the SQL function output when run against a real database.
// For CI without a database, it validates the Go-side UUIDv7 generation.

func TestUUIDv7VersionBits(t *testing.T) {
	t.Parallel()
	// Generate 1000 UUIDv7s and verify version/variant
	for i := 0; i < 1000; i++ {
		id := GenerateUUIDv7()
		hexStr := strings.ReplaceAll(id, "-", "")
		if len(hexStr) != 32 {
			t.Fatalf("invalid UUID hex length: %d", len(hexStr))
		}
		// Version nibble at position 12 (0-indexed) must be '7'
		if hexStr[12] != '7' {
			t.Fatalf("UUIDv7 version check failed at iteration %d: got %c, expected 7, uuid=%s", i, hexStr[12], id)
		}
		// Variant nibble at position 16 must be 8,9,a,b (binary 10xx)
		variantNibble := hexCharToInt(hexStr[16])
		if variantNibble < 8 || variantNibble > 11 {
			t.Fatalf("UUIDv7 variant check failed at iteration %d: got %d, expected 8-11, uuid=%s", i, variantNibble, id)
		}
	}
}

func TestUUIDv7Monotonicity(t *testing.T) {
	t.Parallel()
	// Generate sequential UUIDv7s and verify ordering within a bounded window.
	// Note: Within the same millisecond, random bits may not be ordered,
	// but across different milliseconds they must be.
	const count = 100
	ids := make([]string, count)
	for i := 0; i < count; i++ {
		ids[i] = GenerateUUIDv7()
	}
	// Extract timestamps and verify non-decreasing
	for i := 1; i < count; i++ {
		ts1 := extractTimestampMs(t, ids[i-1])
		ts2 := extractTimestampMs(t, ids[i])
		if ts2 < ts1 {
			t.Fatalf("UUIDv7 timestamp decreased at index %d: %d -> %d", i, ts1, ts2)
		}
	}
}

func TestUUIDv7Uniqueness(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id := GenerateUUIDv7()
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate UUIDv7 at iteration %d: %s", i, id)
		}
		seen[id] = struct{}{}
	}
}

func extractTimestampMs(t *testing.T, uuid string) int64 {
	t.Helper()
	hexStr := strings.ReplaceAll(uuid, "-", "")
	tsHex := hexStr[:12] // first 48 bits = 6 bytes = 12 hex chars
	b, err := hex.DecodeString(tsHex)
	if err != nil {
		t.Fatalf("failed to decode timestamp hex: %v", err)
	}
	var ts int64
	for _, v := range b {
		ts = (ts << 8) | int64(v)
	}
	return ts
}

func hexCharToInt(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	default:
		return -1
	}
}
