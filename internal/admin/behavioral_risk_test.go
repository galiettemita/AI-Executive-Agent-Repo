package admin

import (
	"testing"
	"time"
)

func TestBehavioralRiskComputeHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	svc.RecordRiskAction("ws1", "u1", "permission_escalation")
	svc.RecordRiskAction("ws1", "u1", "admin_access_attempt")

	rs, err := svc.ComputeRisk("ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs.RiskScore <= 0 {
		t.Fatal("expected positive risk score")
	}
	if len(rs.RiskFactors) == 0 {
		t.Fatal("expected at least one risk factor")
	}
	if rs.ID == "" {
		t.Fatal("expected generated ID")
	}
}

func TestBehavioralRiskComputeNoActions(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	rs, err := svc.ComputeRisk("ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs.RiskScore != 0 {
		t.Fatalf("expected 0 risk score, got %f", rs.RiskScore)
	}
}

func TestBehavioralRiskComputeMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	_, err := svc.ComputeRisk("", "u1")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.ComputeRisk("ws1", "")
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestBehavioralRiskGetUserRisk(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	svc.RecordRiskAction("ws1", "u1", "role_change")
	_, _ = svc.ComputeRisk("ws1", "u1")

	rs, err := svc.GetUserRisk("ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rs.RiskScore <= 0 {
		t.Fatal("expected positive risk score")
	}
}

func TestBehavioralRiskGetUserRiskNotFound(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	_, err := svc.GetUserRisk("ws1", "u1")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestBehavioralRiskGetHighRiskUsers(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()

	// u1: high risk
	svc.RecordRiskAction("ws1", "u1", "permission_escalation")
	svc.RecordRiskAction("ws1", "u1", "permission_escalation")
	_, _ = svc.ComputeRisk("ws1", "u1")

	// u2: no risk
	_, _ = svc.ComputeRisk("ws1", "u2")

	high := svc.GetHighRiskUsers("ws1", 20)
	if len(high) != 1 {
		t.Fatalf("expected 1 high risk user, got %d", len(high))
	}
	if high[0].UserID != "u1" {
		t.Fatalf("expected u1, got %s", high[0].UserID)
	}
}

func TestBehavioralRiskUnusualHours(t *testing.T) {
	t.Parallel()
	svc := NewBehavioralRiskService()
	// Set time to 3 AM
	svc.now = func() time.Time {
		return time.Date(2025, 6, 1, 3, 0, 0, 0, time.UTC)
	}
	svc.RecordRiskAction("ws1", "u1", "normal_action")
	svc.RecordRiskAction("ws1", "u1", "normal_action")

	rs, _ := svc.ComputeRisk("ws1", "u1")
	found := false
	for _, f := range rs.RiskFactors {
		if len(f) > 14 && f[:14] == "unusual_hours:" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected unusual_hours factor")
	}
}
