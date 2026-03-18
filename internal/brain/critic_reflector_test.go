package brain

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCritique_HighQuality(t *testing.T) {
	svc := NewCriticReflectorService()

	output, err := svc.Critique(ExecutionTrace{
		WorkspaceID: "ws1",
		Intent:      "read email",
		PlanSteps:   3,
		Succeeded:   3,
		Failed:      0,
		ToolsUsed:   []string{"email_read"},
		Duration:    2 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !output.Passed {
		t.Fatalf("expected pass, got score %f", output.OverallScore)
	}
	if output.OverallScore < 0.7 {
		t.Fatalf("expected high score, got %f", output.OverallScore)
	}
	if len(output.FailureModes) != 0 {
		t.Fatalf("expected no failure modes, got %v", output.FailureModes)
	}
}

func TestCritique_LowQuality(t *testing.T) {
	svc := NewCriticReflectorService()

	output, err := svc.Critique(ExecutionTrace{
		WorkspaceID: "ws1",
		Intent:      "complex multi-step",
		PlanSteps:   10,
		Succeeded:   2,
		Failed:      8,
		ToolsUsed:   []string{"a", "b", "c"},
		Duration:    45 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output.Passed {
		t.Fatal("expected fail for low quality execution")
	}
	if len(output.FailureModes) == 0 {
		t.Fatal("expected failure modes")
	}
	if output.ImprovementDirective == "" {
		t.Fatal("expected improvement directive")
	}
}

func TestCritique_EmptyWorkspace(t *testing.T) {
	svc := NewCriticReflectorService()
	_, err := svc.Critique(ExecutionTrace{})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestReflect(t *testing.T) {
	svc := NewCriticReflectorService()

	criticOutput := &CriticOutput{
		OverallScore: 0.4,
		FailureModes: []string{"low_completeness", "low_reliability"},
	}
	trace := ExecutionTrace{
		WorkspaceID: "ws1",
		PlanSteps:   5,
		Succeeded:   1,
		Failed:      4,
	}

	reflectorOutput, err := svc.Reflect(criticOutput, trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reflectorOutput.LessonCandidates) == 0 {
		t.Fatal("expected lesson candidates")
	}
	if !reflectorOutput.PatternDetected {
		t.Fatal("expected pattern detection for multiple failures")
	}
}

func TestReflect_EscalateToFeedback(t *testing.T) {
	svc := NewCriticReflectorService()

	criticOutput := &CriticOutput{
		OverallScore: 0.2,
		FailureModes: []string{"low_completeness"},
	}
	trace := ExecutionTrace{WorkspaceID: "ws1", Failed: 0}

	reflectorOutput, err := svc.Reflect(criticOutput, trace)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflectorOutput.EscalateToFeedback {
		t.Fatal("expected escalation for very low score")
	}
}

func TestGetCritiqueHistory(t *testing.T) {
	svc := NewCriticReflectorService()

	_, _ = svc.Critique(ExecutionTrace{WorkspaceID: "ws1", PlanSteps: 1, Succeeded: 1, Duration: time.Second})
	_, _ = svc.Critique(ExecutionTrace{WorkspaceID: "ws2", PlanSteps: 2, Succeeded: 1, Failed: 1, Duration: time.Second})

	history := svc.GetCritiqueHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
}

// ---------------------------------------------------------------------------
// Ring buffer tests
// ---------------------------------------------------------------------------

func TestRingBuffer_WrapAround(t *testing.T) {
	t.Parallel()

	rb := newRingBuffer(3)
	for i := 0; i < 5; i++ {
		rb.push(CriticOutput{ID: string(rune('A' + i))})
	}
	snap := rb.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3 entries after wraparound, got %d", len(snap))
	}
	// Should be the last 3 pushed: C, D, E
	if snap[0].ID != string(rune('C')) || snap[1].ID != string(rune('D')) || snap[2].ID != string(rune('E')) {
		t.Errorf("expected [C D E], got [%s %s %s]", snap[0].ID, snap[1].ID, snap[2].ID)
	}
}

func TestRingBuffer_SnapshotOrder(t *testing.T) {
	t.Parallel()

	rb := newRingBuffer(5)
	rb.push(CriticOutput{ID: "A"})
	rb.push(CriticOutput{ID: "B"})
	rb.push(CriticOutput{ID: "C"})

	snap := rb.snapshot()
	if len(snap) != 3 {
		t.Fatalf("expected 3, got %d", len(snap))
	}
	if snap[0].ID != "A" || snap[1].ID != "B" || snap[2].ID != "C" {
		t.Errorf("expected [A B C], got [%s %s %s]", snap[0].ID, snap[1].ID, snap[2].ID)
	}
}

func TestCriticReflectorService_BoundedMemory(t *testing.T) {
	t.Parallel()

	svc := NewCriticReflectorServiceWithConfig(CriticReflectorConfig{
		RingBufferSize: 1000,
	})

	for i := 0; i < 2000; i++ {
		_, _ = svc.Critique(ExecutionTrace{
			WorkspaceID: "ws1",
			PlanSteps:   1,
			Succeeded:   1,
			Duration:    time.Millisecond,
		})
	}

	history := svc.GetCritiqueHistory()
	if len(history) > 1000 {
		t.Fatalf("expected ≤1000 entries, got %d", len(history))
	}
	if len(history) != 1000 {
		t.Fatalf("expected exactly 1000 entries (full ring), got %d", len(history))
	}
}

// mockCriticTraceRepo records Save calls for testing.
type mockCriticTraceRepo struct {
	mu    sync.Mutex
	saved []CriticOutput
}

func (m *mockCriticTraceRepo) Save(_ context.Context, output CriticOutput) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved = append(m.saved, output)
	return nil
}

func (m *mockCriticTraceRepo) StoreORMResult(_ context.Context, _, _ string, _ *OutcomeScore) error {
	return nil
}

func TestCriticReflectorService_RepositorySaveAsync(t *testing.T) {
	t.Parallel()

	repo := &mockCriticTraceRepo{}
	svc := NewCriticReflectorServiceWithConfig(CriticReflectorConfig{
		Repository: repo,
	})

	for i := 0; i < 5; i++ {
		_, _ = svc.Critique(ExecutionTrace{
			WorkspaceID: "ws1",
			PlanSteps:   1,
			Succeeded:   1,
			Duration:    time.Millisecond,
		})
	}

	// Give async goroutines time to complete.
	time.Sleep(50 * time.Millisecond)

	repo.mu.Lock()
	count := len(repo.saved)
	repo.mu.Unlock()

	if count != 5 {
		t.Fatalf("expected 5 Save calls, got %d", count)
	}
}
