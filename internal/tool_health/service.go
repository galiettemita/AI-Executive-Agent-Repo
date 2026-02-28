package tool_health

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type ToolScore struct {
	WorkspaceID  string    `json:"workspace_id"`
	ToolKey      string    `json:"tool_key"`
	Score        float64   `json:"score"`
	FailureCount int       `json:"failure_count"`
	Status       string    `json:"status"`
	LatencyMS    float64   `json:"latency_ms"`
	ErrorRate    float64   `json:"error_rate"`
	EvaluatedAt  time.Time `json:"evaluated_at"`
}

type QuarantineRule struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	ToolKey      string  `json:"tool_key"`
	MinScore     float64 `json:"min_score"`
	MaxFailures  int     `json:"max_failures"`
	MaxErrorRate float64 `json:"max_error_rate"`
	MaxLatencyMS float64 `json:"max_latency_ms"`
	Enabled      bool    `json:"enabled"`
}

type HealthEvent struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	ToolKey     string `json:"tool_key"`
	EventType   string `json:"event_type"`
	FromStatus  string `json:"from_status"`
	ToStatus    string `json:"to_status"`
}

type Service struct {
	mu         sync.RWMutex
	nextRuleID int
	scores     map[string]ToolScore
	rules      map[string]QuarantineRule
	events     []HealthEvent
}

func NewService() *Service {
	return &Service{
		nextRuleID: 1,
		scores:     map[string]ToolScore{},
		rules:      map[string]QuarantineRule{},
		events:     []HealthEvent{},
	}
}

func (s *Service) UpsertScore(score ToolScore) ToolScore {
	s.mu.Lock()
	defer s.mu.Unlock()

	score = normalizeScore(score)
	key := scoreKey(score.WorkspaceID, score.ToolKey)
	previous := s.scores[key]
	targetStatus := deriveStatus(score.Score, score.FailureCount, score.ErrorRate, score.LatencyMS)
	if s.matchesQuarantineRule(score) {
		targetStatus = "quarantined"
	}
	score.Status = targetStatus
	score.EvaluatedAt = time.Now().UTC()
	s.scores[key] = score

	if previous.ToolKey != "" && previous.Status != score.Status {
		s.recordStatusEventLocked(score.WorkspaceID, score.ToolKey, previous.Status, score.Status)
	}
	if previous.ToolKey == "" && score.Status == "quarantined" {
		s.recordStatusEventLocked(score.WorkspaceID, score.ToolKey, "unknown", score.Status)
	}
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

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]ToolScore, 0, len(s.scores))
	for _, score := range s.scores {
		if score.WorkspaceID != workspaceID {
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
	if rule.MaxErrorRate == 0 {
		rule.MaxErrorRate = 0.5
	}
	if rule.MaxLatencyMS == 0 {
		rule.MaxLatencyMS = 2000
	}
	s.rules[rule.ID] = rule
	return rule
}

func (s *Service) ListRules(workspaceID string) []QuarantineRule {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]QuarantineRule, 0, len(s.rules))
	for _, rule := range s.rules {
		if rule.WorkspaceID != workspaceID {
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
	prevStatus := score.Status
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
		if score.ErrorRate > 0.2 {
			score.ErrorRate = 0.2
		}
	}
	score.EvaluatedAt = time.Now().UTC()
	s.scores[key] = score
	if prevStatus != score.Status {
		s.recordStatusEventLocked(workspaceID, toolKey, prevStatus, score.Status)
	}
	return score
}

func (s *Service) ListEvents(workspaceID string) []HealthEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]HealthEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, event)
	}
	return out
}

func (s *Service) matchesQuarantineRule(score ToolScore) bool {
	for _, rule := range s.rules {
		if !rule.Enabled {
			continue
		}
		if rule.WorkspaceID != score.WorkspaceID {
			continue
		}
		if strings.TrimSpace(rule.ToolKey) != "" && rule.ToolKey != score.ToolKey {
			continue
		}
		if score.Score < rule.MinScore || score.FailureCount >= rule.MaxFailures || score.ErrorRate > rule.MaxErrorRate || score.LatencyMS > rule.MaxLatencyMS {
			return true
		}
	}
	return false
}

func (s *Service) recordStatusEventLocked(workspaceID, toolKey, from, to string) {
	eventType := "BREVIO.tool_health.degraded.v1"
	if to == "quarantined" {
		eventType = "BREVIO.tool_health.quarantined.v1"
	}
	if to == "healthy" && from == "quarantined" {
		eventType = "BREVIO.tool_health.recovered.v1"
	}
	if to == "healthy" && from != "quarantined" {
		eventType = "BREVIO.tool_health.score_computed.v1"
	}

	s.events = append(s.events, HealthEvent{
		ID:          fmt.Sprintf("tool_health_event_%06d", len(s.events)+1),
		WorkspaceID: workspaceID,
		ToolKey:     toolKey,
		EventType:   eventType,
		FromStatus:  from,
		ToStatus:    to,
	})
}

func normalizeScore(score ToolScore) ToolScore {
	if score.WorkspaceID == "" {
		score.WorkspaceID = "default"
	}
	if score.Score < 0 {
		score.Score = 0
	}
	if score.Score > 1 {
		score.Score = 1
	}
	if score.ErrorRate < 0 {
		score.ErrorRate = 0
	}
	if score.ErrorRate > 1 {
		score.ErrorRate = 1
	}
	if score.LatencyMS < 0 {
		score.LatencyMS = 0
	}
	return score
}

func deriveStatus(score float64, failures int, errorRate float64, latencyMS float64) string {
	if failures >= 5 || score < 0.5 || errorRate >= 0.5 || latencyMS >= 2000 {
		return "quarantined"
	}
	if failures >= 3 || score < 0.8 || errorRate >= 0.25 || latencyMS >= 1200 {
		return "degraded"
	}
	return "healthy"
}

func scoreKey(workspaceID, toolKey string) string {
	return workspaceID + "|" + toolKey
}
