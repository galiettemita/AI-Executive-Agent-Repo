package cognition

import (
	"testing"
)

func TestCreateBelief(t *testing.T) {
	e := NewBayesianEngine()
	b, err := e.CreateBelief("ws1", "it will rain", 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if b.Posterior != 0.5 {
		t.Fatalf("expected posterior 0.5, got %f", b.Posterior)
	}
}

func TestCreateBeliefValidation(t *testing.T) {
	e := NewBayesianEngine()
	_, err := e.CreateBelief("", "test", 0.5)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	_, err = e.CreateBelief("ws1", "", 0.5)
	if err == nil {
		t.Fatal("expected error for empty hypothesis")
	}
	_, err = e.CreateBelief("ws1", "test", 1.5)
	if err == nil {
		t.Fatal("expected error for prior > 1")
	}
}

func TestUpdateBelief(t *testing.T) {
	e := NewBayesianEngine()
	b, _ := e.CreateBelief("ws1", "hypothesis", 0.5)

	err := e.UpdateBelief(b.ID, "supporting evidence", 0.9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := e.GetBelief("ws1", "hypothesis")
	if updated.Posterior <= 0.5 {
		t.Fatalf("expected posterior to increase with supporting evidence, got %f", updated.Posterior)
	}
	if updated.ObservationCount != 1 {
		t.Fatalf("expected 1 observation, got %d", updated.ObservationCount)
	}
}

func TestUpdateBeliefContraEvidence(t *testing.T) {
	e := NewBayesianEngine()
	b, _ := e.CreateBelief("ws1", "hypothesis", 0.7)

	_ = e.UpdateBelief(b.ID, "contrary evidence", 0.1)

	updated, _ := e.GetBelief("ws1", "hypothesis")
	if updated.Posterior >= 0.7 {
		t.Fatalf("expected posterior to decrease with contrary evidence, got %f", updated.Posterior)
	}
}

func TestUpdateBeliefValidation(t *testing.T) {
	e := NewBayesianEngine()
	err := e.UpdateBelief("nonexistent", "evidence", 0.5)
	if err == nil {
		t.Fatal("expected error for nonexistent belief")
	}

	b, _ := e.CreateBelief("ws1", "test", 0.5)
	err = e.UpdateBelief(b.ID, "", 0.5)
	if err == nil {
		t.Fatal("expected error for empty evidence")
	}
	err = e.UpdateBelief(b.ID, "ev", 1.5)
	if err == nil {
		t.Fatal("expected error for likelihood > 1")
	}
}

func TestGetStrongestBeliefs(t *testing.T) {
	e := NewBayesianEngine()
	_, _ = e.CreateBelief("ws1", "weak", 0.2)
	b2, _ := e.CreateBelief("ws1", "strong", 0.8)
	_ = e.UpdateBelief(b2.ID, "more evidence", 0.9)

	strongest := e.GetStrongestBeliefs("ws1", 1)
	if len(strongest) != 1 {
		t.Fatalf("expected 1 result, got %d", len(strongest))
	}
	if strongest[0].Hypothesis != "strong" {
		t.Fatalf("expected 'strong' hypothesis first, got %s", strongest[0].Hypothesis)
	}
}

func TestGetStrongestBeliefsLimit(t *testing.T) {
	e := NewBayesianEngine()
	_, _ = e.CreateBelief("ws1", "a", 0.3)
	_, _ = e.CreateBelief("ws1", "b", 0.5)
	_, _ = e.CreateBelief("ws1", "c", 0.7)

	result := e.GetStrongestBeliefs("ws1", 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 results, got %d", len(result))
	}
}

func TestDecayBeliefs(t *testing.T) {
	e := NewBayesianEngine()
	b, _ := e.CreateBelief("ws1", "ephemeral", 0.8)

	decayed := e.DecayBeliefs("ws1", 0.5)
	if decayed != 1 {
		t.Fatalf("expected 1 decayed, got %d", decayed)
	}

	updated, _ := e.GetBelief("ws1", "ephemeral")
	if updated.Posterior >= 0.8 {
		t.Fatalf("expected posterior to decrease, got %f", updated.Posterior)
	}
	_ = b
}

func TestDecayBeliefsSkipsWellObserved(t *testing.T) {
	e := NewBayesianEngine()
	b, _ := e.CreateBelief("ws1", "observed", 0.8)
	_ = e.UpdateBelief(b.ID, "e1", 0.9)
	_ = e.UpdateBelief(b.ID, "e2", 0.9)
	_ = e.UpdateBelief(b.ID, "e3", 0.9)

	decayed := e.DecayBeliefs("ws1", 0.5)
	if decayed != 0 {
		t.Fatalf("expected 0 decayed for well-observed belief, got %d", decayed)
	}
}

func TestMultipleUpdatesConverge(t *testing.T) {
	e := NewBayesianEngine()
	b, _ := e.CreateBelief("ws1", "converging", 0.5)

	for i := 0; i < 10; i++ {
		_ = e.UpdateBelief(b.ID, "supporting", 0.9)
	}

	updated, _ := e.GetBelief("ws1", "converging")
	if updated.Posterior < 0.9 {
		t.Fatalf("expected posterior near 1.0 after many supporting updates, got %f", updated.Posterior)
	}
}
