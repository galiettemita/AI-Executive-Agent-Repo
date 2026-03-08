package admin

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BehavioralRiskScore represents a computed behavioral risk assessment.
type BehavioralRiskScore struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	RiskScore   float64   `json:"risk_score"`
	RiskFactors []string  `json:"risk_factors"`
	ComputedAt  time.Time `json:"computed_at"`
}

// riskAction stores a user action for risk computation.
type riskAction struct {
	WorkspaceID string
	UserID      string
	Action      string
	Timestamp   time.Time
}

// BehavioralRiskService computes and manages behavioral risk scores.
type BehavioralRiskService struct {
	mu      sync.Mutex
	actions []riskAction
	scores  map[string]*BehavioralRiskScore // key: workspaceID:userID
	now     func() time.Time
}

// NewBehavioralRiskService creates a new BehavioralRiskService.
func NewBehavioralRiskService() *BehavioralRiskService {
	return &BehavioralRiskService{
		actions: []riskAction{},
		scores:  map[string]*BehavioralRiskScore{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func riskKey(workspaceID, userID string) string {
	return workspaceID + ":" + userID
}

// RecordRiskAction records a user action for risk analysis.
func (s *BehavioralRiskService) RecordRiskAction(workspaceID, userID, action string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.actions = append(s.actions, riskAction{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Action:      action,
		Timestamp:   s.now(),
	})
}

// ComputeRisk computes a risk score for a user based on their actions.
func (s *BehavioralRiskService) ComputeRisk(workspaceID, userID string) (BehavioralRiskScore, error) {
	if workspaceID == "" {
		return BehavioralRiskScore{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return BehavioralRiskScore{}, fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var userActions []riskAction
	for _, a := range s.actions {
		if a.WorkspaceID == workspaceID && a.UserID == userID {
			userActions = append(userActions, a)
		}
	}

	score := 0.0
	var factors []string

	// Factor 1: unusual hours (outside 06:00-22:00)
	unusualCount := 0
	for _, a := range userActions {
		h := a.Timestamp.Hour()
		if h < 6 || h > 22 {
			unusualCount++
		}
	}
	if unusualCount > 0 {
		s1 := math.Min(float64(unusualCount)*10, 30)
		score += s1
		factors = append(factors, fmt.Sprintf("unusual_hours:%d", unusualCount))
	}

	// Factor 2: high volume (>50 actions)
	if len(userActions) > 50 {
		s2 := math.Min(float64(len(userActions)-50)*0.5, 30)
		score += s2
		factors = append(factors, fmt.Sprintf("high_volume:%d", len(userActions)))
	}

	// Factor 3: privilege escalation attempts
	escalations := 0
	for _, a := range userActions {
		switch a.Action {
		case "permission_escalation", "role_change", "admin_access_attempt":
			escalations++
		}
	}
	if escalations > 0 {
		s3 := math.Min(float64(escalations)*20, 40)
		score += s3
		factors = append(factors, fmt.Sprintf("escalation_attempts:%d", escalations))
	}

	score = math.Min(score, 100)

	rs := BehavioralRiskScore{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		UserID:      userID,
		RiskScore:   score,
		RiskFactors: factors,
		ComputedAt:  s.now(),
	}
	s.scores[riskKey(workspaceID, userID)] = &rs
	return rs, nil
}

// GetUserRisk returns the latest computed risk score for a user.
func (s *BehavioralRiskService) GetUserRisk(workspaceID, userID string) (*BehavioralRiskScore, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rs, ok := s.scores[riskKey(workspaceID, userID)]
	if !ok {
		return nil, fmt.Errorf("no risk score found for %s:%s", workspaceID, userID)
	}
	cp := *rs
	return &cp, nil
}

// GetHighRiskUsers returns users with risk scores above a threshold.
func (s *BehavioralRiskService) GetHighRiskUsers(workspaceID string, threshold float64) []BehavioralRiskScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []BehavioralRiskScore
	for _, rs := range s.scores {
		if rs.WorkspaceID == workspaceID && rs.RiskScore >= threshold {
			result = append(result, *rs)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].RiskScore > result[j].RiskScore
	})
	return result
}
