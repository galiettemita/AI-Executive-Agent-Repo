package cognition

import (
	"testing"
)

func TestLearnHeuristic(t *testing.T) {
	e := NewDualProcessEngine()
	h, err := e.LearnHeuristic("greeting", "Hello!", "social", "user-interaction")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if h.Pattern != "greeting" {
		t.Fatalf("expected pattern 'greeting', got %s", h.Pattern)
	}
	if h.SuccessCount != 1 {
		t.Fatalf("expected success count 1, got %d", h.SuccessCount)
	}
}

func TestLearnHeuristicEmptyPattern(t *testing.T) {
	e := NewDualProcessEngine()
	_, err := e.LearnHeuristic("", "response", "d", "s")
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
}

func TestLearnHeuristicEmptyResponse(t *testing.T) {
	e := NewDualProcessEngine()
	_, err := e.LearnHeuristic("pattern", "", "d", "s")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestSystem1MatchFound(t *testing.T) {
	e := NewDualProcessEngine()
	h, _ := e.LearnHeuristic("hello", "Hi there!", "social", "test")
	// Add more successes so confidence is high
	e.RecordOutcome(h.ID, true)
	e.RecordOutcome(h.ID, true)

	result, found := e.System1Match("hello world")
	if !found {
		t.Fatal("expected match")
	}
	if result.Response != "Hi there!" {
		t.Fatalf("expected 'Hi there!', got %s", result.Response)
	}
	if result.Confidence <= 0 {
		t.Fatal("expected positive confidence")
	}
}

func TestSystem1MatchNotFound(t *testing.T) {
	e := NewDualProcessEngine()
	_, found := e.System1Match("xyz unknown")
	if found {
		t.Fatal("expected no match")
	}
}

func TestShouldEscalateNilResult(t *testing.T) {
	e := NewDualProcessEngine()
	if !e.ShouldEscalateToSystem2("test", nil) {
		t.Fatal("expected escalation for nil result")
	}
}

func TestShouldEscalateLowConfidence(t *testing.T) {
	e := NewDualProcessEngine()
	result := &System1Result{Confidence: 0.5}
	if !e.ShouldEscalateToSystem2("test", result) {
		t.Fatal("expected escalation for low confidence")
	}
}

func TestShouldNotEscalateHighConfidence(t *testing.T) {
	e := NewDualProcessEngine()
	result := &System1Result{Confidence: 0.9}
	if e.ShouldEscalateToSystem2("simple", result) {
		t.Fatal("did not expect escalation for high confidence simple input")
	}
}

func TestRecordOutcome(t *testing.T) {
	e := NewDualProcessEngine()
	h, _ := e.LearnHeuristic("test", "resp", "d", "s")

	e.RecordOutcome(h.ID, true)
	e.RecordOutcome(h.ID, false)

	e.mu.Lock()
	stored := e.heuristics[h.ID]
	e.mu.Unlock()

	if stored.SuccessCount != 2 { // 1 initial + 1 recorded
		t.Fatalf("expected 2 successes, got %d", stored.SuccessCount)
	}
	if stored.FailCount != 1 {
		t.Fatalf("expected 1 failure, got %d", stored.FailCount)
	}
}

func TestPruneIneffective(t *testing.T) {
	e := NewDualProcessEngine()
	h, _ := e.LearnHeuristic("bad", "resp", "d", "s")
	e.RecordOutcome(h.ID, false)
	e.RecordOutcome(h.ID, false)
	e.RecordOutcome(h.ID, false)

	pruned := e.PruneIneffective(0.5)
	if pruned != 1 {
		t.Fatalf("expected 1 pruned, got %d", pruned)
	}
}

func TestIsComplex(t *testing.T) {
	e := NewDualProcessEngine()

	tests := []struct {
		input    string
		expected bool
	}{
		{"simple query", false},
		{"what if we do this? and what about that?", true},              // multiple questions
		{"if the condition is true then do something", true},            // conditional
		{"do not do this", true},                                        // negation
		{string(make([]byte, 201)), true},                               // long input
	}

	for _, tc := range tests {
		got := e.IsComplex(tc.input)
		if got != tc.expected {
			t.Errorf("IsComplex(%q) = %v, want %v", tc.input[:min(len(tc.input), 40)], got, tc.expected)
		}
	}
}

func TestShouldEscalateComplexInput(t *testing.T) {
	e := NewDualProcessEngine()
	result := &System1Result{Confidence: 0.95}
	if !e.ShouldEscalateToSystem2("if this then that but not the other", result) {
		t.Fatal("expected escalation for complex input even with high confidence")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
