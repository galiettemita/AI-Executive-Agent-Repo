package identity

import (
	"slices"
	"testing"
)

func TestNormalizeDomainAutonomy(t *testing.T) {
	t.Parallel()

	normalized := NormalizeDomainAutonomy(map[string]string{
		"calendar": "A2",
		"email":    "A1",
		"web":      "A3",
	})
	if len(normalized) != 11 {
		t.Fatalf("expected all 11 domains, got %d", len(normalized))
	}
	if normalized["tasks"] != "A0" {
		t.Fatalf("expected missing domain to default A0, got %s", normalized["tasks"])
	}
}

func TestAllowedConnectorLifecycleAndDelegationCap(t *testing.T) {
	t.Parallel()

	keys := UpdateAllowedConnectorKeys(nil, "provisioned", "google_calendar")
	if !slices.Contains(keys, "google_calendar") {
		t.Fatalf("expected connector to be added")
	}
	keys = UpdateAllowedConnectorKeys(keys, "admin_block", "google_calendar")
	if slices.Contains(keys, "google_calendar") {
		t.Fatalf("expected connector to be removed on admin block")
	}

	if cap := EffectiveWorkspaceAutonomyCap("delegation", "A3"); cap != "A2" {
		t.Fatalf("expected delegation autonomy cap max A2, got %s", cap)
	}
	if cap := EffectiveWorkspaceAutonomyCap("professional", "A1"); cap != "A4" {
		t.Fatalf("expected non-delegation workspace cap A4, got %s", cap)
	}
}
