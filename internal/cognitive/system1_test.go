package cognitive

import (
	"testing"
)

func TestLearnHeuristicValidation(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()

	h, err := svc.LearnHeuristic("greet user", "Hello! How can I help?", "experience")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if h.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if h.Confidence != 0.5 {
		t.Fatalf("expected initial confidence 0.5, got %f", h.Confidence)
	}

	_, err = svc.LearnHeuristic("", "response", "src")
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}

	_, err = svc.LearnHeuristic("pattern", "", "src")
	if err == nil {
		t.Fatal("expected error for empty response")
	}
}

func TestMatchHeuristicExact(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()
	svc.LearnHeuristic("hello", "Hi there!", "test")

	matched, found := svc.MatchHeuristic("hello")
	if !found {
		t.Fatal("expected exact match")
	}
	if matched.Response != "Hi there!" {
		t.Fatalf("expected 'Hi there!', got %s", matched.Response)
	}
}

func TestMatchHeuristicNoMatch(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()
	svc.LearnHeuristic("schedule meeting", "Opening calendar", "test")

	_, found := svc.MatchHeuristic("z")
	if found {
		t.Fatal("expected no match for completely unrelated input")
	}
}

func TestUpdateHeuristicConfidence(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()
	h, _ := svc.LearnHeuristic("pattern one", "response one", "test")

	initialConf := h.Confidence
	svc.UpdateHeuristic(h.ID, true)
	if h.Confidence <= initialConf {
		t.Fatalf("expected confidence to increase after success, got %f (was %f)", h.Confidence, initialConf)
	}

	afterSuccess := h.Confidence
	svc.UpdateHeuristic(h.ID, false)
	if h.Confidence >= afterSuccess {
		t.Fatalf("expected confidence to decrease after failure, got %f (was %f)", h.Confidence, afterSuccess)
	}
}

func TestPruneHeuristics(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()
	h1, _ := svc.LearnHeuristic("low confidence pattern", "resp1", "test")
	svc.LearnHeuristic("normal confidence pattern", "resp2", "test")

	// Reduce h1 confidence below threshold.
	for i := 0; i < 10; i++ {
		svc.UpdateHeuristic(h1.ID, false)
	}

	pruned := svc.PruneHeuristics(0.3)
	if pruned != 1 {
		t.Fatalf("expected 1 pruned heuristic, got %d", pruned)
	}
}

func TestSystem1DecisionHighConfidence(t *testing.T) {
	t.Parallel()

	svc := NewSystem1Service()
	h, _ := svc.LearnHeuristic("schedule meeting", "Opening calendar app", "test")

	// Pump up confidence above 0.85.
	for i := 0; i < 20; i++ {
		svc.UpdateHeuristic(h.ID, true)
	}

	result, err := svc.System1Decision("schedule meeting")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.UsedHeuristic {
		t.Fatalf("expected heuristic to be used with high confidence, confidence=%f", h.Confidence)
	}

	// Empty input should error.
	_, err = svc.System1Decision("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}
