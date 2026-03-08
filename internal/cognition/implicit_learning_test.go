package cognition

import (
	"testing"
)

func TestRecordSignal(t *testing.T) {
	s := NewImplicitLearningService()
	err := s.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1",
		UserID:      "u1",
		SignalType:  "click",
		Context:     "search results",
		Value:       1.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordSignalValidation(t *testing.T) {
	s := NewImplicitLearningService()
	err := s.RecordSignal(BehaviorSignal{
		WorkspaceID: "",
		UserID:      "u1",
		SignalType:  "click",
		Context:     "test",
	})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}

	err = s.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1",
		UserID:      "u1",
		SignalType:  "invalid",
		Context:     "test",
	})
	if err == nil {
		t.Fatal("expected error for invalid signal type")
	}
}

func TestInferPreferencePositive(t *testing.T) {
	s := NewImplicitLearningService()
	for i := 0; i < 5; i++ {
		_ = s.RecordSignal(BehaviorSignal{
			WorkspaceID: "ws1",
			UserID:      "u1",
			SignalType:  "click",
			Context:     "dashboard",
			Value:       1.0,
		})
	}

	pref, err := s.InferPreference("ws1", "u1", "dashboard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pref.PreferredAction != "repeat" {
		t.Fatalf("expected 'repeat', got %s", pref.PreferredAction)
	}
	if pref.SignalCount != 5 {
		t.Fatalf("expected 5 signals, got %d", pref.SignalCount)
	}
}

func TestInferPreferenceNegative(t *testing.T) {
	s := NewImplicitLearningService()
	for i := 0; i < 5; i++ {
		_ = s.RecordSignal(BehaviorSignal{
			WorkspaceID: "ws1",
			UserID:      "u1",
			SignalType:  "skip",
			Context:     "notifications",
			Value:       1.0,
		})
	}

	pref, err := s.InferPreference("ws1", "u1", "notifications")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pref.PreferredAction != "change" {
		t.Fatalf("expected 'change', got %s", pref.PreferredAction)
	}
}

func TestInferPreferenceNoSignals(t *testing.T) {
	s := NewImplicitLearningService()
	_, err := s.InferPreference("ws1", "u1", "unknown")
	if err == nil {
		t.Fatal("expected error for no signals")
	}
}

func TestGetPreferences(t *testing.T) {
	s := NewImplicitLearningService()
	_ = s.RecordSignal(BehaviorSignal{WorkspaceID: "ws1", UserID: "u1", SignalType: "click", Context: "search"})
	_ = s.RecordSignal(BehaviorSignal{WorkspaceID: "ws1", UserID: "u1", SignalType: "skip", Context: "ads"})

	prefs := s.GetPreferences("ws1", "u1")
	if len(prefs) != 2 {
		t.Fatalf("expected 2 preferences, got %d", len(prefs))
	}
}

func TestSignalTypesValence(t *testing.T) {
	s := NewImplicitLearningService()
	types := []string{"click", "dwell", "skip", "edit", "undo", "retry"}
	for _, st := range types {
		err := s.RecordSignal(BehaviorSignal{
			WorkspaceID: "ws1",
			UserID:      "u1",
			SignalType:  st,
			Context:     "test_" + st,
		})
		if err != nil {
			t.Fatalf("unexpected error for signal type %s: %v", st, err)
		}
	}
}

func TestConfidenceIncreases(t *testing.T) {
	s := NewImplicitLearningService()
	_ = s.RecordSignal(BehaviorSignal{WorkspaceID: "ws1", UserID: "u1", SignalType: "click", Context: "feature"})
	pref1, _ := s.InferPreference("ws1", "u1", "feature")

	for i := 0; i < 10; i++ {
		_ = s.RecordSignal(BehaviorSignal{WorkspaceID: "ws1", UserID: "u1", SignalType: "click", Context: "feature"})
	}
	pref2, _ := s.InferPreference("ws1", "u1", "feature")

	if pref2.Confidence <= pref1.Confidence {
		t.Fatalf("expected confidence to increase with more signals: %f -> %f", pref1.Confidence, pref2.Confidence)
	}
}
