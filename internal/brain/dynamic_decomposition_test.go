package brain

import (
	"testing"
)

func TestEstimateComplexity_Simple(t *testing.T) {
	svc := NewDynamicDecompositionService()

	signals := svc.EstimateComplexity("please send an email to John")
	if signals.WordCount != 6 {
		t.Fatalf("expected 6 words, got %d", signals.WordCount)
	}
	if signals.EntityCount == 0 {
		t.Fatal("expected at least one entity (email)")
	}
	if signals.IntentCount < 1 {
		t.Fatal("expected at least 1 intent")
	}
}

func TestEstimateComplexity_Complex(t *testing.T) {
	svc := NewDynamicDecompositionService()

	request := "First, find the client's email and then schedule a meeting for next week. After that, create a document summarizing the project report and send it to the team before the deadline."
	signals := svc.EstimateComplexity(request)

	if signals.IntentCount < 3 {
		t.Fatalf("expected at least 3 intents for complex request, got %d", signals.IntentCount)
	}
	if !signals.HasTemporalConstraints {
		t.Fatal("expected temporal constraints")
	}
	if !signals.HasDependencies {
		t.Fatal("expected dependencies")
	}
	if signals.DomainCount < 2 {
		t.Fatalf("expected at least 2 domains, got %d", signals.DomainCount)
	}
}

func TestEstimateComplexity_Empty(t *testing.T) {
	svc := NewDynamicDecompositionService()

	signals := svc.EstimateComplexity("")
	if signals.WordCount != 0 {
		t.Fatalf("expected 0 words, got %d", signals.WordCount)
	}
	if signals.IntentCount != 1 {
		t.Fatalf("expected default 1 intent, got %d", signals.IntentCount)
	}
}

func TestComputeLimits_Simple(t *testing.T) {
	svc := NewDynamicDecompositionService()

	maxTasks, maxDepth := svc.ComputeLimits(ComplexitySignals{
		WordCount:   10,
		EntityCount: 1,
		IntentCount: 1,
		DomainCount: 1,
	})

	if maxTasks != 5 {
		t.Fatalf("expected 5 base max tasks, got %d", maxTasks)
	}
	if maxDepth != 3 {
		t.Fatalf("expected 3 base max depth, got %d", maxDepth)
	}
}

func TestComputeLimits_Complex(t *testing.T) {
	svc := NewDynamicDecompositionService()

	maxTasks, maxDepth := svc.ComputeLimits(ComplexitySignals{
		WordCount:              60,
		EntityCount:            5,
		IntentCount:            4,
		DomainCount:            3,
		HasTemporalConstraints: true,
		HasDependencies:        true,
	})

	// base(5) + intents(3) + domains(2) + words(2) = 12
	if maxTasks < 10 {
		t.Fatalf("expected maxTasks >= 10 for complex request, got %d", maxTasks)
	}
	// base(3) + dependencies(1) + temporal(1) = 5
	if maxDepth < 5 {
		t.Fatalf("expected maxDepth >= 5 for complex request, got %d", maxDepth)
	}
}

func TestComputeLimits_Cap(t *testing.T) {
	svc := NewDynamicDecompositionService()

	maxTasks, maxDepth := svc.ComputeLimits(ComplexitySignals{
		WordCount:              200,
		EntityCount:            20,
		IntentCount:            15,
		DomainCount:            10,
		HasTemporalConstraints: true,
		HasDependencies:        true,
	})

	if maxTasks > 20 {
		t.Fatalf("expected maxTasks capped at 20, got %d", maxTasks)
	}
	if maxDepth > 8 {
		t.Fatalf("expected maxDepth capped at 8, got %d", maxDepth)
	}
}
