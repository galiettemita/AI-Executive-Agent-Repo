package admin

import (
	"math"
	"testing"
	"time"
)

func floatClose(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestRecordLLMCostHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	rec, err := svc.RecordLLMCost(LLMCostRecord{
		WorkspaceID:         "ws1",
		UserID:              "u1",
		ModelID:             "gpt-4",
		InputTokens:         100,
		OutputTokens:        50,
		CostUSD:             0.05,
		WorkflowExecutionID: "wf1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected generated ID")
	}
	if rec.CreatedAt.IsZero() {
		t.Fatal("expected non-zero CreatedAt")
	}
}

func TestRecordLLMCostMissingWorkspace(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, err := svc.RecordLLMCost(LLMCostRecord{UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestRecordLLMCostMissingUser(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, err := svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestRecordConnectorCostHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	rec, err := svc.RecordConnectorCost(ConnectorCostRecord{
		WorkspaceID: "ws1",
		UserID:      "u1",
		ConnectorID: "stripe",
		CallCount:   5,
		CostUSD:     0.10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("expected generated ID")
	}
}

func TestRecordConnectorCostMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, err := svc.RecordConnectorCost(ConnectorCostRecord{UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.RecordConnectorCost(ConnectorCostRecord{WorkspaceID: "ws1"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

func TestRollupTaskCost(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, _ = svc.RecordLLMCost(LLMCostRecord{
		WorkspaceID:         "ws1",
		UserID:              "u1",
		CostUSD:             0.10,
		WorkflowExecutionID: "wf1",
	})
	_, _ = svc.RecordLLMCost(LLMCostRecord{
		WorkspaceID:         "ws1",
		UserID:              "u1",
		CostUSD:             0.20,
		WorkflowExecutionID: "wf1",
	})
	_, _ = svc.RecordConnectorCost(ConnectorCostRecord{
		WorkspaceID: "ws1",
		UserID:      "u1",
		CostUSD:     0.05,
	})

	rollup, err := svc.RollupTaskCost("ws1", "wf1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !floatClose(rollup.LLMCostUSD, 0.30, 0.001) {
		t.Fatalf("expected LLM cost ~0.30, got %f", rollup.LLMCostUSD)
	}
	if !floatClose(rollup.TotalCostUSD, 0.35, 0.001) {
		t.Fatalf("expected total ~0.35, got %f", rollup.TotalCostUSD)
	}
}

func TestRollupTaskCostMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, err := svc.RollupTaskCost("", "wf1")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.RollupTaskCost("ws1", "")
	if err == nil {
		t.Fatal("expected error for missing workflow_execution_id")
	}
}

func TestRollupDailyUserCost(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	today := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return today.Add(2 * time.Hour) }

	_, _ = svc.RecordLLMCost(LLMCostRecord{
		WorkspaceID: "ws1",
		UserID:      "u1",
		CostUSD:     0.50,
	})
	_, _ = svc.RecordConnectorCost(ConnectorCostRecord{
		WorkspaceID: "ws1",
		UserID:      "u1",
		CostUSD:     0.10,
	})

	rollup, err := svc.RollupDailyUserCost("ws1", "u1", today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rollup.TotalCostUSD != 0.60 {
		t.Fatalf("expected 0.60, got %f", rollup.TotalCostUSD)
	}
}

func TestGetCostSummary(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, _ = svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1", UserID: "u1", CostUSD: 1.0})
	_, _ = svc.RecordConnectorCost(ConnectorCostRecord{WorkspaceID: "ws1", UserID: "u1", CostUSD: 0.5})

	summary, err := svc.GetCostSummary("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.TotalCostUSD != 1.5 {
		t.Fatalf("expected 1.5, got %f", summary.TotalCostUSD)
	}
	if summary.RecordCount != 2 {
		t.Fatalf("expected 2 records, got %d", summary.RecordCount)
	}
}

func TestGetCostSummaryEmpty(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, err := svc.GetCostSummary("")
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestGetUserCosts(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	_, _ = svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1", UserID: "u1", CostUSD: 1.0})
	_, _ = svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1", UserID: "u2", CostUSD: 2.0})

	costs, err := svc.GetUserCosts("ws1", "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(costs) != 1 {
		t.Fatalf("expected 1 record, got %d", len(costs))
	}
}

func TestGetCostProjections(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	day1 := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2025, 6, 2, 10, 0, 0, 0, time.UTC)

	_, _ = svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1", UserID: "u1", CostUSD: 10.0, CreatedAt: day1})
	_, _ = svc.RecordLLMCost(LLMCostRecord{WorkspaceID: "ws1", UserID: "u1", CostUSD: 20.0, CreatedAt: day2})

	proj, err := svc.GetCostProjections("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.DaysObserved != 2 {
		t.Fatalf("expected 2 days, got %d", proj.DaysObserved)
	}
	if proj.DailyAvgCostUSD != 15.0 {
		t.Fatalf("expected daily avg 15.0, got %f", proj.DailyAvgCostUSD)
	}
	if proj.ProjectedMonthUSD != 450.0 {
		t.Fatalf("expected projected 450.0, got %f", proj.ProjectedMonthUSD)
	}
}

func TestGetCostProjectionsEmpty(t *testing.T) {
	t.Parallel()
	svc := NewCostAttributionService()
	proj, err := svc.GetCostProjections("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proj.DaysObserved != 0 {
		t.Fatalf("expected 0 days observed, got %d", proj.DaysObserved)
	}
}
