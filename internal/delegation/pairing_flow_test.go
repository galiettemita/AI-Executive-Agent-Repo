package delegation

import (
	"testing"
	"time"
)

func TestPairingFlowHelpers(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 1, 20, 0, 0, 0, time.UTC)
	code := GeneratePairingCode("ws_1", "owner_1", now)
	if len(code) != 6 {
		t.Fatalf("expected 6-char pairing code, got %s", code)
	}
	if PairingCodeTTL() != 15*time.Minute {
		t.Fatalf("unexpected pairing ttl: %s", PairingCodeTTL())
	}
	if err := ValidatePairingCode(now, now.Add(14*time.Minute)); err != nil {
		t.Fatalf("expected valid pairing code: %v", err)
	}
	if err := ValidatePairingCode(now, now.Add(16*time.Minute)); err == nil {
		t.Fatal("expected expired pairing code")
	}
	if got := DelegationMaxAutonomy("A4"); got != "A2" {
		t.Fatalf("unexpected delegation autonomy cap: %s", got)
	}
}
