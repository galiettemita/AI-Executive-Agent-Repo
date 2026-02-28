package tool_health

import "testing"

func TestToolHealthLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	score := s.UpsertScore(ToolScore{
		WorkspaceID:  "ws_1",
		ToolKey:      "calendar.create_event",
		Score:        0.42,
		FailureCount: 6,
		LatencyMS:    2200,
		ErrorRate:    0.62,
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

	events := s.ListEvents("ws_1")
	if len(events) == 0 {
		t.Fatal("expected tool health events for quarantine/recovery transitions")
	}
}

func TestToolHealthRuleTriggeredQuarantine(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertRule(QuarantineRule{
		WorkspaceID:  "ws_2",
		ToolKey:      "email.send_message",
		MinScore:     0.7,
		MaxFailures:  3,
		MaxErrorRate: 0.3,
		MaxLatencyMS: 1500,
		Enabled:      true,
	})
	score := s.UpsertScore(ToolScore{
		WorkspaceID:  "ws_2",
		ToolKey:      "email.send_message",
		Score:        0.8,
		FailureCount: 2,
		LatencyMS:    900,
		ErrorRate:    0.35,
	})
	if score.Status != "quarantined" {
		t.Fatalf("expected rule-triggered quarantine status, got %#v", score)
	}
}
