package cognition

import (
	"testing"
)

func TestMonitor(t *testing.T) {
	m := NewMetacognitiveMonitor()
	cs, err := m.Monitor("ws1", 0.5, 0.1, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cs.WorkspaceID != "ws1" {
		t.Fatalf("expected ws1, got %s", cs.WorkspaceID)
	}
	if cs.CognitiveLoad < 0 || cs.CognitiveLoad > 1 {
		t.Fatalf("cognitive load out of range: %f", cs.CognitiveLoad)
	}
}

func TestMonitorEmptyWorkspace(t *testing.T) {
	m := NewMetacognitiveMonitor()
	_, err := m.Monitor("", 0.5, 0.1, 100)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
}

func TestMonitorAlertState(t *testing.T) {
	m := NewMetacognitiveMonitor()
	cs, _ := m.Monitor("ws1", 0.9, 0.6, 5000)
	if cs.State != "alert" {
		t.Fatalf("expected alert state, got %s", cs.State)
	}
}

func TestMonitorStableState(t *testing.T) {
	m := NewMetacognitiveMonitor()
	cs, _ := m.Monitor("ws1", 0.1, 0.05, 50)
	if cs.State != "stable" {
		t.Fatalf("expected stable state, got %s", cs.State)
	}
}

func TestShouldReflect(t *testing.T) {
	m := NewMetacognitiveMonitor()
	_, _ = m.Monitor("ws1", 0.9, 0.5, 5000)
	if !m.ShouldReflect("ws1") {
		t.Fatal("expected reflection needed")
	}
}

func TestShouldNotReflect(t *testing.T) {
	m := NewMetacognitiveMonitor()
	_, _ = m.Monitor("ws1", 0.1, 0.05, 50)
	if m.ShouldReflect("ws1") {
		t.Fatal("did not expect reflection")
	}
}

func TestRecordObservation(t *testing.T) {
	m := NewMetacognitiveMonitor()
	_, _ = m.Monitor("ws1", 0.5, 0.1, 100)
	err := m.RecordObservation("ws1", "latency spike detected")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cs, _ := m.GetState("ws1")
	if len(cs.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(cs.Observations))
	}
}

func TestRecordObservationNoState(t *testing.T) {
	m := NewMetacognitiveMonitor()
	err := m.RecordObservation("ws_unknown", "test")
	if err == nil {
		t.Fatal("expected error for unknown workspace")
	}
}

func TestAdjustStrategy(t *testing.T) {
	m := NewMetacognitiveMonitor()
	_, _ = m.Monitor("ws1", 0.9, 0.6, 5000) // alert state

	adj, err := m.AdjustStrategy("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adj.Action != "reduce_complexity" {
		t.Fatalf("expected reduce_complexity, got %s", adj.Action)
	}
}
