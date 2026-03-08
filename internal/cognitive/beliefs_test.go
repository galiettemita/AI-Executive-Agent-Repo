package cognitive

import (
	"testing"
)

func TestRegisterBeliefValidatesInputs(t *testing.T) {
	t.Parallel()

	svc := NewBeliefService()

	// Valid registration.
	b, err := svc.RegisterBelief("ws1", "topic_a", 0.7)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if b.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if b.Prior != 0.7 {
		t.Fatalf("expected prior 0.7, got %f", b.Prior)
	}
	if b.Posterior != 0.7 {
		t.Fatalf("expected posterior to equal prior initially, got %f", b.Posterior)
	}

	// Invalid prior.
	_, err = svc.RegisterBelief("ws1", "topic_b", 1.5)
	if err == nil {
		t.Fatal("expected error for prior > 1")
	}
	_, err = svc.RegisterBelief("ws1", "topic_b", -0.1)
	if err == nil {
		t.Fatal("expected error for prior < 0")
	}

	// Empty topic.
	_, err = svc.RegisterBelief("ws1", "", 0.5)
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
}

func TestUpdateBeliefBayesianUpdate(t *testing.T) {
	t.Parallel()

	svc := NewBeliefService()
	b, _ := svc.RegisterBelief("ws1", "hypothesis", 0.5)

	// Positive observation should increase posterior.
	updated, err := svc.UpdateBelief(b.ID, true, 0.8)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Posterior <= 0.5 {
		t.Fatalf("expected posterior > 0.5 after positive observation, got %f", updated.Posterior)
	}
	if updated.ObservationCount != 1 {
		t.Fatalf("expected observation count 1, got %d", updated.ObservationCount)
	}

	// Negative observation should decrease posterior.
	prevPosterior := updated.Posterior
	updated, err = svc.UpdateBelief(b.ID, false, 0.8)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if updated.Posterior >= prevPosterior {
		t.Fatalf("expected posterior to decrease after negative observation, got %f (was %f)", updated.Posterior, prevPosterior)
	}

	// Invalid strength.
	_, err = svc.UpdateBelief(b.ID, true, 1.5)
	if err == nil {
		t.Fatal("expected error for strength > 1")
	}

	// Non-existent belief.
	_, err = svc.UpdateBelief("nonexistent", true, 0.5)
	if err == nil {
		t.Fatal("expected error for non-existent belief")
	}
}

func TestGetBeliefByWorkspaceAndTopic(t *testing.T) {
	t.Parallel()

	svc := NewBeliefService()
	svc.RegisterBelief("ws1", "topic_x", 0.6)

	found, err := svc.GetBelief("ws1", "topic_x")
	if err != nil {
		t.Fatalf("expected to find belief, got %v", err)
	}
	if found.Topic != "topic_x" {
		t.Fatalf("expected topic_x, got %s", found.Topic)
	}

	_, err = svc.GetBelief("ws1", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent topic")
	}
}

func TestDecayBeliefsReducesConfidence(t *testing.T) {
	t.Parallel()

	svc := NewBeliefService()
	b, _ := svc.RegisterBelief("ws1", "decay_topic", 0.8)
	// Give it some observations to build confidence.
	svc.UpdateBelief(b.ID, true, 0.9)
	svc.UpdateBelief(b.ID, true, 0.9)

	beforeConfidence := b.Confidence
	beforePosterior := b.Posterior

	decayed := svc.DecayBeliefs("ws1", 0.1)
	if decayed != 1 {
		t.Fatalf("expected 1 decayed belief, got %d", decayed)
	}
	if b.Confidence >= beforeConfidence {
		t.Fatalf("expected confidence to decrease after decay")
	}
	// Posterior should move toward 0.5.
	distBefore := beforePosterior - 0.5
	distAfter := b.Posterior - 0.5
	if distBefore > 0 && distAfter >= distBefore {
		t.Fatalf("expected posterior to move toward 0.5 after decay")
	}
}

func TestMostAndLeastConfident(t *testing.T) {
	t.Parallel()

	svc := NewBeliefService()
	b1, _ := svc.RegisterBelief("ws1", "low", 0.5)
	b2, _ := svc.RegisterBelief("ws1", "high", 0.5)
	svc.RegisterBelief("ws2", "other_ws", 0.5)

	// Build confidence on b2 with multiple observations.
	for i := 0; i < 10; i++ {
		svc.UpdateBelief(b2.ID, true, 0.9)
	}
	_ = b1 // b1 has no observations, lower confidence.

	most := svc.MostConfident("ws1", 1)
	if len(most) != 1 {
		t.Fatalf("expected 1 result, got %d", len(most))
	}
	if most[0].Topic != "high" {
		t.Fatalf("expected 'high' topic first, got %s", most[0].Topic)
	}

	least := svc.LeastConfident("ws1", 1)
	if len(least) != 1 {
		t.Fatalf("expected 1 result, got %d", len(least))
	}
	if least[0].Topic != "low" {
		t.Fatalf("expected 'low' topic first, got %s", least[0].Topic)
	}
}
