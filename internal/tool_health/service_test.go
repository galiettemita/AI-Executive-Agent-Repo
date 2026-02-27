package tool_health

import "testing"

func TestToolHealthLifecycle(t *testing.T) {
	s := NewService()

	score := s.UpsertScore(ToolScore{
		WorkspaceID:  "ws_1",
		ToolKey:      "calendar.create_event",
		Score:        0.42,
		FailureCount: 6,
	})
	if score.Status != "quarantined" {
		t.Fatalf("expected quarantined status, got %#v", score)
	}

	rule := s.UpsertRule(QuarantineRule{
		WorkspaceID: "ws_1",
		ToolKey:     "calendar.create_event",
		MinScore:    0.6,
		MaxFailures: 4,
		Enabled:     true,
	})
	if rule.ID == "" {
		t.Fatalf("expected generated rule id")
	}

	rules := s.ListRules("ws_1")
	if len(rules) != 1 {
		t.Fatalf("expected 1 quarantine rule, got %d", len(rules))
	}

	override := s.ApplyOverride("ws_1", "calendar.create_event", "healthy")
	if override.Status != "healthy" {
		t.Fatalf("expected healthy override status, got %#v", override)
	}

	loaded, ok := s.GetScore("ws_1", "calendar.create_event")
	if !ok {
		t.Fatalf("expected tool score lookup success")
	}
	if loaded.Status != "healthy" {
		t.Fatalf("unexpected loaded score state: %#v", loaded)
	}
}
