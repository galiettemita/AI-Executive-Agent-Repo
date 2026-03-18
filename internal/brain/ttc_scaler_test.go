package brain

import (
	"testing"
)

func TestComputeThinkingBudget_Simple(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1}, nil); got != 4096 {
		t.Errorf("expected 4096, got %d", got)
	}
}

func TestComputeThinkingBudget_TwoDomains(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 2}, nil); got != 8192 {
		t.Errorf("expected 8192, got %d", got)
	}
}

func TestComputeThinkingBudget_HasDependencies(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1, HasDependencies: true}, nil); got != 8192 {
		t.Errorf("expected 8192, got %d", got)
	}
}

func TestComputeThinkingBudget_ThreeDomains(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 3}, nil); got != 16384 {
		t.Errorf("expected 16384, got %d", got)
	}
}

func TestComputeThinkingBudget_ManyIntents(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1, IntentCount: 3}, nil); got != 16384 {
		t.Errorf("expected 16384, got %d", got)
	}
}

func TestComputeThinkingBudget_Complex(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 3, IntentCount: 3, EntityCount: 9}, nil); got != 32768 {
		t.Errorf("expected 32768, got %d", got)
	}
}

func TestComputeThinkingBudget_ManyEntities(t *testing.T) {
	t.Parallel()
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1, EntityCount: 10}, nil); got != 32768 {
		t.Errorf("expected 32768, got %d", got)
	}
}

func TestComputeThinkingBudget_ORMRetry(t *testing.T) {
	t.Parallel()
	prev := &OutcomeScore{OverallQuality: 2.0}
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1}, prev); got != 8192 {
		t.Errorf("expected 8192 (4096*2), got %d", got)
	}
}

func TestComputeThinkingBudget_ORMRetryComplex(t *testing.T) {
	t.Parallel()
	prev := &OutcomeScore{OverallQuality: 1.5}
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 3, IntentCount: 5, EntityCount: 10}, prev); got != 65536 {
		t.Errorf("expected 65536 (32768*2 capped), got %d", got)
	}
}

func TestComputeThinkingBudget_CapAt65536(t *testing.T) {
	t.Parallel()
	prev := &OutcomeScore{OverallQuality: 1.5}
	got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 3, IntentCount: 5, EntityCount: 10}, prev)
	if got > 65536 {
		t.Errorf("exceeded cap: got %d", got)
	}
}

func TestComputeThinkingBudget_HighScoreNoEscalation(t *testing.T) {
	t.Parallel()
	prev := &OutcomeScore{OverallQuality: 4.0}
	if got := ComputeThinkingBudget(ComplexitySignals{DomainCount: 1}, prev); got != 4096 {
		t.Errorf("expected 4096 (no escalation for good score), got %d", got)
	}
}

func TestExtractComplexityFromSteps(t *testing.T) {
	t.Parallel()
	steps := []PlanStep{
		{ToolKey: "calendar.read", Phase: "gather"},
		{ToolKey: "calendar.create_event", Phase: "act", DependsOn: []int{0}},
		{ToolKey: "email.send", Phase: "act"},
	}
	sig := ExtractComplexityFromSteps(steps, 2, 3, true, false)
	if sig.DomainCount != 2 {
		t.Errorf("expected 2 domains, got %d", sig.DomainCount)
	}
	if sig.IntentCount != 2 {
		t.Errorf("expected 2 intents, got %d", sig.IntentCount)
	}
	if sig.EntityCount != 3 {
		t.Errorf("expected 3 entities, got %d", sig.EntityCount)
	}
	if !sig.HasDependencies {
		t.Error("expected HasDependencies=true")
	}
}
