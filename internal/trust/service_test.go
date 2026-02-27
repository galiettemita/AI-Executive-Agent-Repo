package trust

import "testing"

func TestTrustLifecycle(t *testing.T) {
	s := NewService()
	s.UpsertScore(TrustScore{
		WorkspaceID:      "ws_1",
		Score:            0.92,
		SuccessCount30d:  24,
		FailureCount30d:  0,
		OverrideCount30d: 1,
	})
	if len(s.ListScores()) != 1 {
		t.Fatalf("expected one trust score")
	}

	promotion := s.AddPromotion(Promotion{
		WorkspaceID: "ws_1",
	})
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
}
