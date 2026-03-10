package temporal

import (
	"context"
	"testing"
)

func TestInitVoiceSessionActivity_Valid(t *testing.T) {
	a := NewActivities()
	result, err := a.InitVoiceSessionActivity(context.Background(), VoiceInitInput{
		SessionID:   "sess-12345678-abcd",
		WorkspaceID: "ws-1",
		UserID:      "u-1",
		ChannelType: "livekit",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RoomName == "" {
		t.Fatal("expected room name")
	}
	if result.Token == "" {
		t.Fatal("expected token")
	}
}

func TestInitVoiceSessionActivity_MissingFields(t *testing.T) {
	a := NewActivities()
	_, err := a.InitVoiceSessionActivity(context.Background(), VoiceInitInput{})
	if err == nil {
		t.Fatal("expected error for missing fields")
	}
}

func TestExtractVoiceTasksActivity_WithTasks(t *testing.T) {
	a := NewActivities()
	result, err := a.ExtractVoiceTasksActivity(context.Background(), VoiceTaskExtractInput{
		SessionID:   "sess-1",
		WorkspaceID: "ws-1",
		Transcript:  "Please remind me to call John tomorrow. Also schedule a meeting with the team.",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(result.Tasks))
	}
}

func TestExtractVoiceTasksActivity_Empty(t *testing.T) {
	a := NewActivities()
	result, err := a.ExtractVoiceTasksActivity(context.Background(), VoiceTaskExtractInput{
		SessionID:   "sess-1",
		WorkspaceID: "ws-1",
		Transcript:  "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(result.Tasks))
	}
}
