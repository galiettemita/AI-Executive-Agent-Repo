package brain

import "testing"

func TestMultiIntentSingleIntent(t *testing.T) {
	t.Parallel()

	mic := NewMultiIntentClassifier()
	result := mic.Classify("send an email to Bob")
	if result.CompoundRequest {
		t.Fatal("expected single intent, not compound")
	}
	if len(result.Intents) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(result.Intents))
	}
	if result.Intents[0].Intent != "send_email" {
		t.Fatalf("expected send_email, got %s", result.Intents[0].Intent)
	}
	if !result.Intents[0].IsPrimary {
		t.Fatal("expected primary intent")
	}
}

func TestMultiIntentCompoundRequest(t *testing.T) {
	t.Parallel()

	mic := NewMultiIntentClassifier()
	result := mic.Classify("send an email to Bob and schedule a meeting")
	if !result.CompoundRequest {
		t.Fatal("expected compound request")
	}
	if len(result.Intents) < 2 {
		t.Fatalf("expected at least 2 intents, got %d", len(result.Intents))
	}
	if !result.RequiresDecomposition {
		t.Fatal("expected decomposition required")
	}
	if result.Intents[0].Intent != "send_email" {
		t.Fatalf("expected first intent send_email, got %s", result.Intents[0].Intent)
	}
}

func TestMultiIntentEmptyInput(t *testing.T) {
	t.Parallel()

	mic := NewMultiIntentClassifier()
	result := mic.Classify("")
	if result.CompoundRequest {
		t.Fatal("expected not compound for empty")
	}
	if len(result.Intents) != 0 {
		t.Fatalf("expected 0 intents, got %d", len(result.Intents))
	}
}

func TestMultiIntentDependencyChain(t *testing.T) {
	t.Parallel()

	mic := NewMultiIntentClassifier()
	result := mic.Classify("search for documents and summarize them")
	if !result.CompoundRequest {
		t.Fatal("expected compound request")
	}
	if len(result.Intents) < 2 {
		t.Fatalf("expected at least 2 intents, got %d", len(result.Intents))
	}
	// Second intent should depend on first
	if result.Intents[1].DependsOnIndex == nil {
		t.Fatal("expected second intent to depend on first")
	}
	if *result.Intents[1].DependsOnIndex != 0 {
		t.Fatalf("expected depends on index 0, got %d", *result.Intents[1].DependsOnIndex)
	}
}

func TestMultiIntentOverallConfidence(t *testing.T) {
	t.Parallel()

	mic := NewMultiIntentClassifier()
	result := mic.Classify("remind me about the deadline")
	if result.OverallConfidence <= 0 {
		t.Fatal("expected positive overall confidence")
	}
	if result.OverallConfidence > 1 {
		t.Fatalf("expected confidence <= 1, got %f", result.OverallConfidence)
	}
}
