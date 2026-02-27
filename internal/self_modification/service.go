package self_modification

import "sync"

type Policy struct {
	WorkspaceID     string `json:"workspace_id"`
	Enabled         bool   `json:"enabled"`
	RequireApproval bool   `json:"require_approval"`
	MaxAllowedRisk  string `json:"max_allowed_risk"`
}

type Service struct {
	mu       sync.RWMutex
	policies map[string]Policy
}

func NewService() *Service {
	return &Service{
		policies: map[string]Policy{},
	}
}

func (s *Service) GetPolicy(workspaceID string) (Policy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	policy, ok := s.policies[workspaceID]
	return policy, ok
}

func (s *Service) UpsertPolicy(workspaceID string, policy Policy) Policy {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	policy.WorkspaceID = workspaceID
	if policy.MaxAllowedRisk == "" {
		policy.MaxAllowedRisk = "elevated"
	}
	s.policies[workspaceID] = policy
	return policy
}
