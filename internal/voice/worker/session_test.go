package worker

import (
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	t.Parallel()

	svc := NewVoiceSessionService()

	session, err := svc.CreateSession("ws1", "user1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if session.ID == "" {
		t.Fatal("expected non-empty session ID")
	}
	if session.Status != "active" {
		t.Fatalf("expected active status, got %s", session.Status)
	}
	if session.WorkspaceID != "ws1" {
		t.Fatalf("expected ws1, got %s", session.WorkspaceID)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	t.Parallel()

	svc := NewVoiceSessionService()

	_, err := svc.CreateSession("", "user1")
	if err == nil {
		t.Fatal("expected error for empty workspaceID")
	}

	_, err = svc.CreateSession("ws1", "")
	if err == nil {
		t.Fatal("expected error for empty userID")
	}
}

func TestEndSession(t *testing.T) {
	t.Parallel()

	svc := NewVoiceSessionService()
	session, _ := svc.CreateSession("ws1", "user1")

	ended, err := svc.EndSession(session.ID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ended.Status != "ended" {
		t.Fatalf("expected ended status, got %s", ended.Status)
	}
	if ended.DurationMs < 0 {
		t.Fatalf("expected non-negative duration, got %d", ended.DurationMs)
	}

	// Ending again should fail.
	_, err = svc.EndSession(session.ID)
	if err == nil {
		t.Fatal("expected error when ending already ended session")
	}

	// Non-existent session.
	_, err = svc.EndSession("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestAddTranscriptTurn(t *testing.T) {
	t.Parallel()

	svc := NewVoiceSessionService()
	session, _ := svc.CreateSession("ws1", "user1")

	err := svc.AddTranscriptTurn(session.ID, TranscriptTurn{
		Speaker:   "user",
		Text:      "Hello, I need help",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got, _ := svc.GetSession(session.ID)
	if len(got.TranscriptTurns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(got.TranscriptTurns))
	}
	if got.TranscriptTurns[0].Speaker != "user" {
		t.Fatalf("expected speaker 'user', got %s", got.TranscriptTurns[0].Speaker)
	}

	// Cannot add to ended session.
	svc.EndSession(session.ID)
	err = svc.AddTranscriptTurn(session.ID, TranscriptTurn{Speaker: "agent", Text: "hi"})
	if err == nil {
		t.Fatal("expected error when adding turn to ended session")
	}
}

func TestListSessions(t *testing.T) {
	t.Parallel()

	svc := NewVoiceSessionService()
	svc.CreateSession("ws1", "user1")
	svc.CreateSession("ws1", "user2")
	svc.CreateSession("ws2", "user3")

	ws1Sessions := svc.ListSessions("ws1")
	if len(ws1Sessions) != 2 {
		t.Fatalf("expected 2 sessions for ws1, got %d", len(ws1Sessions))
	}

	ws2Sessions := svc.ListSessions("ws2")
	if len(ws2Sessions) != 1 {
		t.Fatalf("expected 1 session for ws2, got %d", len(ws2Sessions))
	}

	empty := svc.ListSessions("nonexistent")
	if len(empty) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(empty))
	}
}
