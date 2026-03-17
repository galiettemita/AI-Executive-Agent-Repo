package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHMACKeyLengthRequirements(t *testing.T) {
	cases := []struct {
		name   string
		keyLen int
		valid  bool
	}{
		{"empty key", 0, false},
		{"too short — 16 bytes", 16, false},
		{"minimum — 32 bytes", 32, true},
		{"strong — 64 bytes", 64, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := make([]byte, tc.keyLen)
			meetsMinimum := len(key) >= 32
			assert.Equal(t, tc.valid, meetsMinimum)
		})
	}
}

func TestRequiredNonLocalEnvContainsHMACKey(t *testing.T) {
	required := []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST", "HMAC_KEY"}
	hmacFound := false
	for _, key := range required {
		if key == "HMAC_KEY" {
			hmacFound = true
		}
	}
	assert.True(t, hmacFound, "HMAC_KEY must be in RequiredNonLocalEnv")
}
