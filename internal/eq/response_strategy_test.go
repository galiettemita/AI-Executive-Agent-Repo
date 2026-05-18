package eq

import (
	"testing"
	"time"
)

func TestAddStrategy(t *testing.T) {
	svc := NewEQStrategyService()

	// Valid strategy.
	st, err := svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "frustrated",
		CommStyle:      "formal",
		TimeBucket:     "morning",
		LengthModifier: 0.8,
		FormalityLevel: 4,
		OfferHelp:      true,
		ToneDirective:  "empathetic",
		CheckInAfter:   15,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.ID == "" {
		t.Fatal("expected strategy to have an ID")
	}
	if st.DetectedState != "frustrated" {
		t.Fatalf("expected detected_state=frustrated, got %s", st.DetectedState)
	}

	// Missing detected_state.
	_, err = svc.AddStrategy(EQResponseStrategy{CommStyle: "formal", FormalityLevel: 3})
	if err == nil {
		t.Fatal("expected error for missing detected_state")
	}

	// Invalid formality level.
	_, err = svc.AddStrategy(EQResponseStrategy{DetectedState: "happy", CommStyle: "casual", FormalityLevel: 0})
	if err == nil {
		t.Fatal("expected error for invalid formality_level")
	}
}

func TestGetStrategy(t *testing.T) {
	svc := NewEQStrategyService()

	_, _ = svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "positive",
		CommStyle:      "casual",
		TimeBucket:     "afternoon",
		LengthModifier: 1.2,
		FormalityLevel: 2,
		ToneDirective:  "encouraging",
	})
	_, _ = svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "positive",
		CommStyle:      "formal",
		TimeBucket:     "morning",
		LengthModifier: 1.0,
		FormalityLevel: 4,
		ToneDirective:  "professional",
	})

	// Exact match on comm_style + time_bucket.
	st, err := svc.GetStrategy("positive", "casual", "afternoon")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.ToneDirective != "encouraging" {
		t.Fatalf("expected encouraging, got %s", st.ToneDirective)
	}

	// No strategy for unknown state.
	_, err = svc.GetStrategy("confused", "casual", "morning")
	if err == nil {
		t.Fatal("expected error for unknown state")
	}

	// Empty detected_state.
	_, err = svc.GetStrategy("", "casual", "morning")
	if err == nil {
		t.Fatal("expected error for empty detected_state")
	}
}

func TestListStrategies(t *testing.T) {
	svc := NewEQStrategyService()

	if len(svc.ListStrategies()) != 0 {
		t.Fatal("expected empty list initially")
	}

	_, _ = svc.AddStrategy(EQResponseStrategy{DetectedState: "a", CommStyle: "b", FormalityLevel: 1})
	_, _ = svc.AddStrategy(EQResponseStrategy{DetectedState: "c", CommStyle: "d", FormalityLevel: 2})

	if len(svc.ListStrategies()) != 2 {
		t.Fatalf("expected 2 strategies, got %d", len(svc.ListStrategies()))
	}
}

func TestApplyStrategy(t *testing.T) {
	svc := NewEQStrategyService()
	svc.now = func() time.Time {
		return time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC) // afternoon
	}

	_, _ = svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "frustrated",
		CommStyle:      "balanced",
		TimeBucket:     "afternoon",
		LengthModifier: 0.7,
		FormalityLevel: 3,
		OfferHelp:      true,
		ToneDirective:  "empathetic",
		CheckInAfter:   10,
	})

	result, err := svc.ApplyStrategy("frustrated", "balanced")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ToneDirective != "empathetic" {
		t.Fatalf("expected empathetic, got %s", result.ToneDirective)
	}
	if result.LengthModifier != 0.7 {
		t.Fatalf("expected 0.7, got %f", result.LengthModifier)
	}
	if !result.OfferHelp {
		t.Fatal("expected offer_help=true")
	}

	// Fallback to defaults when no strategy matches.
	result, err = svc.ApplyStrategy("unknown_state", "casual")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ToneDirective != "neutral" {
		t.Fatalf("expected neutral default, got %s", result.ToneDirective)
	}
}

func TestApplyStrategyTimeBuckets(t *testing.T) {
	svc := NewEQStrategyService()

	// Set morning time.
	svc.now = func() time.Time {
		return time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	}

	_, _ = svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "neutral",
		CommStyle:      "formal",
		TimeBucket:     "morning",
		LengthModifier: 1.0,
		FormalityLevel: 5,
		ToneDirective:  "professional",
	})
	_, _ = svc.AddStrategy(EQResponseStrategy{
		DetectedState:  "neutral",
		CommStyle:      "formal",
		TimeBucket:     "evening",
		LengthModifier: 0.9,
		FormalityLevel: 3,
		ToneDirective:  "relaxed",
	})

	result, _ := svc.ApplyStrategy("neutral", "formal")
	if result.ToneDirective != "professional" {
		t.Fatalf("expected professional for morning, got %s", result.ToneDirective)
	}

	// Switch to evening.
	svc.now = func() time.Time {
		return time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC)
	}
	result, _ = svc.ApplyStrategy("neutral", "formal")
	if result.ToneDirective != "relaxed" {
		t.Fatalf("expected relaxed for evening, got %s", result.ToneDirective)
	}
}
