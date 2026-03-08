package trust

import (
	"testing"
	"time"
)

func TestCheckForDemotion_TrustBelow(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{IncidentThreshold: 0.5, DriftDays: 30})
	svc.SetLevel("ws1", "email", 3)

	event, err := svc.CheckForDemotion("ws1", "email", 0.3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected demotion event")
	}
	if event.PreviousLevel != 3 || event.NewLevel != 2 {
		t.Fatalf("expected 3->2, got %d->%d", event.PreviousLevel, event.NewLevel)
	}
	if svc.GetLevel("ws1", "email") != 2 {
		t.Fatalf("expected level 2 after demotion")
	}
}

func TestCheckForDemotion_FailureCount(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{IncidentThreshold: 0.3, DriftDays: 30})
	svc.SetLevel("ws1", "calendar", 2)

	event, err := svc.CheckForDemotion("ws1", "calendar", 0.9, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event == nil {
		t.Fatal("expected demotion event for high failure count")
	}
	if event.NewLevel != 1 {
		t.Fatalf("expected level 1, got %d", event.NewLevel)
	}
}

func TestCheckForDemotion_NoDemotion(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{IncidentThreshold: 0.3, DriftDays: 30})
	svc.SetLevel("ws1", "email", 3)

	event, err := svc.CheckForDemotion("ws1", "email", 0.8, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event != nil {
		t.Fatal("expected no demotion")
	}
}

func TestDemote(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{})
	svc.SetLevel("ws1", "crm", 2)

	event, err := svc.Demote("ws1", "crm", "manual override")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.NewLevel != 1 {
		t.Fatalf("expected level 1, got %d", event.NewLevel)
	}

	// Demote again to 0.
	event, err = svc.Demote("ws1", "crm", "second demotion")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.NewLevel != 0 {
		t.Fatalf("expected level 0, got %d", event.NewLevel)
	}

	// Cannot demote below 0.
	_, err = svc.Demote("ws1", "crm", "too low")
	if err == nil {
		t.Fatal("expected error for demotion at level 0")
	}
}

func TestGetDemotionHistory(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{IncidentThreshold: 0.5})
	svc.SetLevel("ws1", "email", 3)
	svc.SetLevel("ws2", "email", 2)

	_, _ = svc.CheckForDemotion("ws1", "email", 0.2, 0)
	_, _ = svc.CheckForDemotion("ws2", "email", 0.1, 0)

	history := svc.GetDemotionHistory("ws1")
	if len(history) != 1 {
		t.Fatalf("expected 1 event for ws1, got %d", len(history))
	}
	if history[0].WorkspaceID != "ws1" {
		t.Fatalf("expected ws1, got %s", history[0].WorkspaceID)
	}
}

func TestGetDemotionCount90d(t *testing.T) {
	svc := NewAutonomyDemotionService(DemotionConfig{IncidentThreshold: 0.5})
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	svc.SetLevel("ws1", "email", 5)

	// Create some demotions.
	_, _ = svc.Demote("ws1", "email", "r1")
	svc.SetLevel("ws1", "email", 5) // reset for next demotion
	_, _ = svc.Demote("ws1", "email", "r2")

	// Inject an old event manually.
	svc.events = append(svc.events, DemotionEvent{
		WorkspaceID: "ws1",
		Domain:      "email",
		DemotedAt:   now.AddDate(0, 0, -100),
	})

	count := svc.GetDemotionCount90d("ws1")
	if count != 2 {
		t.Fatalf("expected 2 recent demotions, got %d", count)
	}
}
