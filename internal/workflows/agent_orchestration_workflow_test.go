package workflows

import (
	"slices"
	"testing"
)

func TestAgentWorkflowHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:      "exec-001",
		AgentID:          "agent-001",
		UserID:           "user-001",
		WorkerCount:      3,
		QualityThreshold: 0.7,
		QualityScore:     0.9,
		MaxIterations:    5,
		IterationCount:   1,
	})

	if result.WorkflowID != "agent-exec-001" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != AgentStateCompleted {
		t.Fatalf("expected COMPLETED, got %s", result.TerminalState)
	}
	if !slices.Contains(result.States, AgentStateDelegating) {
		t.Fatalf("expected DELEGATING state for multi-worker: %v", result.States)
	}
}

func TestAgentWorkflowTimeout(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:     "exec-002",
		TimeoutExceeded: true,
	})

	if result.TerminalState != AgentStateTimedOut {
		t.Fatalf("expected TIMED_OUT, got %s", result.TerminalState)
	}
}

func TestAgentWorkflowQualityRetry(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:      "exec-003",
		WorkerCount:      1,
		QualityThreshold: 0.8,
		QualityScore:     0.5,
		MaxIterations:    5,
		IterationCount:   2,
	})

	if result.TerminalState != AgentStateCompleted {
		t.Fatalf("expected COMPLETED with retry, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "retry_with_feedback") {
		t.Fatalf("expected retry_with_feedback fallback: %v", result.Fallbacks)
	}
}

func TestAgentWorkflowExecuteError(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:  "exec-004",
		WorkerCount:  1,
		ExecuteError: true,
	})

	if result.TerminalState != AgentStateFailed {
		t.Fatalf("expected FAILED on execute error, got %s", result.TerminalState)
	}
}

func TestAgentWorkflowQualityAcceptBestEffort(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:      "exec-005",
		WorkerCount:      1,
		QualityThreshold: 0.8,
		QualityScore:     0.5,
		MaxIterations:    3,
		IterationCount:   3,
	})

	if result.TerminalState != AgentStateCompleted {
		t.Fatalf("expected COMPLETED with accept_best_effort, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "accept_best_effort") {
		t.Fatalf("expected accept_best_effort fallback: %v", result.Fallbacks)
	}
}

func TestAgentWorkflowNoDelegationForSingleWorker(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:      "exec-006",
		WorkerCount:      1,
		QualityThreshold: 0.5,
		QualityScore:     0.9,
	})

	if slices.Contains(result.States, AgentStateDelegating) {
		t.Fatalf("should not delegate for single worker: %v", result.States)
	}
}

func TestAgentWorkflowDelegationErrorFallback(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:     "exec-007",
		WorkerCount:     3,
		DelegationError: true,
	})

	if result.TerminalState != AgentStateCompleted {
		t.Fatalf("expected COMPLETED with delegation fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "sequential_execution") {
		t.Fatalf("expected sequential_execution fallback: %v", result.Fallbacks)
	}
}

func TestAgentWorkflowIterationIncrement(t *testing.T) {
	t.Parallel()
	svc := NewService()
	result := svc.AgentOrchestrationWorkflowV1(AgentWorkflowInput{
		ExecutionID:      "exec-008",
		WorkerCount:      1,
		QualityThreshold: 0.8,
		QualityScore:     0.5,
		MaxIterations:    5,
		IterationCount:   2,
	})

	if result.Iterations != 3 {
		t.Fatalf("expected iteration count 3, got %d", result.Iterations)
	}
}

func TestAgentWorkflowIDTrimming(t *testing.T) {
	t.Parallel()
	id := AgentWorkflowID("  exec-padded  ")
	if id != "agent-exec-padded" {
		t.Fatalf("expected trimmed ID, got %s", id)
	}
}
