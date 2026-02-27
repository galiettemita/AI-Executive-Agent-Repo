package learning

import "testing"

func TestLearningLifecycle(t *testing.T) {
	s := NewService()
	s.UpsertConfig("ws_1", Config{MaxActiveLessons: 10, AutoApplyLessons: false})
	feedback := s.AddFeedback(Feedback{
		WorkspaceID:  "ws_1",
		FeedbackType: "positive",
		Content:      "Great reasoning quality",
	})
	if feedback.ID == "" {
		t.Fatalf("expected feedback id")
	}
	lessons := s.ListLessons("ws_1")
	if len(lessons) != 1 {
		t.Fatalf("expected one lesson")
	}
	if _, ok := s.ConfirmLesson(lessons[0].ID); !ok {
		t.Fatalf("expected lesson confirmation")
	}
	if _, ok := s.RetireLesson(lessons[0].ID); !ok {
		t.Fatalf("expected lesson retirement")
	}
}
