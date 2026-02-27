package admin

import "testing"

func TestAdminLifecycle(t *testing.T) {
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
		Name:      "error_rate_spike",
		Metric:    "error_rate_pct",
		Threshold: 1.0,
		Enabled:   true,
	})
	if rule.ID == "" {
		t.Fatalf("expected alert rule id")
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
