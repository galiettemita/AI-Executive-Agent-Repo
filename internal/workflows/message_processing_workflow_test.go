package workflows

import (
	"context"
	"slices"
	"testing"
)

func TestMessageProcessingWorkflowHappyPath(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.MessageProcessingWorkflowV1(MessageProcessingInput{
		MessageID:                    "018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f",
		WorkflowRunID:                "run-1",
		EnvelopeValid:                true,
		ClassifyConfidence:           0.91,
		DAGValid:                     true,
		DeliveryFailures:             0,
		KeywordFallbackAvailable:     true,
		DeliveryFallbackQueueAllowed: true,
	})

	if result.WorkflowID != "msg-018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f" {
		t.Fatalf("unexpected workflow id: %s", result.WorkflowID)
	}
	if result.TerminalState != MessageStateCompleted {
		t.Fatalf("unexpected terminal state: %s", result.TerminalState)
	}
	wantStates := []MessageProcessingState{
		MessageStateReceived,
		MessageStateClassifying,
		MessageStateDecomposing,
		MessageStateExecuting,
		MessageStateAggregating,
		MessageStateFormatting,
		MessageStateDelivering,
		MessageStateCompleted,
	}
	if !slices.Equal(result.States, wantStates) {
		t.Fatalf("unexpected state path: got=%v want=%v", result.States, wantStates)
	}
	if len(result.Fallbacks) != 0 {
		t.Fatalf("did not expect fallbacks on happy path: %v", result.Fallbacks)
	}
}

func TestMessageProcessingWorkflowClassificationFallback(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.MessageProcessingWorkflowV1(MessageProcessingInput{
		MessageID:                    "msg-low-confidence",
		EnvelopeValid:                true,
		ClassifyConfidence:           0.5,
		KeywordFallbackAvailable:     true,
		DAGValid:                     false,
		DeliveryFailures:             0,
		DeliveryFallbackQueueAllowed: true,
	})

	if result.TerminalState != MessageStateCompleted {
		t.Fatalf("expected completed terminal state with fallback, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "keyword_classifier_low_confidence") {
		t.Fatalf("missing expected classification fallback: %v", result.Fallbacks)
	}
	if !slices.Contains(result.Fallbacks, "single_skill_fallback") {
		t.Fatalf("missing expected decomposition fallback: %v", result.Fallbacks)
	}
}

func TestMessageProcessingWorkflowDeadLetterOnDeliveryFailure(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.MessageProcessingWorkflowV1(MessageProcessingInput{
		MessageID:                    "msg-delivery-fail",
		EnvelopeValid:                true,
		ClassifyConfidence:           0.9,
		DAGValid:                     true,
		DeliveryFailures:             3,
		DeliveryFallbackQueueAllowed: true,
		KeywordFallbackAvailable:     true,
	})

	if result.TerminalState != MessageStateDeadLetter {
		t.Fatalf("expected dead letter terminal state, got %s", result.TerminalState)
	}
	if !slices.Contains(result.Fallbacks, "delivery_dlq") {
		t.Fatalf("expected delivery_dlq fallback: %v", result.Fallbacks)
	}
}

func TestMessageProcessingWorkflowCompensationFlag(t *testing.T) {
	t.Parallel()

	svc := NewService()
	result := svc.MessageProcessingWorkflowV1(MessageProcessingInput{
		MessageID:                    "msg-exec-fail",
		EnvelopeValid:                true,
		ClassifyConfidence:           0.85,
		DAGValid:                     true,
		ExecuteError:                 true,
		RequiresCompensation:         true,
		KeywordFallbackAvailable:     true,
		DeliveryFallbackQueueAllowed: true,
	})
	if result.TerminalState != MessageStateFailed {
		t.Fatalf("expected failed terminal state, got %s", result.TerminalState)
	}
	if !result.CompensationNeeded {
		t.Fatal("expected compensation flag to be true")
	}
}

func TestDailyRhythmWorkflowStateMachine(t *testing.T) {
	t.Parallel()

	svc := NewService()
	success := svc.DailyRhythmWorkflowV1(DailyRhythmInput{
		UserID:              "u-1",
		HasProfile:          true,
		HasScheduleContext:  true,
		ComposeFailed:       false,
		DeliveryFailures:    1,
		DeliveryRetryBudget: 3,
	})
	if success.TerminalState != DailyRhythmStateCompleted {
		t.Fatalf("expected completed daily rhythm state, got %s", success.TerminalState)
	}
	failed := svc.DailyRhythmWorkflowV1(DailyRhythmInput{
		UserID:              "u-1",
		HasProfile:          true,
		HasScheduleContext:  true,
		ComposeFailed:       false,
		DeliveryFailures:    3,
		DeliveryRetryBudget: 3,
	})
	if failed.TerminalState != DailyRhythmStateFailed {
		t.Fatalf("expected failed daily rhythm state, got %s", failed.TerminalState)
	}
}

func TestDeterministicRetryJitterMS(t *testing.T) {
	t.Parallel()

	j1 := DeterministicRetryJitterMS("run-abc", 1, 500)
	j2 := DeterministicRetryJitterMS("run-abc", 1, 500)
	if j1 != j2 {
		t.Fatalf("expected deterministic jitter, got %d and %d", j1, j2)
	}
	if j1 < 0 || j1 >= 500 {
		t.Fatalf("expected jitter in [0, 500), got %d", j1)
	}
	other := DeterministicRetryJitterMS("run-abc", 2, 500)
	if other == j1 {
		t.Fatal("expected different jitter across attempts")
	}
}

func TestWorkflowMethodCompatibilityWithExistingService(t *testing.T) {
	t.Parallel()

	svc := NewService()
	_ = svc.InteractiveTurnV1(context.Background(), "compatibility-check")
}
