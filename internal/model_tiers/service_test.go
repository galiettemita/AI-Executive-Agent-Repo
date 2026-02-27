package model_tiers

import "testing"

func TestModelTierLifecycle(t *testing.T) {
	s := NewService()

	policy := s.UpsertPolicy(Policy{
		WorkspaceID: "ws_1",
		Tier:        "T3",
		Enabled:     true,
	})
	if policy.ID == "" {
		t.Fatalf("expected policy id")
	}

	policies := s.ListPolicies("ws_1")
	if len(policies) != 1 {
		t.Fatalf("expected one policy, got %d", len(policies))
	}

	overrides := s.ListOverrides("ws_1")
	if len(overrides) != 1 {
		t.Fatalf("expected one override for T3 tier, got %d", len(overrides))
	}
	if overrides[0].Reason != "MODEL_TIER_EXCEEDED" {
		t.Fatalf("unexpected override reason: %#v", overrides[0])
	}
}
