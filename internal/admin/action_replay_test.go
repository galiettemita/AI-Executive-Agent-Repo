package admin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestActionReplayRecordHappyPath(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	entry, err := svc.RecordAction(ActionReplayEntry{
		WorkspaceID:   "ws1",
		UserID:        "u1",
		ActionType:    "tool_invoke",
		ActionPayload: json.RawMessage(`{"tool":"stripe","method":"charge"}`),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected generated ID")
	}
	if entry.ReplayStatus != "recorded" {
		t.Fatalf("expected recorded status, got %s", entry.ReplayStatus)
	}
}

func TestActionReplayRecordMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()

	_, err := svc.RecordAction(ActionReplayEntry{UserID: "u1", ActionType: "t"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.RecordAction(ActionReplayEntry{WorkspaceID: "ws1", ActionType: "t"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
	_, err = svc.RecordAction(ActionReplayEntry{WorkspaceID: "ws1", UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing action_type")
	}
}

func TestActionReplayReplay(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	entry, _ := svc.RecordAction(ActionReplayEntry{
		WorkspaceID: "ws1",
		UserID:      "u1",
		ActionType:  "tool_invoke",
	})

	replayed, err := svc.ReplayAction(entry.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if replayed.ReplayStatus != "replayed" {
		t.Fatalf("expected replayed status, got %s", replayed.ReplayStatus)
	}
}

func TestActionReplayReplayNotFound(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	_, err := svc.ReplayAction("nonexistent")
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestActionReplayDoubleReplay(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	entry, _ := svc.RecordAction(ActionReplayEntry{
		WorkspaceID: "ws1",
		UserID:      "u1",
		ActionType:  "tool_invoke",
	})
	_, _ = svc.ReplayAction(entry.ID)
	_, err := svc.ReplayAction(entry.ID)
	if err == nil {
		t.Fatal("expected error for double replay")
	}
}

func TestActionReplayGetReplayLog(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	t0 := time.Date(2025, 6, 1, 10, 0, 0, 0, time.UTC)

	svc.now = func() time.Time { return t0 }
	_, _ = svc.RecordAction(ActionReplayEntry{
		WorkspaceID: "ws1", UserID: "u1", ActionType: "a1",
	})
	svc.now = func() time.Time { return t0.Add(time.Minute) }
	_, _ = svc.RecordAction(ActionReplayEntry{
		WorkspaceID: "ws1", UserID: "u1", ActionType: "a2",
	})
	// Different workspace
	_, _ = svc.RecordAction(ActionReplayEntry{
		WorkspaceID: "ws2", UserID: "u1", ActionType: "a3",
	})

	log := svc.GetReplayLog("ws1")
	if len(log) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(log))
	}
	if log[0].ActionType != "a1" {
		t.Fatalf("expected a1 first, got %s", log[0].ActionType)
	}
}

func TestActionReplayGetReplayLogEmpty(t *testing.T) {
	t.Parallel()
	svc := NewActionReplayService()
	log := svc.GetReplayLog("ws1")
	if len(log) != 0 {
		t.Fatalf("expected empty log, got %d", len(log))
	}
}
