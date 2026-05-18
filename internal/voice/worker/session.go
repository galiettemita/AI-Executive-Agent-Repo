package worker

import (
	"fmt"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/determinism"
)

// TranscriptTurn represents a single turn in a voice conversation.
type TranscriptTurn struct {
	Speaker   string
	Text      string
	Timestamp time.Time
}

// VoiceSession represents an active or completed voice session.
type VoiceSession struct {
	ID              string
	WorkspaceID     string
	UserID          string
	Status          string // active, ended, error
	StartedAt       time.Time
	EndedAt         time.Time
	DurationMs      int64
	TranscriptTurns []TranscriptTurn
}

// VoiceSessionService manages voice sessions.
type VoiceSessionService struct {
	mu       sync.Mutex
	sessions map[string]*VoiceSession
}

// NewVoiceSessionService creates a new VoiceSessionService.
func NewVoiceSessionService() *VoiceSessionService {
	return &VoiceSessionService{
		sessions: make(map[string]*VoiceSession),
	}
}

// CreateSession starts a new voice session.
func (s *VoiceSessionService) CreateSession(workspaceID, userID string) (*VoiceSession, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID must not be empty")
	}
	if userID == "" {
		return nil, fmt.Errorf("userID must not be empty")
	}

	id, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	session := &VoiceSession{
		ID:              id.String(),
		WorkspaceID:     workspaceID,
		UserID:          userID,
		Status:          "active",
		StartedAt:       time.Now(),
		TranscriptTurns: []TranscriptTurn{},
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return session, nil
}

// EndSession marks a session as ended and records duration.
func (s *VoiceSessionService) EndSession(sessionID string) (*VoiceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	if session.Status != "active" {
		return nil, fmt.Errorf("session %s is already %s", sessionID, session.Status)
	}

	session.Status = "ended"
	session.EndedAt = time.Now()
	session.DurationMs = session.EndedAt.Sub(session.StartedAt).Milliseconds()

	return session, nil
}

// AddTranscriptTurn appends a transcript turn to an active session.
func (s *VoiceSessionService) AddTranscriptTurn(sessionID string, turn TranscriptTurn) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %s not found", sessionID)
	}
	if session.Status != "active" {
		return fmt.Errorf("cannot add turns to %s session", session.Status)
	}

	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now()
	}

	session.TranscriptTurns = append(session.TranscriptTurns, turn)
	return nil
}

// GetSession retrieves a session by ID.
func (s *VoiceSessionService) GetSession(sessionID string) (*VoiceSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}
	return session, nil
}

// ListSessions returns all sessions for a workspace.
func (s *VoiceSessionService) ListSessions(workspaceID string) []VoiceSession {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []VoiceSession
	for _, session := range s.sessions {
		if session.WorkspaceID == workspaceID {
			result = append(result, *session)
		}
	}
	return result
}
