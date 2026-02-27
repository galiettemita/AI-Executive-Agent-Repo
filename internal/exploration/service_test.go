package exploration

import "testing"

func TestExplorationLifecycle(t *testing.T) {
	s := NewService()
	recs := s.ListRecommendations("ws_1")
	if len(recs) == 0 {
		t.Fatalf("expected seeded recommendation")
	}
	decided, ok := s.DecideRecommendation(recs[0].ID, "accept")
	if !ok {
		t.Fatalf("expected recommendation decision")
	}
	if decided.Status != "accepted" {
		t.Fatalf("unexpected recommendation status: %#v", decided)
	}
}
