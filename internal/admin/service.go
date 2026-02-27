package admin

import (
	"fmt"
	"sort"
	"sync"
)

type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type UserSession struct {
	SessionID string `json:"session_id"`
	UserID    string `json:"user_id"`
	State     string `json:"state"`
}

type CostBudget struct {
	WorkspaceID string  `json:"workspace_id"`
	MonthlyCap  float64 `json:"monthly_cap"`
	CurrentCost float64 `json:"current_cost"`
	Currency    string  `json:"currency"`
}

type AlertRule struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Metric     string  `json:"metric"`
	Threshold  float64 `json:"threshold"`
	Comparator string  `json:"comparator"`
	Enabled    bool    `json:"enabled"`
}

type AlertChannel struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Target  string `json:"target"`
	Enabled bool   `json:"enabled"`
}

type Service struct {
	mu            sync.RWMutex
	nextID        int
	users         map[string]User
	userSessions  map[string][]UserSession
	budget        CostBudget
	alertRules    map[string]AlertRule
	alertChannels map[string]AlertChannel
}

func NewService() *Service {
	return &Service{
		nextID:        1,
		users:         map[string]User{},
		userSessions:  map[string][]UserSession{},
		budget:        CostBudget{WorkspaceID: "default", MonthlyCap: 1000, CurrentCost: 200, Currency: "USD"},
		alertRules:    map[string]AlertRule{},
		alertChannels: map[string]AlertChannel{},
	}
}

func (s *Service) RecalculateTrustScores() map[string]any {
	return map[string]any{
		"status":                  "completed",
		"recalculated_workspaces": 1,
	}
}

func (s *Service) BulkRetireLessons() map[string]any {
	return map[string]any{
		"status":          "completed",
		"retired_lessons": 0,
	}
}

func (s *Service) ListUsers() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, user := range s.users {
		out = append(out, user)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) UpsertUser(user User) User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if user.ID == "" {
		user.ID = fmt.Sprintf("admin_user_%06d", s.nextID)
		s.nextID++
	}
	if user.Role == "" {
		user.Role = "operator"
	}
	if user.Status == "" {
		user.Status = "active"
	}
	s.users[user.ID] = user
	if _, ok := s.userSessions[user.ID]; !ok {
		s.userSessions[user.ID] = []UserSession{
			{
				SessionID: fmt.Sprintf("session_%06d", s.nextID),
				UserID:    user.ID,
				State:     "active",
			},
		}
		s.nextID++
	}
	return user
}

func (s *Service) GetUser(id string) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	return user, ok
}

func (s *Service) ListUserSessions(userID string) []UserSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]UserSession, len(s.userSessions[userID]))
	copy(out, s.userSessions[userID])
	return out
}

func (s *Service) Dashboard() map[string]any {
	return map[string]any{
		"active_workflows": 3,
		"queue_backlog":    12,
		"error_rate_pct":   0.2,
	}
}

func (s *Service) Workflows() []map[string]any {
	return []map[string]any{
		{"workflow": "interactive_turn_v1", "state": "healthy"},
		{"workflow": "provisioning_v9", "state": "healthy"},
	}
}

func (s *Service) Queues() []map[string]any {
	return []map[string]any{
		{"queue": "interactive_turns", "depth": 2},
		{"queue": "workflow_tasks", "depth": 1},
	}
}

func (s *Service) CostSummary() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return map[string]any{
		"monthly_cap":  s.budget.MonthlyCap,
		"current_cost": s.budget.CurrentCost,
		"currency":     s.budget.Currency,
	}
}

func (s *Service) CostAnomalies() []map[string]any {
	return []map[string]any{
		{
			"id":       "cost_anomaly_000001",
			"severity": "low",
			"reason":   "spend_increase_detected",
		},
	}
}

func (s *Service) GetBudget() CostBudget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.budget
}

func (s *Service) SetBudget(budget CostBudget) CostBudget {
	s.mu.Lock()
	defer s.mu.Unlock()
	if budget.WorkspaceID == "" {
		budget.WorkspaceID = "default"
	}
	if budget.Currency == "" {
		budget.Currency = "USD"
	}
	s.budget = budget
	return budget
}

func (s *Service) ListAlertRules() []AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AlertRule, 0, len(s.alertRules))
	for _, rule := range s.alertRules {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) UpsertAlertRule(rule AlertRule) AlertRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("alert_rule_%06d", s.nextID)
		s.nextID++
	}
	if rule.Comparator == "" {
		rule.Comparator = ">"
	}
	s.alertRules[rule.ID] = rule
	return rule
}

func (s *Service) DeleteAlertRule(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.alertRules[id]; !ok {
		return false
	}
	delete(s.alertRules, id)
	return true
}

func (s *Service) ListAlertChannels() []AlertChannel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AlertChannel, 0, len(s.alertChannels))
	for _, channel := range s.alertChannels {
		out = append(out, channel)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) UpsertAlertChannel(channel AlertChannel) AlertChannel {
	s.mu.Lock()
	defer s.mu.Unlock()
	if channel.ID == "" {
		channel.ID = fmt.Sprintf("alert_channel_%06d", s.nextID)
		s.nextID++
	}
	if channel.Type == "" {
		channel.Type = "email"
	}
	s.alertChannels[channel.ID] = channel
	return channel
}

func (s *Service) KPIReport() map[string]any {
	return map[string]any{
		"availability_pct": 99.95,
		"p95_t1_ms":        2300,
		"error_rate_pct":   0.2,
	}
}
