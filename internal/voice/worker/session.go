package worker

import (
	"context"
	"time"
)

// TranscriptTurn represents a single turn in a voice conversation.
type TranscriptTurn struct {
	Speaker   string    `json:"speaker"`
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

// VoiceSession represents an active or completed voice session.
type VoiceSession struct {
	ID              string           `json:"id"`
	WorkspaceID     string           `json:"workspace_id"`
	UserID          string           `json:"user_id"`
	Status          string           `json:"status"` // active, ended, error
	StartedAt       time.Time        `json:"started_at"`
	EndedAt         time.Time        `json:"ended_at"`
	DurationMs      int64            `json:"duration_ms"`
	TranscriptTurns []TranscriptTurn `json:"transcript_turns"`
}

// VoiceSessionService provides backward-compatible methods over a SessionStore.
// New code should use SessionStore directly.
type VoiceSessionService struct {
	store SessionStore
}

// NewVoiceSessionService creates a VoiceSessionService backed by the given store.
// If store is nil, an InMemorySessionStore is used.
func NewVoiceSessionService(store ...SessionStore) *VoiceSessionService {
	var s SessionStore
	if len(store) > 0 && store[0] != nil {
		s = store[0]
	} else {
		s = NewInMemorySessionStore()
	}
	return &VoiceSessionService{store: s}
}

// CreateSession starts a new voice session.
func (s *VoiceSessionService) CreateSession(workspaceID, userID string) (*VoiceSession, error) {
	return s.store.Create(context.Background(), workspaceID, userID)
}

// EndSession marks a session as ended and records duration.
func (s *VoiceSessionService) EndSession(sessionID string) (*VoiceSession, error) {
	return s.store.End(context.Background(), sessionID)
}

// AddTranscriptTurn appends a transcript turn to an active session.
func (s *VoiceSessionService) AddTranscriptTurn(sessionID string, turn TranscriptTurn) error {
	return s.store.AddTurn(context.Background(), sessionID, turn)
}

// GetSession retrieves a session by ID.
func (s *VoiceSessionService) GetSession(sessionID string) (*VoiceSession, error) {
	return s.store.Get(context.Background(), sessionID)
}

// ListSessions returns all sessions for a workspace.
func (s *VoiceSessionService) ListSessions(workspaceID string) []VoiceSession {
	sessions, _ := s.store.List(context.Background(), workspaceID)
	return sessions
}
