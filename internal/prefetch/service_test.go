package prefetch

import (
	"testing"
)

func TestRecordIntentSequenceAndPredictNext(t *testing.T) {
	svc := NewPrefetchService()
	svc.RecordIntentSequence("ws1", []string{"schedule", "calendar", "contacts"})

	candidates := svc.PredictNext("ws1", []string{"calendar"})
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].Pattern != "contacts" {
		t.Fatalf("expected pattern 'contacts', got %q", candidates[0].Pattern)
	}
	if candidates[0].Probability != 1.0 {
		t.Fatalf("expected probability 1.0, got %f", candidates[0].Probability)
	}
}

func TestRecordIntentSequenceTooShort(t *testing.T) {
	svc := NewPrefetchService()
	svc.RecordIntentSequence("ws1", []string{"only_one"})

	candidates := svc.PredictNext("ws1", []string{"only_one"})
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates for short sequence, got %d", len(candidates))
	}
}

func TestPredictNextNoData(t *testing.T) {
	svc := NewPrefetchService()

	candidates := svc.PredictNext("ws1", []string{"schedule"})
	if candidates != nil {
		t.Fatalf("expected nil candidates, got %v", candidates)
	}
}

func TestPredictNextEmptyIntents(t *testing.T) {
	svc := NewPrefetchService()

	candidates := svc.PredictNext("ws1", []string{})
	if candidates != nil {
		t.Fatalf("expected nil for empty intents, got %v", candidates)
	}
}

func TestCachePrefetchAndGetPrefetched(t *testing.T) {
	svc := NewPrefetchService()

	cached := svc.CachePrefetch("ws1", []PrefetchCandidate{
		{Pattern: "calendar", Probability: 0.8, PrecomputedResult: "meeting at 3pm"},
		{Pattern: "contacts", Probability: 0.2, PrecomputedResult: ""},
	})
	if cached != 1 {
		t.Fatalf("expected 1 cached, got %d", cached)
	}

	result, ok := svc.GetPrefetched("ws1", "calendar")
	if !ok {
		t.Fatal("expected cache hit for 'calendar'")
	}
	if result != "meeting at 3pm" {
		t.Fatalf("expected 'meeting at 3pm', got %q", result)
	}
}

func TestGetPrefetchedNoMatch(t *testing.T) {
	svc := NewPrefetchService()

	_, ok := svc.GetPrefetched("ws1", "anything")
	if ok {
		t.Fatal("expected no match for empty cache")
	}
}

func TestGetPrefetchedPrefixMatch(t *testing.T) {
	svc := NewPrefetchService()

	svc.CachePrefetch("ws1", []PrefetchCandidate{
		{Pattern: "schedule", Probability: 0.9, PrecomputedResult: "schedule result"},
	})

	result, ok := svc.GetPrefetched("ws1", "schedule_meeting")
	if !ok {
		t.Fatal("expected prefix match")
	}
	if result != "schedule result" {
		t.Fatalf("expected 'schedule result', got %q", result)
	}
}

func TestPredictNextWithCachedResults(t *testing.T) {
	svc := NewPrefetchService()

	svc.RecordIntentSequence("ws1", []string{"schedule", "calendar"})
	svc.CachePrefetch("ws1", []PrefetchCandidate{
		{Pattern: "calendar", Probability: 1.0, PrecomputedResult: "cached calendar data"},
	})

	candidates := svc.PredictNext("ws1", []string{"schedule"})
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if candidates[0].PrecomputedResult != "cached calendar data" {
		t.Fatalf("expected precomputed result, got %q", candidates[0].PrecomputedResult)
	}
}

func TestPredictNextMultipleCandidatesSorted(t *testing.T) {
	svc := NewPrefetchService()

	// Record sequences so "calendar" follows "schedule" 3 times, "contacts" follows 1 time
	svc.RecordIntentSequence("ws1", []string{"schedule", "calendar"})
	svc.RecordIntentSequence("ws1", []string{"schedule", "calendar"})
	svc.RecordIntentSequence("ws1", []string{"schedule", "calendar"})
	svc.RecordIntentSequence("ws1", []string{"schedule", "contacts"})

	candidates := svc.PredictNext("ws1", []string{"schedule"})
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}
	// First candidate should have higher probability
	if candidates[0].Probability <= candidates[1].Probability {
		t.Fatalf("expected candidates sorted by probability descending, got %f <= %f",
			candidates[0].Probability, candidates[1].Probability)
	}
	if candidates[0].Pattern != "calendar" {
		t.Fatalf("expected first candidate 'calendar', got %q", candidates[0].Pattern)
	}
}
