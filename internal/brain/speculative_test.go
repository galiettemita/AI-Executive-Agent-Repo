package brain

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockSpecToolExecutor is a thread-safe test double for ToolExecutor.
type mockSpecToolExecutor struct {
	mu       sync.Mutex
	callLog  []string
	response map[string]any
	err      error
	delay    time.Duration
}

func (m *mockSpecToolExecutor) Execute(
	ctx context.Context,
	toolKey string,
	params map[string]any,
) (map[string]any, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	m.mu.Lock()
	m.callLog = append(m.callLog, toolKey)
	m.mu.Unlock()
	return m.response, m.err
}

func (m *mockSpecToolExecutor) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.callLog)
}

// TestSpeculativeExecutor_ConsumePrecomputed proves: a result pre-executed by
// PreExecute is returned by Consume without re-invoking the tool executor.
func TestSpeculativeExecutor_ConsumePrecomputed(t *testing.T) {
	t.Parallel()

	mock := &mockSpecToolExecutor{
		response: map[string]any{"events": []string{"standup 9am", "1:1 3pm"}},
	}
	exec := NewSpeculativeExecutor(mock, 0.60)

	predictions := []SpeculativePrediction{
		{
			ToolKey:    "calendar_read",
			Input:      map[string]any{},
			Confidence: 0.65,
			Rationale:  "test: above threshold",
		},
	}

	exec.PreExecute(context.Background(), predictions)

	var result *SpeculativeResult
	var hit bool
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		result, hit = exec.Consume("calendar_read", map[string]any{})
		if hit {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !hit {
		t.Fatal("Consume: expected cache hit after PreExecute; got miss after 2s deadline")
	}
	if result == nil {
		t.Fatal("Consume: hit=true but result is nil")
	}
	if result.Error != nil {
		t.Fatalf("Consume: unexpected error in speculative result: %v", result.Error)
	}
	if result.Output == nil {
		t.Fatal("Consume: result.Output is nil; expected non-nil output from mock")
	}
	if !result.Used {
		t.Fatal("Consume: result.Used should be true after successful Consume")
	}
	if result.LatencyMs < 0 {
		t.Fatalf("Consume: result.LatencyMs should be >= 0; got %d", result.LatencyMs)
	}

	_, secondHit := exec.Consume("calendar_read", map[string]any{})
	if secondHit {
		t.Fatal("second Consume: expected false (key must be deleted after first hit); got true")
	}

	if got := mock.calls(); got != 1 {
		t.Fatalf("ExecuteTool call count: expected 1; got %d", got)
	}
	mock.mu.Lock()
	firstKey := mock.callLog[0]
	mock.mu.Unlock()
	if firstKey != "calendar_read" {
		t.Fatalf("ExecuteTool called with toolKey=%q; expected calendar_read", firstKey)
	}
}

// TestSpeculativeExecutor_BelowConfidenceSkipped proves: a prediction with
// Confidence < minConfidence is never pre-executed.
func TestSpeculativeExecutor_BelowConfidenceSkipped(t *testing.T) {
	t.Parallel()

	mock := &mockSpecToolExecutor{
		response: map[string]any{"should": "never appear"},
	}
	exec := NewSpeculativeExecutor(mock, 0.60)

	predictions := []SpeculativePrediction{
		{
			ToolKey:    "web_search",
			Input:      map[string]any{"query": "brevio"},
			Confidence: 0.50,
			Rationale:  "test: below threshold",
		},
	}

	exec.PreExecute(context.Background(), predictions)

	time.Sleep(200 * time.Millisecond)

	if got := mock.calls(); got != 0 {
		t.Fatalf("ExecuteTool call count: expected 0 for below-threshold prediction; got %d", got)
	}

	_, hit := exec.Consume("web_search", map[string]any{"query": "brevio"})
	if hit {
		t.Fatal("Consume: expected false for below-threshold prediction; got true")
	}
}
