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
