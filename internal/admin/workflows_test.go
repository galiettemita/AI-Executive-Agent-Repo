package admin

import (
	"testing"
	"time"
)

func newTestWorkflowService() *AdminWorkflowService {
	costSvc := NewCostAttributionService()
	revSvc := NewRevenueOpsService()
	riskSvc := NewBehavioralRiskService()
	oauthSvc := NewOAuthMonitorService()
	return NewAdminWorkflowService(costSvc, revSvc, riskSvc, oauthSvc)
}

func TestRunCostRollupWorkflow(t *testing.T) {
	t.Parallel()
	costSvc := NewCostAttributionService()
	_, _ = costSvc.RecordLLMCost(LLMCostRecord{
		WorkspaceID:         "ws1",
		UserID:              "u1",
		CostUSD:             0.50,
		WorkflowExecutionID: "wf1",
	})

	wfSvc := NewAdminWorkflowService(costSvc, NewRevenueOpsService(), NewBehavioralRiskService(), NewOAuthMonitorService())
	result := wfSvc.RunCostRollup("ws1", "wf1")
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s (%s)", result.Status, result.Error)
	}
	if result.WorkflowName != "CostRollupWorkflow" {
		t.Fatalf("expected CostRollupWorkflow, got %s", result.WorkflowName)
	}
}

func TestRunCostRollupWorkflowFailure(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	result := wfSvc.RunCostRollup("", "wf1")
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestRunDailyRollupWorkflow(t *testing.T) {
	t.Parallel()
	costSvc := NewCostAttributionService()
	today := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	costSvc.now = func() time.Time { return today.Add(2 * time.Hour) }
	_, _ = costSvc.RecordLLMCost(LLMCostRecord{
		WorkspaceID: "ws1", UserID: "u1", CostUSD: 1.0,
	})

	wfSvc := NewAdminWorkflowService(costSvc, NewRevenueOpsService(), NewBehavioralRiskService(), NewOAuthMonitorService())
	result := wfSvc.RunDailyRollup("ws1", "u1", today)
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s (%s)", result.Status, result.Error)
	}
}

func TestRunMarginSnapshotWorkflow(t *testing.T) {
	t.Parallel()
	revSvc := NewRevenueOpsService()
	_, _ = revSvc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1", EventType: "new", Amount: 500.0,
	})
	revSvc.SetCOGS("ws1", 100.0)

	wfSvc := NewAdminWorkflowService(NewCostAttributionService(), revSvc, NewBehavioralRiskService(), NewOAuthMonitorService())
	result := wfSvc.RunMarginSnapshot("ws1")
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestRunMRRSnapshotWorkflow(t *testing.T) {
	t.Parallel()
	revSvc := NewRevenueOpsService()
	_, _ = revSvc.RecordSubscriptionEvent(SubscriptionEvent{
		WorkspaceID: "ws1", EventType: "new", Amount: 99.0,
	})

	wfSvc := NewAdminWorkflowService(NewCostAttributionService(), revSvc, NewBehavioralRiskService(), NewOAuthMonitorService())
	result := wfSvc.RunMRRSnapshot("ws1")
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s (%s)", result.Status, result.Error)
	}
}

func TestRunMRRSnapshotWorkflowFailure(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	result := wfSvc.RunMRRSnapshot("")
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestRunCohortComputeWorkflow(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	week := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	result := wfSvc.RunCohortCompute("ws1", "u1", week, 4, true)
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestRunBehavioralRiskWorkflow(t *testing.T) {
	t.Parallel()
	riskSvc := NewBehavioralRiskService()
	riskSvc.RecordRiskAction("ws1", "u1", "permission_escalation")

	wfSvc := NewAdminWorkflowService(NewCostAttributionService(), NewRevenueOpsService(), riskSvc, NewOAuthMonitorService())
	result := wfSvc.RunBehavioralRisk("ws1", "u1")
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s (%s)", result.Status, result.Error)
	}
}

func TestRunBehavioralRiskWorkflowFailure(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	result := wfSvc.RunBehavioralRisk("", "u1")
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestRunOAuthExpiryCheckWorkflow(t *testing.T) {
	t.Parallel()
	oauthSvc := NewOAuthMonitorService()
	now := time.Now().UTC()
	oauthSvc.now = func() time.Time { return now }
	_, _ = oauthSvc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: now.Add(12 * time.Hour),
	})

	wfSvc := NewAdminWorkflowService(NewCostAttributionService(), NewRevenueOpsService(), NewBehavioralRiskService(), oauthSvc)
	result := wfSvc.RunOAuthExpiryCheck(24 * time.Hour)
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
}

func TestWorkflowHistory(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	wfSvc.RunMarginSnapshot("ws1")
	wfSvc.RunMRRSnapshot("ws1")
	wfSvc.RunOAuthExpiryCheck(24 * time.Hour)

	history := wfSvc.GetHistory()
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}
}

func TestWorkflowHistoryByName(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	wfSvc.RunMarginSnapshot("ws1")
	wfSvc.RunMarginSnapshot("ws2")
	wfSvc.RunOAuthExpiryCheck(24 * time.Hour)

	margins := wfSvc.GetHistoryByName("MarginSnapshotWorkflow")
	if len(margins) != 2 {
		t.Fatalf("expected 2 margin snapshots, got %d", len(margins))
	}
}

func TestWorkflowFailedWorkflows(t *testing.T) {
	t.Parallel()
	wfSvc := newTestWorkflowService()
	wfSvc.RunCostRollup("", "wf1")      // will fail
	wfSvc.RunMRRSnapshot("")             // will fail
	wfSvc.RunOAuthExpiryCheck(time.Hour) // will succeed

	failed := wfSvc.FailedWorkflows()
	if len(failed) != 2 {
		t.Fatalf("expected 2 failed workflows, got %d", len(failed))
	}
}
