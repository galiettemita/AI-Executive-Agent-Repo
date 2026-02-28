package model_tiers

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	tierRank = map[string]int{
		"T0": 0,
		"T1": 1,
		"T2": 2,
		"T3": 3,
	}
	complexityThresholdByTier = map[string]int{
		"T0": 20,
		"T1": 40,
		"T2": 70,
		"T3": 100,
	}
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
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	TargetTier  string    `json:"target_tier"`
	Tier        string    `json:"tier,omitempty"`
	Reason      string    `json:"reason"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type Decision struct {
	WorkspaceID     string `json:"workspace_id"`
	RequestedTier   string `json:"requested_tier"`
	ResolvedTier    string `json:"resolved_tier"`
	ComplexityScore int    `json:"complexity_score"`
	Reason          string `json:"reason"`
}

type Service struct {
	mu           sync.RWMutex
	nextPolicy   int
	nextOverride int
	policies     map[string]Policy
	overrides    []Override
	usage        map[string]map[string]int
	now          func() time.Time
}

func NewService() *Service {
	return &Service{
		nextPolicy:   1,
		nextOverride: 1,
		policies:     map[string]Policy{},
		overrides:    []Override{},
		usage:        map[string]map[string]int{},
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) UpsertPolicy(policy Policy) Policy {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("model_tier_policy_%06d", s.nextPolicy)
		s.nextPolicy++
	}
	policy.WorkspaceID = normalizeWorkspaceID(policy.WorkspaceID)
	policy.Tier = normalizeTier(policy.Tier)
	if policy.MaxOutputTokens == 0 {
		policy.MaxOutputTokens = defaultTokensForTier(policy.Tier)
	}
	if policy.MaxSteps == 0 {
		policy.MaxSteps = defaultStepsForTier(policy.Tier)
	}
	if !policy.Enabled {
		policy.Enabled = true
	}
	s.policies[policy.ID] = policy

	if tierRank[policy.Tier] >= tierRank["T3"] {
		s.appendOverrideLocked(policy.WorkspaceID, policy.Tier, "MODEL_TIER_EXCEEDED")
	}

	return policy
}

func (s *Service) EnforceTier(workspaceID, requestedTier string, complexityScore int) Decision {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	requestedTier = normalizeTier(requestedTier)
	resolvedTier := requestedTier
	maxTier := s.maxAllowedTierLocked(workspaceID)
	reason := "ALLOW"

	if tierRank[resolvedTier] > tierRank[maxTier] {
		resolvedTier = maxTier
		reason = "MODEL_TIER_EXCEEDED"
		s.appendOverrideLocked(workspaceID, resolvedTier, reason)
	}

	threshold := complexityThreshold(resolvedTier)
	if complexityScore > threshold {
		escalated := nextTier(resolvedTier)
		if tierRank[escalated] <= tierRank[maxTier] {
			resolvedTier = escalated
			reason = "MODEL_TIER_COMPLEXITY_ESCALATED"
		} else {
			reason = "MODEL_TIER_EXCEEDED"
			s.appendOverrideLocked(workspaceID, maxTier, reason)
			resolvedTier = maxTier
		}
	}

	if _, ok := s.usage[workspaceID]; !ok {
		s.usage[workspaceID] = map[string]int{}
	}
	s.usage[workspaceID][resolvedTier]++

	return Decision{
		WorkspaceID:     workspaceID,
		RequestedTier:   requestedTier,
		ResolvedTier:    resolvedTier,
		ComplexityScore: complexityScore,
		Reason:          reason,
	}
}

func (s *Service) ListPolicies(workspaceID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Policy, 0, len(s.policies))
	for _, policy := range s.policies {
		if policy.WorkspaceID != workspaceID {
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

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Override, 0, len(s.overrides))
	for _, override := range s.overrides {
		if override.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, override)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) TierUsage(workspaceID string) map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	usage := s.usage[workspaceID]
	out := make(map[string]int, len(usage))
	for tier, count := range usage {
		out[tier] = count
	}
	return out
}

func (s *Service) appendOverrideLocked(workspaceID, targetTier, reason string) {
	now := s.now()
	override := Override{
		ID:          fmt.Sprintf("model_tier_override_%06d", s.nextOverride),
		WorkspaceID: workspaceID,
		TargetTier:  targetTier,
		Tier:        targetTier,
		Reason:      reason,
		Status:      "active",
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	s.nextOverride++
	s.overrides = append(s.overrides, override)
}

func (s *Service) maxAllowedTierLocked(workspaceID string) string {
	maxTier := "T1"
	for _, policy := range s.policies {
		if policy.WorkspaceID != workspaceID || !policy.Enabled {
			continue
		}
		tier := normalizeTier(policy.Tier)
		if tierRank[tier] > tierRank[maxTier] {
			maxTier = tier
		}
	}
	return maxTier
}

func complexityThreshold(tier string) int {
	if threshold, ok := complexityThresholdByTier[tier]; ok {
		return threshold
	}
	return complexityThresholdByTier["T1"]
}

func nextTier(tier string) string {
	tier = normalizeTier(tier)
	switch tier {
	case "T0":
		return "T1"
	case "T1":
		return "T2"
	case "T2":
		return "T3"
	default:
		return "T3"
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeTier(tier string) string {
	normalized := strings.ToUpper(strings.TrimSpace(tier))
	if _, ok := tierRank[normalized]; !ok {
		return "T1"
	}
	return normalized
}

func defaultTokensForTier(tier string) int {
	switch normalizeTier(tier) {
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
	switch normalizeTier(tier) {
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
