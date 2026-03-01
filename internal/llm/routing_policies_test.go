package llm

import "testing"

func TestResolveTierSelectionWithPolicies(t *testing.T) {
	t.Parallel()

	defaults := ResolveTierModel("T2")
	resolved := ResolveTierSelectionWithPolicies("T2", defaults, []RoutingPolicy{
		{Tier: "*", PrimaryModelID: "fallback-all-primary", IsActive: true},
		{Tier: "T2", PrimaryModelID: "custom-primary", FallbackModelID: "custom-fallback", MaxTokensPerTurn: 2048, IsActive: true},
	})
	if resolved.PrimaryModel != "custom-primary" || resolved.FallbackModel != "custom-fallback" || resolved.MaxOutputTokens != 2048 {
		t.Fatalf("unexpected exact-tier policy override result: %+v", resolved)
	}

	wildcardOnly := ResolveTierSelectionWithPolicies("T3", ResolveTierModel("T3"), []RoutingPolicy{
		{Tier: "*", PrimaryModelID: "wild-primary", IsActive: true},
	})
	if wildcardOnly.PrimaryModel != "wild-primary" {
		t.Fatalf("expected wildcard override, got %+v", wildcardOnly)
	}
}
