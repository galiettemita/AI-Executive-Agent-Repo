package tool_health

import (
	"fmt"
	"sort"
	"sync"
)

type ToolScore struct {
	WorkspaceID  string  `json:"workspace_id"`
	ToolKey      string  `json:"tool_key"`
	Score        float64 `json:"score"`
	FailureCount int     `json:"failure_count"`
	Status       string  `json:"status"`
}

type QuarantineRule struct {
	ID          string  `json:"id"`
	WorkspaceID string  `json:"workspace_id"`
	ToolKey     string  `json:"tool_key"`
	MinScore    float64 `json:"min_score"`
	MaxFailures int     `json:"max_failures"`
	Enabled     bool    `json:"enabled"`
}

type Service struct {
	mu         sync.RWMutex
	nextRuleID int
	scores     map[string]ToolScore
	rules      map[string]QuarantineRule
}

func NewService() *Service {
	return &Service{
		nextRuleID: 1,
		scores:     map[string]ToolScore{},
		rules:      map[string]QuarantineRule{},
	}
}

func (s *Service) UpsertScore(score ToolScore) ToolScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if score.WorkspaceID == "" {
		score.WorkspaceID = "default"
	}
	score.Status = deriveStatus(score.Score, score.FailureCount)
	s.scores[scoreKey(score.WorkspaceID, score.ToolKey)] = score
	return score
}

func (s *Service) GetScore(workspaceID, toolKey string) (ToolScore, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	score, ok := s.scores[scoreKey(workspaceID, toolKey)]
	return score, ok
}

func (s *Service) ListScores(workspaceID string) []ToolScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ToolScore, 0, len(s.scores))
	for _, score := range s.scores {
		if workspaceID != "" && score.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, score)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ToolKey < out[j].ToolKey
	})
	return out
}

func (s *Service) UpsertRule(rule QuarantineRule) QuarantineRule {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("quarantine_rule_%06d", s.nextRuleID)
		s.nextRuleID++
	}
	if rule.WorkspaceID == "" {
		rule.WorkspaceID = "default"
	}
	if rule.MaxFailures == 0 {
		rule.MaxFailures = 5
	}
	if rule.MinScore == 0 {
		rule.MinScore = 0.5
	}
	s.rules[rule.ID] = rule
	return rule
}

func (s *Service) ListRules(workspaceID string) []QuarantineRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]QuarantineRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if workspaceID != "" && rule.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ApplyOverride(workspaceID, toolKey, status string) ToolScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	key := scoreKey(workspaceID, toolKey)
	score, ok := s.scores[key]
	if !ok {
		score = ToolScore{
			WorkspaceID: workspaceID,
			ToolKey:     toolKey,
			Score:       1.0,
			Status:      "healthy",
		}
	}
	switch status {
	case "quarantined":
		score.Status = "quarantined"
	case "degraded":
		score.Status = "degraded"
	default:
		score.Status = "healthy"
		score.FailureCount = 0
		if score.Score < 0.8 {
			score.Score = 0.8
		}
	}
	s.scores[key] = score
	return score
}

func deriveStatus(score float64, failures int) string {
	if failures >= 5 || score < 0.5 {
		return "quarantined"
	}
	if failures >= 3 || score < 0.8 {
		return "degraded"
	}
	return "healthy"
}

func scoreKey(workspaceID, toolKey string) string {
	return workspaceID + "|" + toolKey
}
