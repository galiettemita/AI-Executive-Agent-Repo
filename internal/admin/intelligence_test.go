package admin

import (
	"testing"
	"time"
)

func TestRecordCost(t *testing.T) {
	ledger := NewLLMCostLedger()
	err := ledger.RecordCost(CostAttribution{
		WorkspaceID:    "ws1",
		UserID:         "u1",
		ToolKey:        "gpt4",
		TokensUsed:     100,
		CostMicroCents: 5000,
		Model:          "gpt-4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordCostMissingWorkspace(t *testing.T) {
	ledger := NewLLMCostLedger()
	err := ledger.RecordCost(CostAttribution{UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestRecordCostMissingUser(t *testing.T) {
	ledger := NewLLMCostLedger()
	err := ledger.RecordCost(CostAttribution{WorkspaceID: "ws1"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestGetWorkspaceCostSummary(t *testing.T) {
	ledger := NewLLMCostLedger()
	now := time.Now().UTC()
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws1", UserID: "u1", ToolKey: "t1", CostMicroCents: 100, Model: "m1", Timestamp: now})
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws1", UserID: "u2", ToolKey: "t1", CostMicroCents: 200, Model: "m2", Timestamp: now.Add(time.Hour)})
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws2", UserID: "u1", ToolKey: "t1", CostMicroCents: 999, Timestamp: now})

	summary, err := ledger.GetWorkspaceCostSummary("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalMicroCents != 300 {
		t.Fatalf("expected total 300, got %d", summary.TotalMicroCents)
	}
}

func TestGetWorkspaceCostSummaryMissingWorkspace(t *testing.T) {
	ledger := NewLLMCostLedger()
	_, err := ledger.GetWorkspaceCostSummary("")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetUserCostBreakdown(t *testing.T) {
	ledger := NewLLMCostLedger()
	now := time.Now().UTC()
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws1", UserID: "u1", ToolKey: "t1", CostMicroCents: 100, Model: "m1", Timestamp: now})
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws1", UserID: "u1", ToolKey: "t2", CostMicroCents: 200, Model: "m1", Timestamp: now})

	bd, err := ledger.GetUserCostBreakdown("ws1", "u1", now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bd.TotalMicroCents != 300 {
		t.Fatalf("expected 300, got %d", bd.TotalMicroCents)
	}
	if bd.InvocationCount != 2 {
		t.Fatalf("expected 2 invocations, got %d", bd.InvocationCount)
	}
}

func TestGetOperatorMargin(t *testing.T) {
	ledger := NewLLMCostLedger()
	ledger.SetRevenue("ws1", 100000)
	_ = ledger.RecordCost(CostAttribution{WorkspaceID: "ws1", UserID: "u1", CostMicroCents: 50000})

	report := ledger.GetMarginReport("ws1")
	if report.RevenueMicroCents != 100000 {
		t.Fatalf("expected revenue 100000, got %d", report.RevenueMicroCents)
	}
	if report.MarginPct <= 0 {
		t.Fatalf("expected positive margin, got %f", report.MarginPct)
	}
}

func TestMRRTracker(t *testing.T) {
	tracker := NewMRRTracker()
	tracker.RecordSubscriptionEvent("ws1", "pro", 10000, "new")

	mrr := tracker.GetMRR()
	if mrr != 10000 {
		t.Fatalf("expected MRR 10000, got %d", mrr)
	}
}

func TestMRRTrackerChurn(t *testing.T) {
	tracker := NewMRRTracker()
	tracker.RecordSubscriptionEvent("ws1", "pro", 10000, "new")
	tracker.RecordSubscriptionEvent("ws1", "pro", 0, "churn")

	mrr := tracker.GetMRR()
	if mrr != 0 {
		t.Fatalf("expected MRR 0 after churn, got %d", mrr)
	}
}

func TestGetMRRSnapshots(t *testing.T) {
	tracker := NewMRRTracker()
	tracker.RecordSubscriptionEvent("ws1", "pro", 10000, "new")
	tracker.RecordSubscriptionEvent("ws2", "pro", 12000, "new")

	snapshots := tracker.GetMRRSnapshots(10)
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}
}

func TestCohortRetention(t *testing.T) {
	tracker := NewCohortTracker()
	cohortWeek := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)

	// Users active during cohort week
	tracker.RecordUserActivity("ws1", "u1", cohortWeek.Add(1*time.Hour))
	tracker.RecordUserActivity("ws1", "u2", cohortWeek.Add(2*time.Hour))

	// u1 active after 7-day retention period
	tracker.RecordUserActivity("ws1", "u1", cohortWeek.AddDate(0, 0, 8))

	retention := tracker.ComputeRetention(cohortWeek, 7)
	if retention.TotalUsers != 2 {
		t.Fatalf("expected 2 total users, got %d", retention.TotalUsers)
	}
	if retention.RetainedUsers != 1 {
		t.Fatalf("expected 1 retained user, got %d", retention.RetainedUsers)
	}
	if retention.RetentionPct != 50 {
		t.Fatalf("expected 50%% retention, got %f", retention.RetentionPct)
	}
}

func TestCohortRetentionEmpty(t *testing.T) {
	tracker := NewCohortTracker()
	cohortWeek := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	retention := tracker.ComputeRetention(cohortWeek, 7)
	if retention.TotalUsers != 0 {
		t.Fatalf("expected 0, got %d", retention.TotalUsers)
	}
}

func TestSkillACL(t *testing.T) {
	svc := NewSkillACLService()
	err := svc.SetSkillACL("ws1", "u1", []string{}, []string{"skill1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allowed, _ := svc.CheckSkillAccess("ws1", "u1", "skill1")
	if allowed {
		t.Fatal("expected skill to be denied")
	}
	// Default (no ACL set) should be allowed
	allowed2, _ := svc.CheckSkillAccess("ws1", "u2", "skill1")
	if !allowed2 {
		t.Fatal("expected default allow")
	}
}

func TestSkillACLMissingFields(t *testing.T) {
	svc := NewSkillACLService()
	err := svc.SetSkillACL("", "u1", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	err = svc.SetSkillACL("ws1", "", nil, nil)
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestOAuthExpiryTracker(t *testing.T) {
	tracker := NewOAuthExpiryTracker()
	now := tracker.now()
	soon := now.Add(12 * time.Hour)
	far := now.Add(48 * time.Hour)

	tracker.TrackToken("ws1", "google", "tok1", soon)
	tracker.TrackToken("ws1", "slack", "tok2", far)

	expiring := tracker.GetExpiringSoon(24 * time.Hour)
	if len(expiring) != 1 {
		t.Fatalf("expected 1 expiring token, got %d", len(expiring))
	}
	if expiring[0].Provider != "google" {
		t.Fatalf("expected google, got %s", expiring[0].Provider)
	}
}

func TestOAuthExpiryUpdate(t *testing.T) {
	tracker := NewOAuthExpiryTracker()
	now := tracker.now()
	tracker.TrackToken("ws1", "google", "tok1", now.Add(1*time.Hour))
	// Update same token to expire later
	tracker.TrackToken("ws1", "google", "tok1", now.Add(48*time.Hour))

	expiring := tracker.GetExpiringSoon(24 * time.Hour)
	if len(expiring) != 0 {
		t.Fatalf("expected 0 expiring after update, got %d", len(expiring))
	}
}

func TestComputeRiskScore(t *testing.T) {
	scorer := NewBehavioralRiskScorer()
	// Record actions at unusual hours to trigger risk factors
	scorer.RecordAction("ws1", "u1", "permission_escalation", nil)
	scorer.RecordAction("ws1", "u1", "admin_access_attempt", nil)

	rs := scorer.ComputeRiskScore("ws1", "u1")
	if rs.Score <= 0 {
		t.Fatal("expected positive score")
	}
	if len(rs.Factors) == 0 {
		t.Fatal("expected at least one factor")
	}
}

func TestComputeRiskScoreNoActions(t *testing.T) {
	scorer := NewBehavioralRiskScorer()
	rs := scorer.ComputeRiskScore("ws1", "u1")
	if rs.Score != 0 {
		t.Fatalf("expected zero score for no actions, got %f", rs.Score)
	}
	if rs.Level != "low" {
		t.Fatalf("expected low level, got %s", rs.Level)
	}
}

func TestFeatureAdoption(t *testing.T) {
	tracker := NewFeatureAdoptionTracker(10)
	tracker.RecordAdoption("ws1", "chat")
	tracker.RecordAdoption("ws2", "chat")
	tracker.RecordAdoption("ws1", "search")

	rate := tracker.GetAdoptionRate("chat")
	if rate != 20 {
		t.Fatalf("expected 20%% adoption, got %f", rate)
	}

	heatmap := tracker.GetAdoptionHeatmap()
	if heatmap["chat"] != 2 {
		t.Fatalf("expected chat=2, got %d", heatmap["chat"])
	}
	if heatmap["search"] != 1 {
		t.Fatalf("expected search=1, got %d", heatmap["search"])
	}
}

func TestToolMTTR(t *testing.T) {
	tracker := NewToolMTTRTracker()
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tracker.RecordFailure("stripe", t0)
	tracker.RecordRecovery("stripe", t0.Add(2*time.Minute))

	tracker.RecordFailure("stripe", t0.Add(10*time.Minute))
	tracker.RecordRecovery("stripe", t0.Add(14*time.Minute))

	mttr := tracker.GetMTTR("stripe")
	// Average of 2min and 4min = 3min
	expected := 3 * time.Minute
	if mttr != expected {
		t.Fatalf("expected MTTR %v, got %v", expected, mttr)
	}
}

func TestToolMTTREmpty(t *testing.T) {
	tracker := NewToolMTTRTracker()
	mttr := tracker.GetMTTR("nonexistent")
	if mttr != 0 {
		t.Fatal("expected zero MTTR for unknown tool")
	}
}

func TestToolMTTRAll(t *testing.T) {
	tracker := NewToolMTTRTracker()
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	tracker.RecordFailure("stripe", t0)
	tracker.RecordRecovery("stripe", t0.Add(2*time.Minute))
	tracker.RecordFailure("slack", t0)
	tracker.RecordRecovery("slack", t0.Add(5*time.Minute))

	all := tracker.GetAllMTTR()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}
}

func TestAgentActionReplay(t *testing.T) {
	replay := NewAgentActionReplay()
	now := time.Now().UTC()

	replay.RecordAction("ws1", ActionRecord{
		UserID:    "u1",
		Action:    "invoke",
		ToolKey:   "stripe",
		Timestamp: now,
	})
	replay.RecordAction("ws1", ActionRecord{
		UserID:    "u1",
		Action:    "invoke",
		ToolKey:   "slack",
		Timestamp: now.Add(time.Minute),
	})

	actions := replay.ReplayActions("ws1", now.Add(-time.Hour), now.Add(time.Hour))
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
}

func TestAgentActionReplayGetByID(t *testing.T) {
	replay := NewAgentActionReplay()
	now := time.Now().UTC()

	replay.RecordAction("ws1", ActionRecord{
		ID:        "act_001",
		UserID:    "u1",
		Action:    "invoke",
		Timestamp: now,
	})

	action, err := replay.GetActionByID("act_001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != "invoke" {
		t.Fatalf("expected invoke, got %s", action.Action)
	}

	_, err = replay.GetActionByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}
