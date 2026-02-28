package trust

import "testing"

func TestTrustScoreFormulaAndPromotionEligibility(t *testing.T) {
	t.Parallel()

	s := NewService()
	score := s.RecalculateScore("ws_1", 25, 0, 0, 0, "A1")
	if score.Score != 1.0 {
		t.Fatalf("expected score 1.0, got %.4f", score.Score)
	}
	if !score.PromotionEligible {
		t.Fatal("expected promotion eligibility")
	}
	if len(s.ListPromotions()) == 0 {
		t.Fatal("expected promotion proposal when eligible")
	}

	nonEligible := s.RecalculateScore("ws_2", 10, 1, 1, 1, "A1")
	if nonEligible.PromotionEligible {
		t.Fatal("expected non-eligible score")
	}
}

func TestTrustPromotionDecisionLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	promotion := s.AddPromotion(Promotion{WorkspaceID: "ws_1", FromAutonomy: "A1", ToAutonomy: "A2"})
	if promotion.ID == "" {
		t.Fatalf("expected promotion id")
	}
	decided, ok := s.DecidePromotion(promotion.ID, "approve")
	if !ok {
		t.Fatalf("expected promotion decision success")
	}
	if decided.Status != "approved" {
		t.Fatalf("unexpected promotion decision: %#v", decided)
	}

	decidedAgain, ok := s.DecidePromotion(promotion.ID, "deny")
	if !ok {
		t.Fatal("expected promotion to still be retrievable")
	}
	if decidedAgain.Status != "approved" {
		t.Fatalf("expected decided promotion to remain immutable, got %#v", decidedAgain)
	}
}
