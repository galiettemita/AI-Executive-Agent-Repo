package capture

import "testing"

func TestCaptureLifecycle(t *testing.T) {
	s := NewService()
	s.Add(DailyCapture{
		WorkspaceID: "ws_1",
		Date:        "2026-02-27",
		Summary:     "Completed strict closure work",
	})
	entries := s.List("ws_1")
	if len(entries) != 1 {
		t.Fatalf("expected one daily capture")
	}
	if _, ok := s.Get("ws_1", "2026-02-27"); !ok {
		t.Fatalf("expected date lookup success")
	}
}
