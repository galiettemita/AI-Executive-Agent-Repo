package self_modification

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Policy struct {
	WorkspaceID     string `json:"workspace_id"`
	Enabled         bool   `json:"enabled"`
	RequireApproval bool   `json:"require_approval"`
	MaxAllowedRisk  string `json:"max_allowed_risk"`
}

type ActionRequest struct {
	WorkspaceID      string `json:"workspace_id"`
	ActionKey        string `json:"action_key"`
	RequestedRisk    string `json:"requested_risk"`
	RequiresApproval bool   `json:"requires_approval"`
}

type Decision struct {
	WorkspaceID string    `json:"workspace_id"`
	ActionKey   string    `json:"action_key"`
	Decision    string    `json:"decision"`
	Reason      string    `json:"reason"`
	AuditEvent  string    `json:"audit_event"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

type Service struct {
	mu        sync.RWMutex
	policies  map[string]Policy
	decisions map[string][]Decision
}

func NewService() *Service {
	return &Service{
		policies:  map[string]Policy{},
		decisions: map[string][]Decision{},
	}
}

func (s *Service) GetPolicy(workspaceID string) (Policy, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	policy, ok := s.policies[workspaceID]
	return policy, ok
}

func (s *Service) UpsertPolicy(workspaceID string, policy Policy) Policy {
	stored, _ := s.UpsertPolicyStrict(workspaceID, policy)
	return stored
}

func (s *Service) UpsertPolicyStrict(workspaceID string, policy Policy) (Policy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	policy.WorkspaceID = workspaceID
	risk := normalizeRisk(policy.MaxAllowedRisk)
	if risk == "" {
		return Policy{}, fmt.Errorf("invalid max_allowed_risk")
	}
	policy.MaxAllowedRisk = risk
	s.policies[workspaceID] = policy
	return policy, nil
}

func (s *Service) EvaluateAction(workspaceID string, request ActionRequest) Decision {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	request.WorkspaceID = workspaceID
	if request.ActionKey == "" {
		request.ActionKey = "self_modification"
	}
	request.RequestedRisk = normalizeRisk(request.RequestedRisk)
	if request.RequestedRisk == "" {
		request.RequestedRisk = "elevated"
	}

	policy := s.policies[workspaceID]
	if policy.WorkspaceID == "" {
		policy = defaultPolicy(workspaceID)
	}

	decision := Decision{
		WorkspaceID: workspaceID,
		ActionKey:   request.ActionKey,
		EvaluatedAt: time.Now().UTC(),
	}
	if !policy.Enabled {
		decision.Decision = "deny"
		decision.Reason = "SELF_MODIFICATION_DENIED"
		decision.AuditEvent = "BREVIO.self_modification.denied.v1"
		s.decisions[workspaceID] = append(s.decisions[workspaceID], decision)
		return decision
	}

	if compareRisk(request.RequestedRisk, policy.MaxAllowedRisk) > 0 {
		decision.Decision = "deny"
		decision.Reason = "SELF_MODIFICATION_DENIED"
		decision.AuditEvent = "BREVIO.self_modification.denied.v1"
		s.decisions[workspaceID] = append(s.decisions[workspaceID], decision)
		return decision
	}

	if policy.RequireApproval || request.RequiresApproval {
		decision.Decision = "require_approval"
		decision.Reason = "REQUIRE_APPROVAL"
		decision.AuditEvent = ""
		s.decisions[workspaceID] = append(s.decisions[workspaceID], decision)
		return decision
	}

	decision.Decision = "allow"
	decision.Reason = "ALLOW_WITH_AUDIT"
	decision.AuditEvent = "BREVIO.self_modification.executed.v1"
	s.decisions[workspaceID] = append(s.decisions[workspaceID], decision)
	return decision
}

func (s *Service) Decisions(workspaceID string) []Decision {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := append([]Decision(nil), s.decisions[workspaceID]...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].EvaluatedAt.Before(out[j].EvaluatedAt)
	})
	return out
}

func defaultPolicy(workspaceID string) Policy {
	return Policy{
		WorkspaceID:     workspaceID,
		Enabled:         true,
		RequireApproval: true,
		MaxAllowedRisk:  "elevated",
	}
}

func normalizeRisk(risk string) string {
	switch strings.ToLower(strings.TrimSpace(risk)) {
	case "":
		return "elevated"
	case "low":
		return "low"
	case "elevated":
		return "elevated"
	case "critical":
		return "critical"
	default:
		return ""
	}
}

func compareRisk(left, right string) int {
	return riskRank(left) - riskRank(right)
}

func riskRank(risk string) int {
	switch normalizeRisk(risk) {
	case "low":
		return 1
	case "elevated":
		return 2
	case "critical":
		return 3
	default:
		return 0
	}
}
