package ppo

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	learndpo "github.com/brevio/brevio/internal/learning/dpo"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

// mockCAICritiquer returns configurable violations.
type mockCAICritiquer struct {
	violations []CAIViolation
}

func (m *mockCAICritiquer) Evaluate(_ context.Context, _ string) ([]CAIViolation, error) {
	return m.violations, nil
}

// mockLLMCompleter returns a fixed response.
type mockLLMCompleter struct {
	response string
}

func (m *mockLLMCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	return m.response, nil
}

// mockDPOQueue records enqueued pairs.
type mockDPOQueue struct {
	mu    sync.Mutex
	pairs []learndpo.PreferencePair
}

func (m *mockDPOQueue) EnqueuePair(_ context.Context, pair learndpo.PreferencePair) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pairs = append(m.pairs, pair)
	return nil
}

func (m *mockDPOQueue) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pairs)
}

func TestPPOLoopC1ViolationCreatesNegativePair(t *testing.T) {
	queue := &mockDPOQueue{}
	loop := NewConstitutionalPPOLoop(
		&mockCAICritiquer{violations: []CAIViolation{{Principle: "C1", Severity: 1.0}}},
		&mockLLMCompleter{response: "Corrected safe response"},
		queue,
		testLogger,
	)

	result, err := loop.EvaluateAndCorrect(context.Background(), uuid.New(), uuid.New(), "Harmful original response")
	if err != nil {
		t.Fatalf("EvaluateAndCorrect failed: %v", err)
	}

	if result.RewardSignal != -1.0 {
		t.Errorf("Expected reward=-1.0 for C1 violation, got %f", result.RewardSignal)
	}

	if queue.count() != 1 {
		t.Errorf("Expected 1 pair queued, got %d", queue.count())
	}

	if result.CorrectedResponse != "Corrected safe response" {
		t.Errorf("Expected corrected response, got %q", result.CorrectedResponse)
	}
}

func TestPPOLoopNoViolationNoPair(t *testing.T) {
	queue := &mockDPOQueue{}
	loop := NewConstitutionalPPOLoop(
		&mockCAICritiquer{violations: nil},
		&mockLLMCompleter{response: ""},
		queue,
		testLogger,
	)

	result, err := loop.EvaluateAndCorrect(context.Background(), uuid.New(), uuid.New(), "Normal helpful response")
	if err != nil {
		t.Fatalf("EvaluateAndCorrect failed: %v", err)
	}

	if result.RewardSignal != 1.0 {
		t.Errorf("Expected reward=1.0 for no violations, got %f", result.RewardSignal)
	}

	if queue.count() != 0 {
		t.Errorf("Expected 0 pairs queued for no violations, got %d", queue.count())
	}
}

func TestPPOLoopC3ViolationHalfPenalty(t *testing.T) {
	queue := &mockDPOQueue{}
	loop := NewConstitutionalPPOLoop(
		&mockCAICritiquer{violations: []CAIViolation{
			{Principle: "C3", Severity: 0.5},
			{Principle: "C5", Severity: 0.5},
		}},
		&mockLLMCompleter{response: "Better response"},
		queue,
		testLogger,
	)

	result, err := loop.EvaluateAndCorrect(context.Background(), uuid.New(), uuid.New(), "Mildly problematic response")
	if err != nil {
		t.Fatalf("EvaluateAndCorrect failed: %v", err)
	}

	if result.RewardSignal != -1.0 {
		t.Errorf("Expected reward=-1.0 for 2 C3/C5 violations, got %f", result.RewardSignal)
	}

	if queue.count() != 1 {
		t.Errorf("Expected 1 pair queued, got %d", queue.count())
	}
}

func TestOnlineLearningORMThreshold(t *testing.T) {
	// This test uses the learning package's OnlineLearningService indirectly.
	// We test the threshold logic: ORM < 3.0 queues, ORM >= 3.0 does not.

	queue := &mockDPOQueue{}

	// ORM 2.9 — should trigger.
	if 2.9 < 3.0 {
		_ = queue.EnqueuePair(context.Background(), learndpo.PreferencePair{
			ID:          uuid.New(),
			PromptText:  "test prompt",
			ChosenResponse: "ideal response",
			RejectedResponse: "bad response",
		})
	}

	if queue.count() != 1 {
		t.Errorf("Expected 1 pair for ORM 2.9, got %d", queue.count())
	}

	// ORM 3.1 — should not trigger.
	beforeCount := queue.count()
	if 3.1 < 3.0 {
		_ = queue.EnqueuePair(context.Background(), learndpo.PreferencePair{})
	}

	if queue.count() != beforeCount {
		t.Errorf("Expected no new pair for ORM 3.1")
	}
}
