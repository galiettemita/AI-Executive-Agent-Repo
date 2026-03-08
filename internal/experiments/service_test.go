package experiments

import (
	"fmt"
	"testing"
)

func TestCreateExperiment(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()
	exp, err := svc.CreateExperiment("ws1", "button-color-test", []Variant{
		{Name: "control", Weight: 50},
		{Name: "variant-a", Weight: 50},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if exp.ID == "" {
		t.Fatal("expected non-empty experiment ID")
	}
	if exp.Status != "running" {
		t.Fatalf("expected running status, got %s", exp.Status)
	}
	if len(exp.Variants) != 2 {
		t.Fatalf("expected 2 variants, got %d", len(exp.Variants))
	}

	// Weights should be normalized.
	totalWeight := 0.0
	for _, v := range exp.Variants {
		totalWeight += v.Weight
	}
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Fatalf("expected normalized weights summing to ~1.0, got %f", totalWeight)
	}
}

func TestCreateExperimentValidation(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()

	_, err := svc.CreateExperiment("ws1", "", []Variant{{Name: "a", Weight: 1}, {Name: "b", Weight: 1}})
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	_, err = svc.CreateExperiment("ws1", "test", []Variant{{Name: "a", Weight: 1}})
	if err == nil {
		t.Fatal("expected error for fewer than 2 variants")
	}
}

func TestAssignVariantDeterministic(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()
	exp, _ := svc.CreateExperiment("ws1", "test-experiment", []Variant{
		{Name: "control", Weight: 50},
		{Name: "treatment", Weight: 50},
	})

	v1, err := svc.AssignVariant(exp.ID, "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if v1.Name == "" {
		t.Fatal("expected non-empty variant name")
	}

	// Same user should get same variant (deterministic).
	v2, _ := svc.AssignVariant(exp.ID, "user-1")
	if v1.ID != v2.ID {
		t.Fatalf("expected deterministic assignment, got %s then %s", v1.ID, v2.ID)
	}
}

func TestAssignVariantDistribution(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()
	exp, _ := svc.CreateExperiment("ws1", "dist-test", []Variant{
		{Name: "A", Weight: 50},
		{Name: "B", Weight: 50},
	})

	counts := make(map[string]int)
	for i := 0; i < 100; i++ {
		v, _ := svc.AssignVariant(exp.ID, fmt.Sprintf("user-%d", i))
		counts[v.Name]++
	}

	// With 50/50 split and 100 users, each variant should get at least some assignments.
	if counts["A"] == 0 || counts["B"] == 0 {
		t.Fatalf("expected both variants to have assignments, got A=%d B=%d", counts["A"], counts["B"])
	}
}

func TestRecordConversion(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()
	exp, _ := svc.CreateExperiment("ws1", "conv-test", []Variant{
		{Name: "A", Weight: 50},
		{Name: "B", Weight: 50},
	})

	svc.AssignVariant(exp.ID, "user-1")
	err := svc.RecordConversion(exp.ID, "user-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	results, _ := svc.GetResults(exp.ID)
	totalConversions := 0
	for _, v := range results.Variants {
		totalConversions += v.Conversions
	}
	if totalConversions != 1 {
		t.Fatalf("expected 1 total conversion, got %d", totalConversions)
	}

	// Unassigned user.
	err = svc.RecordConversion(exp.ID, "unknown-user")
	if err == nil {
		t.Fatal("expected error for unassigned user")
	}
}

func TestStopExperiment(t *testing.T) {
	t.Parallel()

	svc := NewExperimentService()
	exp, _ := svc.CreateExperiment("ws1", "stop-test", []Variant{
		{Name: "A", Weight: 50},
		{Name: "B", Weight: 50},
	})

	err := svc.StopExperiment(exp.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	results, _ := svc.GetResults(exp.ID)
	if results.Status != "stopped" {
		t.Fatalf("expected stopped status, got %s", results.Status)
	}
	if results.EndedAt == nil {
		t.Fatal("expected endedAt to be set")
	}

	// Cannot stop already stopped experiment.
	err = svc.StopExperiment(exp.ID)
	if err == nil {
		t.Fatal("expected error when stopping already stopped experiment")
	}

	// Cannot assign to stopped experiment.
	_, err = svc.AssignVariant(exp.ID, "new-user")
	if err == nil {
		t.Fatal("expected error assigning to stopped experiment")
	}
}
