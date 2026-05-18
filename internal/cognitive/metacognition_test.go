package cognitive

import (
	"testing"
)

func TestMonitorCognitiveState(t *testing.T) {
	t.Parallel()

	mm := NewMetacognitiveMonitor()

	state := mm.Monitor("ws1", 0.3, 5, 0)
	if state.WorkspaceID != "ws1" {
		t.Fatalf("expected ws1, got %s", state.WorkspaceID)
	}
	if state.CognitiveLoad < 0 || state.CognitiveLoad > 1 {
		t.Fatalf("expected load in [0,1], got %f", state.CognitiveLoad)
	}
	if state.ReasoningQuality < 0 || state.ReasoningQuality > 1 {
		t.Fatalf("expected quality in [0,1], got %f", state.ReasoningQuality)
	}
}

func TestMonitorHighLoadState(t *testing.T) {
	t.Parallel()

	mm := NewMetacognitiveMonitor()

	// High complexity should produce alert state.
	state := mm.Monitor("ws1", 1.0, 15, 5)
	if state.State != "alert" {
		t.Fatalf("expected alert state for high load, got %s", state.State)
	}
	if state.CognitiveLoad <= 0.8 {
		t.Fatalf("expected load > 0.8, got %f", state.CognitiveLoad)
	}
}

func TestShouldEscalate(t *testing.T) {
	t.Parallel()

	mm := NewMetacognitiveMonitor()

	// High load should escalate.
	highLoad := &CognitiveState{CognitiveLoad: 0.9, ReasoningQuality: 0.7, UncertaintyLevel: 0.3}
	if !mm.ShouldEscalate(highLoad) {
		t.Fatal("expected escalation for high cognitive load")
	}

	// Low quality should escalate.
	lowQuality := &CognitiveState{CognitiveLoad: 0.5, ReasoningQuality: 0.3, UncertaintyLevel: 0.3}
	if !mm.ShouldEscalate(lowQuality) {
		t.Fatal("expected escalation for low reasoning quality")
	}

	// Normal state should not escalate.
	normal := &CognitiveState{CognitiveLoad: 0.5, ReasoningQuality: 0.8, UncertaintyLevel: 0.3}
	if mm.ShouldEscalate(normal) {
		t.Fatal("expected no escalation for normal state")
	}
}

func TestSuggestStrategy(t *testing.T) {
	t.Parallel()

	mm := NewMetacognitiveMonitor()

	abort := &CognitiveState{CognitiveLoad: 0.95, ReasoningQuality: 0.2, UncertaintyLevel: 0.9}
	if mm.SuggestStrategy(abort) != "abort" {
		t.Fatalf("expected abort strategy, got %s", mm.SuggestStrategy(abort))
	}

	simplify := &CognitiveState{CognitiveLoad: 0.85, ReasoningQuality: 0.6, UncertaintyLevel: 0.5}
	if mm.SuggestStrategy(simplify) != "simplify" {
		t.Fatalf("expected simplify strategy, got %s", mm.SuggestStrategy(simplify))
	}

	proceed := &CognitiveState{CognitiveLoad: 0.3, ReasoningQuality: 0.9, UncertaintyLevel: 0.2}
	if mm.SuggestStrategy(proceed) != "proceed" {
		t.Fatalf("expected proceed strategy, got %s", mm.SuggestStrategy(proceed))
	}
}

func TestRecordObservationAndHistory(t *testing.T) {
	t.Parallel()

	mm := NewMetacognitiveMonitor()
	mm.RecordObservation("ws1", "latency spike detected")
	mm.Monitor("ws1", 0.5, 10, 1)
	mm.Monitor("ws1", 0.6, 12, 2)

	history := mm.GetCognitiveHistory("ws1", 1)
	if len(history) != 1 {
		t.Fatalf("expected 1 history entry, got %d", len(history))
	}

	allHistory := mm.GetCognitiveHistory("ws1", 10)
	if len(allHistory) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(allHistory))
	}

	noHistory := mm.GetCognitiveHistory("nonexistent", 5)
	if noHistory != nil {
		t.Fatal("expected nil history for nonexistent workspace")
	}
}
