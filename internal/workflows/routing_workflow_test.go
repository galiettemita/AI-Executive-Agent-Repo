package workflows

import (
	"slices"
	"testing"
)

func TestRoutingWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:     "req-001",
		UserID:        "user-001",
		SelectedModel: "claude-sonnet-4-20250514",
		ModelHealthy:  true,
		FallbackModel: "claude-haiku-4-5-20251001",
	})

	if result.WorkflowID != "routing-req-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != RoutingStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if result.SelectedModel != "claude-sonnet-4-20250514" {
		t.Fatalf("unexpected model: %s", result.SelectedModel)
	}
	wantStates := []RoutingDecisionState{
		RoutingStateInit, RoutingStateClassifying, RoutingStateSelecting,
		RoutingStateValidating, RoutingStateCompleted,
	}
	if !slices.Equal(result.States, wantStates) {
		t.Fatalf("unexpected states: %v", result.States)
	}
}

func TestRoutingWorkflowHealthFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:     "req-002",
		SelectedModel: "claude-opus-4-6",
		ModelHealthy:  false,
		FallbackModel: "claude-sonnet-4-20250514",
	})

	if result.TerminalState != RoutingStateCompleted {
		t.Fatalf("expected COMPLETED with fallback, got %s", result.TerminalState)
	}
	if result.SelectedModel != "claude-sonnet-4-20250514" {
		t.Fatalf("expected fallback model, got %s", result.SelectedModel)
	}
	if !slices.Contains(result.Fallbacks, "health_fallback") {
		t.Fatalf("expected health_fallback: %v", result.Fallbacks)
	}
}

func TestRoutingWorkflowNoFallbackFail(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:    "req-003",
		SelectError:  true,
		ModelHealthy: true,
	})

	if result.TerminalState != RoutingStateFailed {
		t.Fatalf("expected FAILED with no fallback, got %s", result.TerminalState)
	}
}

func TestRoutingWorkflowSelectErrorWithFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:     "req-004",
		SelectError:   true,
		FallbackModel: "claude-haiku-4-5-20251001",
		ModelHealthy:  true,
	})

	if result.TerminalState != RoutingStateCompleted {
		t.Fatalf("expected COMPLETED with select fallback, got %s", result.TerminalState)
	}
	if result.SelectedModel != "claude-haiku-4-5-20251001" {
		t.Fatalf("expected fallback model, got %s", result.SelectedModel)
	}
	if !slices.Contains(result.Fallbacks, "fallback_model") {
		t.Fatalf("expected fallback_model: %v", result.Fallbacks)
	}
}

func TestRoutingWorkflowBudgetExceeded(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:      "req-005",
		SelectedModel:  "claude-opus-4-6",
		ModelHealthy:   true,
		BudgetExceeded: true,
	})

	if result.TerminalState != RoutingStateCompleted {
		t.Fatalf("expected COMPLETED with budget downgrade, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "budget_downgrade") {
		t.Fatalf("expected budget_downgrade fallback: %v", result.Fallbacks)
	}
}

func TestRoutingWorkflowClassifyErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:     "req-006",
		ClassifyError: true,
		SelectedModel: "claude-sonnet-4-20250514",
		ModelHealthy:  true,
	})

	if result.TerminalState != RoutingStateCompleted {
		t.Fatalf("expected COMPLETED with classify fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "default_complexity") {
		t.Fatalf("expected default_complexity fallback: %v", result.Fallbacks)
	}
}

func TestRoutingWorkflowUnhealthyNoFallbackFails(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.RoutingDecisionWorkflowV1(RoutingWorkflowInput{
		RequestID:     "req-007",
		SelectedModel: "claude-opus-4-6",
		ModelHealthy:  false,
		FallbackModel: "",
	})

	if result.TerminalState != RoutingStateFailed {
		t.Fatalf("expected FAILED with no fallback for unhealthy model, got %s", result.TerminalState)
	}
}

func TestRoutingWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	id := RoutingWorkflowID("  req-padded  ")
	if id != "routing-req-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}
