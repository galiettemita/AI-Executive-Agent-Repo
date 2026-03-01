package contextlayer

import (
	"testing"
	"time"
)

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

func TestAttentionBudgetForTier(t *testing.T) {
	t.Parallel()

	budget := AttentionBudgetForTier("T3")
	if budget.MaxInputTokens != 64000 || budget.MaxTotalTokens != 100000 || budget.MaxLLMCallsPerTurn != 15 {
		t.Fatalf("unexpected T3 attention budget: %+v", budget)
	}
}

func TestAssembleDeterministicContextTruncationOrder(t *testing.T) {
	t.Parallel()

	requested := map[string]int{
		"system_prompt":        2000,
		"workspace_context":    1000,
		"tool_registry":        2000,
		"memory_items":         3000,
		"conversation_history": 4000,
		"current_turn":         2000,
		"prior_tool_results":   2000,
		"evidence_citations":   1000,
	}

	// T0 budget is intentionally tiny relative to full slot max aggregate.
	result := AssembleDeterministicContext("T0", requested)
	if result.TotalFinalTokens > result.BudgetTokens {
		t.Fatalf("expected assembled context to fit budget: %+v", result)
	}

	slotByName := map[string]ContextSlot{}
	for _, slot := range result.Slots {
		slotByName[slot.Name] = slot
	}
	if slotByName["system_prompt"].TruncatedTokens != 0 || slotByName["current_turn"].TruncatedTokens != 0 {
		t.Fatalf("expected never-truncate slots untouched: %+v", slotByName)
	}
	// Priority-2 evidence should truncate before lower-priority slots.
	if slotByName["evidence_citations"].TruncatedTokens == 0 {
		t.Fatalf("expected evidence slot to truncate first under heavy overflow: %+v", slotByName)
	}
}

func TestDeterministicSortKeys(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	memories := SortMemoryCandidates([]MemoryCandidate{
		{ID: "m1", CosineSimilarity: 0.9, CreatedAt: now.Add(-time.Hour)},
		{ID: "m2", CosineSimilarity: 0.9, CreatedAt: now},
		{ID: "m3", CosineSimilarity: 0.7, CreatedAt: now.Add(time.Hour)},
	})
	if memories[0].ID != "m2" {
		t.Fatalf("expected memory tiebreaker by created_at desc, got %+v", memories)
	}

	history := SortConversationTurns([]ConversationTurn{
		{IngressTurnID: "turn-1", CreatedAt: now},
		{IngressTurnID: "turn-2", CreatedAt: now},
		{IngressTurnID: "turn-3", CreatedAt: now.Add(-time.Minute)},
	})
	if history[0].IngressTurnID != "turn-2" {
		t.Fatalf("expected conversation tiebreaker ingress_turn_id desc, got %+v", history)
	}

	results := SortToolResultItems([]ToolResultItem{
		{Sequence: 2, ToolExecutionID: "b"},
		{Sequence: 1, ToolExecutionID: "z"},
		{Sequence: 1, ToolExecutionID: "a"},
	})
	if results[0].ToolExecutionID != "a" || results[1].ToolExecutionID != "z" {
		t.Fatalf("expected tool-result sequence asc then id asc, got %+v", results)
	}

	evidence := SortEvidenceItems([]EvidenceItem{
		{Confidence: 0.8, SourceTurnID: "s1"},
		{Confidence: 0.8, SourceTurnID: "s3"},
		{Confidence: 0.9, SourceTurnID: "s2"},
	})
	if evidence[0].SourceTurnID != "s2" || evidence[1].SourceTurnID != "s3" {
		t.Fatalf("expected evidence confidence desc then source_turn_id desc, got %+v", evidence)
	}
}
