package council

import (
	"fmt"
	"sync"
)

// CouncilConvenePolicy defines the criteria for when a council should be convened.
type CouncilConvenePolicy struct {
	MinComplexity          float64 `json:"min_complexity"`           // 0.0-1.0
	MinStakeholders        int     `json:"min_stakeholders"`
	RequiresDomainExpertise bool   `json:"requires_domain_expertise"`
}

// ConveneDecision is the output of the meta-reasoning check.
type ConveneDecision struct {
	ShouldConvene bool   `json:"should_convene"`
	Reason        string `json:"reason"`
}

// CouncilMetaReasoningService determines whether a council should be convened
// based on request complexity, stakeholder count, and domain requirements.
type CouncilMetaReasoningService struct {
	mu       sync.Mutex
	policies map[string]CouncilConvenePolicy // workspaceID -> policy
}

// NewCouncilMetaReasoningService creates a new meta-reasoning service.
func NewCouncilMetaReasoningService() *CouncilMetaReasoningService {
	return &CouncilMetaReasoningService{
		policies: map[string]CouncilConvenePolicy{},
	}
}

// defaultPolicy returns the default convene policy.
func defaultPolicy() CouncilConvenePolicy {
	return CouncilConvenePolicy{
		MinComplexity:          0.6,
		MinStakeholders:        2,
		RequiresDomainExpertise: false,
	}
}

// SetConvenePolicy sets the convene policy for a workspace.
func (s *CouncilMetaReasoningService) SetConvenePolicy(workspaceID string, policy CouncilConvenePolicy) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if policy.MinComplexity < 0 || policy.MinComplexity > 1 {
		return fmt.Errorf("min_complexity must be between 0 and 1")
	}
	if policy.MinStakeholders < 0 {
		return fmt.Errorf("min_stakeholders must be non-negative")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies[workspaceID] = policy
	return nil
}

// GetConvenePolicy returns the convene policy for a workspace.
func (s *CouncilMetaReasoningService) GetConvenePolicy(workspaceID string) CouncilConvenePolicy {
	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.policies[workspaceID]; ok {
		return p
	}
	return defaultPolicy()
}

// ShouldConveneCouncil evaluates whether a council should be convened for a given
// request based on complexity, domain, and stakeholder count.
func (s *CouncilMetaReasoningService) ShouldConveneCouncil(request string, complexity float64, domain string, stakeholderCount int) *ConveneDecision {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Use a default policy if none is set (we don't have workspaceID here,
	// so we check against default thresholds).
	policy := defaultPolicy()

	decision := &ConveneDecision{}

	// Check complexity threshold.
	if complexity >= policy.MinComplexity {
		decision.ShouldConvene = true
		decision.Reason = fmt.Sprintf("complexity %.2f meets threshold %.2f", complexity, policy.MinComplexity)
	}

	// Check stakeholder threshold.
	if stakeholderCount >= policy.MinStakeholders {
		if decision.ShouldConvene {
			decision.Reason += fmt.Sprintf("; stakeholders %d meet threshold %d", stakeholderCount, policy.MinStakeholders)
		} else {
			decision.ShouldConvene = true
			decision.Reason = fmt.Sprintf("stakeholders %d meet threshold %d", stakeholderCount, policy.MinStakeholders)
		}
	}

	// Domain expertise check.
	if policy.RequiresDomainExpertise && domain != "" {
		if !decision.ShouldConvene {
			decision.ShouldConvene = true
			decision.Reason = fmt.Sprintf("domain expertise required for %q", domain)
		} else {
			decision.Reason += fmt.Sprintf("; domain expertise required for %q", domain)
		}
	}

	if !decision.ShouldConvene {
		decision.Reason = "criteria not met"
	}

	return decision
}

// ShouldConveneCouncilForWorkspace evaluates using the workspace-specific policy.
func (s *CouncilMetaReasoningService) ShouldConveneCouncilForWorkspace(workspaceID, request string, complexity float64, domain string, stakeholderCount int) *ConveneDecision {
	s.mu.Lock()
	policy, ok := s.policies[workspaceID]
	if !ok {
		policy = defaultPolicy()
	}
	s.mu.Unlock()

	decision := &ConveneDecision{}

	meetsComplexity := complexity >= policy.MinComplexity
	meetsStakeholders := stakeholderCount >= policy.MinStakeholders

	if meetsComplexity {
		decision.ShouldConvene = true
		decision.Reason = fmt.Sprintf("complexity %.2f meets threshold %.2f", complexity, policy.MinComplexity)
	}

	if meetsStakeholders {
		if decision.ShouldConvene {
			decision.Reason += fmt.Sprintf("; stakeholders %d meet threshold %d", stakeholderCount, policy.MinStakeholders)
		} else {
			decision.ShouldConvene = true
			decision.Reason = fmt.Sprintf("stakeholders %d meet threshold %d", stakeholderCount, policy.MinStakeholders)
		}
	}

	if policy.RequiresDomainExpertise && domain != "" {
		if !decision.ShouldConvene {
			decision.ShouldConvene = true
			decision.Reason = fmt.Sprintf("domain expertise required for %q", domain)
		} else {
			decision.Reason += fmt.Sprintf("; domain expertise required for %q", domain)
		}
	}

	if !decision.ShouldConvene {
		decision.Reason = "criteria not met"
	}

	return decision
}
