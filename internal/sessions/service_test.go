package sessions

import "testing"

func TestSessionLifecycle(t *testing.T) {
	s := NewService()

	seed := s.EnsureSession("session_1", "ws_1", "user_1")
	if seed.Status != "active" {
		t.Fatalf("expected active session, got %q", seed.Status)
	}

	s.UpsertSession(Session{
		ID:          "session_2",
		WorkspaceID: "ws_1",
		UserID:      "user_2",
		Status:      "active",
		TurnCount:   3,
		LastIntent:  "follow_up",
	})
	s.UpsertSession(Session{
		ID:          "session_3",
		WorkspaceID: "ws_1",
		UserID:      "user_3",
		Status:      "expired",
		TurnCount:   8,
		LastIntent:  "closed",
	})

	active := s.ListActive("ws_1")
	if len(active) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(active))
	}
	if active[0].ID != "session_1" || active[1].ID != "session_2" {
		t.Fatalf("unexpected active ordering: %#v", active)
	}

	s.SetEntities("session_1", []Entity{
		{Key: "project", Value: "BREVIO", Confidence: 0.99},
		{Key: "owner", Value: "Alex", Confidence: 0.96},
	})
	entities := s.GetEntities("session_1")
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if entities[0].Key != "owner" || entities[1].Key != "project" {
		t.Fatalf("unexpected entity ordering: %#v", entities)
	}
}
