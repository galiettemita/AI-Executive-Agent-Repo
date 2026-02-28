package exploration

import "testing"

func TestExplorationLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	for i := 0; i < 3; i++ {
		s.RecordCapabilityGap("ws_1", "calendar.intelligent_scheduling")
	}
	recs := s.ListRecommendations("ws_1")
	if len(recs) == 0 {
		t.Fatalf("expected seeded recommendation")
	}
	decided, ok, err := s.DecideRecommendation(recs[0].RecommendationID, "accept")
	if !ok {
		t.Fatalf("expected recommendation decision")
	}
	if err != nil {
		t.Fatalf("unexpected decision error: %v", err)
	}
	if decided.Status != "adopted" {
		t.Fatalf("unexpected recommendation status: %#v", decided)
	}
}

func TestExplorationRejectsInvalidDecision(t *testing.T) {
	t.Parallel()

	s := NewService()
	for i := 0; i < 3; i++ {
		s.RecordCapabilityGap("ws_2", "tasks.auto_prioritization")
	}
	recs := s.ListRecommendations("ws_2")
	if len(recs) == 0 {
		t.Fatal("expected recommendation")
	}
	if _, ok, err := s.DecideRecommendation(recs[0].RecommendationID, "invalid"); err == nil || ok {
		t.Fatalf("expected invalid decision rejection, got ok=%v err=%v", ok, err)
	}
}
