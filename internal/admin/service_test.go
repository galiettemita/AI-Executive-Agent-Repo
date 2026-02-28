package admin

import "testing"

func TestAdminLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	user := s.UpsertUser(User{
		Email: "operator@brev.io",
		Role:  "operator",
	})
	if user.ID == "" {
		t.Fatalf("expected generated user id")
	}

	users := s.ListUsers()
	if len(users) != 1 {
		t.Fatalf("expected one user, got %d", len(users))
	}

	sessions := s.ListUserSessions(user.ID)
	if len(sessions) == 0 {
		t.Fatalf("expected user sessions")
	}

	budget := s.SetBudget(CostBudget{
		WorkspaceID: "default",
		MonthlyCap:  1500,
		CurrentCost: 300,
		Currency:    "USD",
	})
	if budget.MonthlyCap != 1500 {
		t.Fatalf("unexpected budget update: %#v", budget)
	}

	rule := s.UpsertAlertRule(AlertRule{
		Name:       "error_rate_spike",
		Metric:     "error_rate_pct",
		Threshold:  0.1,
		Comparator: ">",
		Enabled:    true,
	})
	if rule.ID == "" {
		t.Fatalf("expected alert rule id")
	}

	fired := s.EvaluateAlertRules(map[string]float64{"error_rate_pct": 0.2})
	if len(fired) != 1 {
		t.Fatalf("expected one fired alert event, got %d", len(fired))
	}

	if !s.DeleteAlertRule(rule.ID) {
		t.Fatalf("expected alert rule delete success")
	}

	channel := s.UpsertAlertChannel(AlertChannel{
		Type:    "email",
		Target:  "ops@brev.io",
		Enabled: true,
	})
	if channel.ID == "" {
		t.Fatalf("expected alert channel id")
	}

	kpi := s.KPIReport()
	if kpi["availability_pct"] == nil {
		t.Fatalf("expected kpi payload")
	}

	trust := s.RecalculateTrustScores()
	if trust["status"] != "completed" {
		t.Fatalf("unexpected trust recalc result: %#v", trust)
	}
}

func TestAdminDashboardConfigAndSavedViews(t *testing.T) {
	t.Parallel()

	s := NewService()
	cfg := s.UpsertDashboardConfig("ws_admin", DashboardConfig{
		RefreshSeconds: 30,
		Widgets:        []string{"active_workflows", "queue_backlog"},
	})
	if cfg.WorkspaceID != "ws_admin" {
		t.Fatalf("unexpected dashboard config workspace: %#v", cfg)
	}
	loaded := s.GetDashboardConfig("ws_admin")
	if loaded.RefreshSeconds != 30 {
		t.Fatalf("unexpected dashboard config load: %#v", loaded)
	}

	view := s.UpsertSavedView("ws_admin", SavedView{
		Name: "cost_focus",
		Filters: map[string]string{
			"section": "costs",
		},
	})
	if view.ID == "" {
		t.Fatalf("expected saved view id")
	}
	views := s.ListSavedViews("ws_admin")
	if len(views) != 1 {
		t.Fatalf("expected one saved view, got %d", len(views))
	}
	if !s.DeleteSavedView("ws_admin", view.ID) {
		t.Fatalf("expected saved view delete success")
	}
	if len(s.ListSavedViews("ws_admin")) != 0 {
		t.Fatalf("expected saved views to be empty after delete")
	}
}
