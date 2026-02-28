package learning

import "testing"

func TestLearningLifecycle(t *testing.T) {
	s := NewService()
	s.UpsertConfig("ws_1", Config{MaxActiveLessons: 10, AutoApplyLessons: false})
	feedback, err := s.SubmitFeedback(Feedback{
		WorkspaceID:  "ws_1",
		FeedbackType: "positive",
		Content:      "Great reasoning quality",
	})
	if err != nil {
		t.Fatalf("submit feedback: %v", err)
	}
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

func TestLearningLessonCapReached(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertConfig("ws_cap", Config{MaxActiveLessons: 1, AutoApplyLessons: false})
	if _, err := s.SubmitFeedback(Feedback{WorkspaceID: "ws_cap", FeedbackType: "positive", Content: "first"}); err != nil {
		t.Fatalf("first feedback should pass: %v", err)
	}
	if _, err := s.SubmitFeedback(Feedback{WorkspaceID: "ws_cap", FeedbackType: "positive", Content: "second"}); err == nil {
		t.Fatal("expected lesson cap error")
	}
}

func TestBulkRetireLessons(t *testing.T) {
	t.Parallel()

	s := NewService()
	_, _ = s.SubmitFeedback(Feedback{WorkspaceID: "ws_bulk", FeedbackType: "positive", Content: "one"})
	_, _ = s.SubmitFeedback(Feedback{WorkspaceID: "ws_bulk", FeedbackType: "positive", Content: "two"})
	if retired := s.BulkRetire("ws_bulk"); retired != 2 {
		t.Fatalf("expected two retired lessons, got %d", retired)
	}
}
