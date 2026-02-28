package contextlayer

import "testing"

func TestBudgetAndAllocationLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	budget := s.SetBudget("ws_1", 2048, "active")
	if budget.BudgetTokens != 2048 {
		t.Fatalf("unexpected budget tokens: %d", budget.BudgetTokens)
	}

	stored, ok := s.GetBudget("ws_1")
	if !ok {
		t.Fatal("expected budget to exist")
	}
	if stored.Status != "active" {
		t.Fatalf("unexpected budget status: %s", stored.Status)
	}
	if stored.MaxContextTokens != 2048 || stored.ReservedResponseTokens == 0 {
		t.Fatalf("unexpected normalized budget config: %+v", stored)
	}

	s.SetAllocations("ws_1", map[string]int{
		"history":   1024,
		"retrieval": 512,
		"tool":      256,
	})
	allocs := s.GetAllocations("ws_1")
	if len(allocs) != 3 {
		t.Fatalf("unexpected allocation count: %d", len(allocs))
	}
	if allocs[0].ItemType != "history" {
		t.Fatalf("expected deterministic item ordering, got %s", allocs[0].ItemType)
	}
}

func TestAllocateContextDeterministicAndOverflowGate(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertBudgetConfig("ws_2", "T2", 2048, 256, 512, "active")

	okReport, err := s.AllocateContext("ws_2", "turn_1", 800, 400, 200)
	if err != nil {
		t.Fatalf("expected successful allocation, got %v", err)
	}
	if okReport.AllocatedPromptTokens != 800 || okReport.AllocatedRAGTokens != 400 || okReport.AllocatedHistoryTokens != 200 {
		t.Fatalf("unexpected deterministic allocation report: %+v", okReport)
	}
	if okReport.Overflowed {
		t.Fatalf("did not expect overflow for valid allocation: %+v", okReport)
	}

	overflowReport, err := s.AllocateContext("ws_2", "turn_2", 1600, 800, 500)
	if err == nil {
		t.Fatal("expected context budget overflow error")
	}
	if !overflowReport.Overflowed {
		t.Fatalf("expected overflow report state, got %+v", overflowReport)
	}
}
