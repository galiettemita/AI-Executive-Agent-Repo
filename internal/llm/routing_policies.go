package llm

import "strings"

type RoutingPolicy struct {
	WorkspaceID       string
	Tier              string
	PrimaryModelID    string
	FallbackModelID   string
	MaxTokensPerTurn  int
	MaxMonthlyLLMCost float64
	IsActive          bool
}

func ResolveTierSelectionWithPolicies(tier string, defaults TierModelSelection, policies []RoutingPolicy) TierModelSelection {
	var wildcard *RoutingPolicy
	for i := range policies {
		policy := policies[i]
		if !policy.IsActive {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(policy.Tier), strings.TrimSpace(tier)) {
			return applyPolicy(defaults, policy)
		}
		if strings.TrimSpace(policy.Tier) == "*" {
			p := policy
			wildcard = &p
		}
	}
	if wildcard != nil {
		return applyPolicy(defaults, *wildcard)
	}
	return defaults
}

func applyPolicy(defaults TierModelSelection, policy RoutingPolicy) TierModelSelection {
	out := defaults
	if strings.TrimSpace(policy.PrimaryModelID) != "" {
		out.PrimaryModel = policy.PrimaryModelID
	}
	if strings.TrimSpace(policy.FallbackModelID) != "" {
		out.FallbackModel = policy.FallbackModelID
	}
	if policy.MaxTokensPerTurn > 0 {
		out.MaxOutputTokens = policy.MaxTokensPerTurn
	}
	return out
}
