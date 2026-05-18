package cognitive

import (
	"testing"
	"time"
)

func TestRecordSignalAndInferPreferences(t *testing.T) {
	t.Parallel()

	svc := NewImplicitPreferenceService()

	svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "accept",
		Context: "format", Value: "markdown", Timestamp: time.Now(),
	})
	svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "accept",
		Context: "format", Value: "markdown", Timestamp: time.Now(),
	})
	svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "dismiss",
		Context: "format", Value: "plaintext", Timestamp: time.Now(),
	})

	prefs := svc.InferPreferences("ws1", "u1")
	if len(prefs) == 0 {
		t.Fatal("expected inferred preferences")
	}

	// Markdown (accept) should have higher confidence than plaintext (dismiss).
	var markdownConf, plaintextConf float64
	for _, p := range prefs {
		if p.Preference == "markdown" {
			markdownConf = p.Confidence
		}
		if p.Preference == "plaintext" {
			plaintextConf = p.Confidence
		}
	}
	if markdownConf <= plaintextConf {
		t.Fatalf("expected markdown confidence > plaintext, got markdown=%f, plaintext=%f", markdownConf, plaintextConf)
	}
}

func TestInferPreferencesEmpty(t *testing.T) {
	t.Parallel()

	svc := NewImplicitPreferenceService()
	prefs := svc.InferPreferences("ws1", "unknown_user")
	if prefs != nil {
		t.Fatal("expected nil preferences for unknown user")
	}
}

func TestGetPreferenceByCategory(t *testing.T) {
	t.Parallel()

	svc := NewImplicitPreferenceService()
	svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "click",
		Context: "theme", Value: "dark", Timestamp: time.Now(),
	})

	pref, err := svc.GetPreference("ws1", "u1", "theme")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pref == nil {
		t.Fatal("expected preference result")
	}
	if pref.Preference != "dark" {
		t.Fatalf("expected dark theme preference, got %s", pref.Preference)
	}
}

func TestGetPreferenceNonexistentCategory(t *testing.T) {
	t.Parallel()

	svc := NewImplicitPreferenceService()
	svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "click",
		Context: "theme", Value: "dark", Timestamp: time.Now(),
	})

	pref, err := svc.GetPreference("ws1", "u1", "nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if pref != nil {
		t.Fatal("expected nil for non-existent category")
	}
}

func TestRecordSignalAutoTimestamp(t *testing.T) {
	t.Parallel()

	svc := NewImplicitPreferenceService()
	err := svc.RecordSignal(BehaviorSignal{
		WorkspaceID: "ws1", UserID: "u1", SignalType: "click",
		Context: "btn", Value: "save",
		// Timestamp is zero, should be auto-set.
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	prefs := svc.InferPreferences("ws1", "u1")
	if len(prefs) != 1 {
		t.Fatalf("expected 1 preference, got %d", len(prefs))
	}
}
