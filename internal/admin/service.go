package admin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
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

type DashboardConfig struct {
	WorkspaceID    string   `json:"workspace_id"`
	RefreshSeconds int      `json:"refresh_seconds"`
	Widgets        []string `json:"widgets"`
}

type SavedView struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	Name        string            `json:"name"`
	Filters     map[string]string `json:"filters"`
	CreatedAt   string            `json:"created_at"`
}

type AlertEvent struct {
	ID        string  `json:"id"`
	RuleID    string  `json:"rule_id"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	FiredAt   string  `json:"fired_at"`
}

type MCPServerHealth struct {
	ServerID          string  `json:"server_id"`
	Status            string  `json:"status"`
	P95LatencyMS      int     `json:"p95_latency_ms"`
	ErrorRate         float64 `json:"error_rate"`
	MonthlyCalls      int     `json:"monthly_calls"`
	MonthlyCostUSD    float64 `json:"monthly_cost_usd"`
	MonthlyCallCap    int     `json:"monthly_call_cap"`
	MonthlyCostCapUSD float64 `json:"monthly_cost_cap_usd"`
}

type Service struct {
	mu               sync.RWMutex
	nextID           int
	users            map[string]User
	userSessions     map[string][]UserSession
	budget           CostBudget
	alertRules       map[string]AlertRule
	alertChannels    map[string]AlertChannel
	dashboardConfigs map[string]DashboardConfig
	savedViews       map[string][]SavedView
	alertEvents      []AlertEvent
	mcpServerHealth  map[string]MCPServerHealth
	now              func() time.Time
}

func NewService() *Service {
	return &Service{
		nextID:        1,
		users:         map[string]User{},
		userSessions:  map[string][]UserSession{},
		budget:        CostBudget{WorkspaceID: "default", MonthlyCap: 1000, CurrentCost: 200, Currency: "USD"},
		alertRules:    map[string]AlertRule{},
		alertChannels: map[string]AlertChannel{},
		dashboardConfigs: map[string]DashboardConfig{
			"default": {
				WorkspaceID:    "default",
				RefreshSeconds: 60,
				Widgets:        []string{"active_workflows", "queue_backlog", "error_rate_pct", "cost_burn"},
			},
		},
		savedViews:  map[string][]SavedView{},
		alertEvents: []AlertEvent{},
		mcpServerHealth: map[string]MCPServerHealth{
			"stripe_mcp": {
				ServerID:          "stripe_mcp",
				Status:            "healthy",
				P95LatencyMS:      130,
				ErrorRate:         0.01,
				MonthlyCalls:      14,
				MonthlyCostUSD:    0.14,
				MonthlyCallCap:    1000,
				MonthlyCostCapUSD: 200,
			},
			"plaid_mcp": {
				ServerID:          "plaid_mcp",
				Status:            "healthy",
				P95LatencyMS:      145,
				ErrorRate:         0.02,
				MonthlyCalls:      9,
				MonthlyCostUSD:    0.09,
				MonthlyCallCap:    1000,
				MonthlyCostCapUSD: 200,
			},
		},
		now: func() time.Time { return time.Now().UTC() },
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
	metrics := map[string]float64{
		"error_rate_pct":   0.2,
		"queue_backlog":    12,
		"active_workflows": 3,
	}
	fired := s.EvaluateAlertRules(metrics)
	config := s.GetDashboardConfig("default")
	return map[string]any{
		"active_workflows":       3,
		"queue_backlog":          12,
		"error_rate_pct":         0.2,
		"dashboard_config":       config,
		"alerts_fired_last_eval": len(fired),
		"mcp_server_health":      s.ListMCPServerHealth(),
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
	burnPct := 0.0
	if s.budget.MonthlyCap > 0 {
		burnPct = (s.budget.CurrentCost / s.budget.MonthlyCap) * 100
	}
	return map[string]any{
		"monthly_cap":        s.budget.MonthlyCap,
		"current_cost":       s.budget.CurrentCost,
		"currency":           s.budget.Currency,
		"burn_pct":           burnPct,
		"mcp_server_health":  s.ListMCPServerHealth(),
		"mcp_total_cost_usd": s.totalMCPCostUSD(),
	}
}

func (s *Service) UpsertMCPServerHealth(health MCPServerHealth) MCPServerHealth {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(health.ServerID) == "" {
		health.ServerID = fmt.Sprintf("mcp_server_%06d", s.nextID)
		s.nextID++
	}
	if strings.TrimSpace(health.Status) == "" {
		health.Status = "healthy"
	}
	s.mcpServerHealth[health.ServerID] = health
	return health
}

func (s *Service) ListMCPServerHealth() []MCPServerHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MCPServerHealth, 0, len(s.mcpServerHealth))
	for _, health := range s.mcpServerHealth {
		out = append(out, health)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ServerID < out[j].ServerID
	})
	return out
}

func (s *Service) totalMCPCostUSD() float64 {
	total := 0.0
	for _, health := range s.mcpServerHealth {
		total += health.MonthlyCostUSD
	}
	return total
}

func (s *Service) CostAnomalies() []map[string]any {
	severity := "low"
	if s.GetBudget().CurrentCost > s.GetBudget().MonthlyCap*0.9 {
		severity = "high"
	}
	return []map[string]any{
		{
			"id":       "cost_anomaly_000001",
			"severity": severity,
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
	rule.Comparator = strings.TrimSpace(rule.Comparator)
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

func (s *Service) EvaluateAlertRules(metrics map[string]float64) []AlertEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	fired := make([]AlertEvent, 0)
	for _, rule := range s.alertRules {
		if !rule.Enabled {
			continue
		}
		value, ok := metrics[rule.Metric]
		if !ok {
			continue
		}
		if !compareMetric(value, rule.Comparator, rule.Threshold) {
			continue
		}
		event := AlertEvent{
			ID:        fmt.Sprintf("alert_event_%06d", s.nextID),
			RuleID:    rule.ID,
			Metric:    rule.Metric,
			Value:     value,
			Threshold: rule.Threshold,
			FiredAt:   s.now().Format(time.RFC3339),
		}
		s.nextID++
		s.alertEvents = append(s.alertEvents, event)
		fired = append(fired, event)
	}
	return fired
}

func (s *Service) ListAlertEvents() []AlertEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AlertEvent, len(s.alertEvents))
	copy(out, s.alertEvents)
	return out
}

func (s *Service) GetDashboardConfig(workspaceID string) DashboardConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	if cfg, ok := s.dashboardConfigs[workspaceID]; ok {
		return cfg
	}
	return s.dashboardConfigs["default"]
}

func (s *Service) UpsertDashboardConfig(workspaceID string, config DashboardConfig) DashboardConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	if config.RefreshSeconds <= 0 {
		config.RefreshSeconds = 60
	}
	if len(config.Widgets) == 0 {
		config.Widgets = []string{"active_workflows", "queue_backlog", "error_rate_pct", "cost_burn"}
	}
	config.WorkspaceID = workspaceID
	s.dashboardConfigs[workspaceID] = config
	return config
}

func (s *Service) ListSavedViews(workspaceID string) []SavedView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	views := s.savedViews[workspaceID]
	out := make([]SavedView, len(views))
	copy(out, views)
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) UpsertSavedView(workspaceID string, view SavedView) SavedView {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	if view.ID == "" {
		view.ID = fmt.Sprintf("saved_view_%06d", s.nextID)
		s.nextID++
	}
	if view.Name == "" {
		view.Name = "default_view"
	}
	if view.Filters == nil {
		view.Filters = map[string]string{}
	}
	view.WorkspaceID = workspaceID
	if view.CreatedAt == "" {
		view.CreatedAt = s.now().Format(time.RFC3339)
	}

	views := s.savedViews[workspaceID]
	updated := false
	for i := range views {
		if views[i].ID == view.ID {
			views[i] = view
			updated = true
			break
		}
	}
	if !updated {
		views = append(views, view)
	}
	s.savedViews[workspaceID] = views
	return view
}

func (s *Service) DeleteSavedView(workspaceID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	views := s.savedViews[workspaceID]
	for i := range views {
		if views[i].ID != id {
			continue
		}
		s.savedViews[workspaceID] = append(views[:i], views[i+1:]...)
		return true
	}
	return false
}

func (s *Service) KPIReport() map[string]any {
	return map[string]any{
		"availability_pct":          99.95,
		"p95_t1_ms":                 2300,
		"error_rate_pct":            0.2,
		"alerts_total":              len(s.ListAlertEvents()),
		"active_alert_rules":        len(s.ListAlertRules()),
		"configured_alert_channels": len(s.ListAlertChannels()),
	}
}

func compareMetric(value float64, comparator string, threshold float64) bool {
	switch comparator {
	case ">":
		return value > threshold
	case ">=":
		return value >= threshold
	case "<":
		return value < threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	default:
		return value > threshold
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}
