package capture

import "testing"

func TestCaptureLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.RecordDailyLog("ws_1", "2026-02-27", "win:closed strict gate")
	s.RecordDailyLog("ws_1", "2026-02-27", "blocker:pending legal approval")
	s.RecordDailyLog("ws_1", "2026-02-27", "next:ship phase 2")
	first := s.CompleteDailyCapture("ws_1", "2026-02-27")
	second := s.CompleteDailyCapture("ws_1", "2026-02-27")
	if first.Summary != second.Summary {
		t.Fatalf("expected idempotent capture summary, got first=%q second=%q", first.Summary, second.Summary)
	}

	entries := s.List("ws_1")
	if len(entries) != 1 {
		t.Fatalf("expected one daily capture")
	}
	if _, ok := s.Get("ws_1", "2026-02-27"); !ok {
		t.Fatalf("expected date lookup success")
	}
	if len(entries[0].Wins) != 1 || len(entries[0].Blockers) != 1 || len(entries[0].NextActions) != 1 {
		t.Fatalf("unexpected classified capture fields: %+v", entries[0])
	}
}

func TestMorningBriefingFromCapture(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.Add(DailyCapture{
		WorkspaceID: "ws_2",
		CaptureDate: "2026-02-28",
		Summary:     "Review complete",
		Wins:        []string{"release passed"},
		Blockers:    []string{"none"},
		NextActions: []string{"start step 20"},
	})

	briefing := s.GenerateMorningBriefing("ws_2", "2026-02-28")
	if briefing.WorkspaceID != "ws_2" {
		t.Fatalf("unexpected workspace in briefing: %+v", briefing)
	}
	if briefing.Headline == "" || len(briefing.Priorities) == 0 {
		t.Fatalf("expected populated morning briefing: %+v", briefing)
	}
}
