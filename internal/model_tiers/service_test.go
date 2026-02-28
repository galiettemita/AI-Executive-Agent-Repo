package model_tiers

import "testing"

func TestModelTierLifecycle(t *testing.T) {
	t.Parallel()

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
	if overrides[0].TargetTier != "T3" {
		t.Fatalf("expected target_tier T3, got %#v", overrides[0])
	}
	if overrides[0].ExpiresAt.IsZero() {
		t.Fatalf("expected override expiration timestamp")
	}
}

func TestEnforceTierDowngradesAndAudits(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertPolicy(Policy{WorkspaceID: "ws_2", Tier: "T2", Enabled: true})

	decision := s.EnforceTier("ws_2", "T3", 85)
	if decision.ResolvedTier != "T2" {
		t.Fatalf("expected downgrade to T2, got %#v", decision)
	}
	if decision.Reason != "MODEL_TIER_EXCEEDED" {
		t.Fatalf("unexpected decision reason: %#v", decision)
	}

	overrides := s.ListOverrides("ws_2")
	if len(overrides) == 0 {
		t.Fatal("expected override audit record")
	}
}

func TestEnforceTierEscalatesWithinAllowedComplexityRange(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertPolicy(Policy{WorkspaceID: "ws_3", Tier: "T3", Enabled: true})

	decision := s.EnforceTier("ws_3", "T1", 95)
	if decision.ResolvedTier != "T2" {
		t.Fatalf("expected escalation from T1 to T2 for high complexity, got %#v", decision)
	}
	if decision.Reason != "MODEL_TIER_COMPLEXITY_ESCALATED" {
		t.Fatalf("unexpected escalation reason: %#v", decision)
	}

	usage := s.TierUsage("ws_3")
	if usage["T2"] != 1 {
		t.Fatalf("expected tier usage increment for T2, got %#v", usage)
	}
}
