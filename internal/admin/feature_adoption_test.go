package admin

import "testing"

func TestFeatureAdoptionTrackHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(10)
	evt, err := svc.TrackAdoption(FeatureAdoptionEvent{
		WorkspaceID: "ws1",
		UserID:      "u1",
		FeatureID:   "chat",
		Action:      "activated",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if evt.ID == "" {
		t.Fatal("expected generated ID")
	}
}

func TestFeatureAdoptionTrackMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(10)

	_, err := svc.TrackAdoption(FeatureAdoptionEvent{UserID: "u1", FeatureID: "f1"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", FeatureID: "f1"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
	_, err = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing feature_id")
	}
}

func TestFeatureAdoptionDefaultAction(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(10)
	evt, _ := svc.TrackAdoption(FeatureAdoptionEvent{
		WorkspaceID: "ws1", UserID: "u1", FeatureID: "chat",
	})
	if evt.Action != "used" {
		t.Fatalf("expected default action 'used', got %s", evt.Action)
	}
}

func TestFeatureAdoptionGetStats(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(10)
	_, _ = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u1", FeatureID: "chat"})
	_, _ = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u2", FeatureID: "chat"})
	_, _ = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u1", FeatureID: "search"})

	stats := svc.GetAdoptionStats()
	if len(stats) != 2 {
		t.Fatalf("expected 2 features, got %d", len(stats))
	}

	// stats sorted by feature_id: chat, search
	if stats[0].FeatureID != "chat" {
		t.Fatalf("expected chat first, got %s", stats[0].FeatureID)
	}
	if stats[0].TotalUsers != 2 {
		t.Fatalf("expected 2 users for chat, got %d", stats[0].TotalUsers)
	}
	if stats[0].AdoptionPct != 20.0 {
		t.Fatalf("expected 20%% adoption for chat, got %f", stats[0].AdoptionPct)
	}
}

func TestFeatureAdoptionGetFeatureAdoption(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(5)
	_, _ = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u1", FeatureID: "chat"})
	_, _ = svc.TrackAdoption(FeatureAdoptionEvent{WorkspaceID: "ws1", UserID: "u1", FeatureID: "chat"}) // duplicate user

	stats := svc.GetFeatureAdoption("chat")
	if stats.TotalUsers != 1 {
		t.Fatalf("expected 1 unique user, got %d", stats.TotalUsers)
	}
	if stats.TotalEvents != 2 {
		t.Fatalf("expected 2 events, got %d", stats.TotalEvents)
	}
	if stats.AdoptionPct != 20.0 {
		t.Fatalf("expected 20%%, got %f", stats.AdoptionPct)
	}
}

func TestFeatureAdoptionGetFeatureAdoptionEmpty(t *testing.T) {
	t.Parallel()
	svc := NewFeatureAdoptionService(10)
	stats := svc.GetFeatureAdoption("nonexistent")
	if stats.TotalUsers != 0 {
		t.Fatalf("expected 0 users, got %d", stats.TotalUsers)
	}
}
