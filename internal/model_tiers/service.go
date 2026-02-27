package model_tiers

import (
	"fmt"
	"sort"
	"sync"
)

type Policy struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	Tier            string `json:"tier"`
	MaxOutputTokens int    `json:"max_output_tokens"`
	MaxSteps        int    `json:"max_steps"`
	Enabled         bool   `json:"enabled"`
}

type Override struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Tier        string `json:"tier"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
}

type Service struct {
	mu           sync.RWMutex
	nextPolicy   int
	nextOverride int
	policies     map[string]Policy
	overrides    []Override
}

func NewService() *Service {
	return &Service{
		nextPolicy:   1,
		nextOverride: 1,
		policies:     map[string]Policy{},
		overrides:    []Override{},
	}
}

func (s *Service) UpsertPolicy(policy Policy) Policy {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("model_tier_policy_%06d", s.nextPolicy)
		s.nextPolicy++
	}
	if policy.WorkspaceID == "" {
		policy.WorkspaceID = "default"
	}
	if policy.Tier == "" {
		policy.Tier = "T1"
	}
	if policy.MaxOutputTokens == 0 {
		policy.MaxOutputTokens = defaultTokensForTier(policy.Tier)
	}
	if policy.MaxSteps == 0 {
		policy.MaxSteps = defaultStepsForTier(policy.Tier)
	}
	s.policies[policy.ID] = policy

	if policy.Tier == "T3" || policy.Tier == "T4" {
		override := Override{
			ID:          fmt.Sprintf("model_tier_override_%06d", s.nextOverride),
			WorkspaceID: policy.WorkspaceID,
			Tier:        policy.Tier,
			Reason:      "MODEL_TIER_EXCEEDED",
			Status:      "active",
		}
		s.nextOverride++
		s.overrides = append(s.overrides, override)
	}

	return policy
}

func (s *Service) ListPolicies(workspaceID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Policy, 0, len(s.policies))
	for _, policy := range s.policies {
		if workspaceID != "" && policy.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, policy)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ListOverrides(workspaceID string) []Override {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Override, 0, len(s.overrides))
	for _, override := range s.overrides {
		if workspaceID != "" && override.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, override)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func defaultTokensForTier(tier string) int {
	switch tier {
	case "T0":
		return 256
	case "T1":
		return 512
	case "T2":
		return 1024
	case "T3":
		return 2048
	default:
		return 512
	}
}

func defaultStepsForTier(tier string) int {
	switch tier {
	case "T0":
		return 2
	case "T1":
		return 5
	case "T2":
		return 8
	case "T3":
		return 10
	default:
		return 5
	}
}
