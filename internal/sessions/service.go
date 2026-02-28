package sessions

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Session struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	ConversationID string    `json:"conversation_id"`
	WorkspaceID    string    `json:"workspace_id"`
	UserID         string    `json:"user_id"`
	Status         string    `json:"status"`
	TurnCount      int       `json:"turn_count"`
	LastIntent     string    `json:"last_intent"`
	ActiveIntent   string    `json:"active_intent"`
	LastSeenAt     time.Time `json:"last_seen_at"`
}

type Entity struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

type Intent struct {
	IntentKey  string    `json:"intent_key"`
	Confidence float64   `json:"confidence"`
	Status     string    `json:"status"`
	ObservedAt time.Time `json:"observed_at"`
}

type Service struct {
	mu       sync.RWMutex
	sessions map[string]Session
	entities map[string][]Entity
	intents  map[string][]Intent
}

func NewService() *Service {
	return &Service{
		sessions: map[string]Session{},
		entities: map[string][]Entity{},
		intents:  map[string][]Intent{},
	}
}

func (s *Service) EnsureSession(sessionID, workspaceID, userID string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID = normalizeSessionID(sessionID)
	workspaceID = normalizeWorkspaceID(workspaceID)
	userID = normalizeUserID(userID)
	if existing, ok := s.sessions[sessionID]; ok {
		existing.LastSeenAt = time.Now().UTC()
		s.sessions[sessionID] = existing
		return existing
	}

	session := Session{
		ID:             sessionID,
		SessionID:      sessionID,
		ConversationID: "conversation_" + sessionID,
		WorkspaceID:    workspaceID,
		UserID:         userID,
		Status:         "active",
		TurnCount:      0,
		LastIntent:     "unknown",
		ActiveIntent:   "unknown",
		LastSeenAt:     time.Now().UTC(),
	}
	s.sessions[sessionID] = session
	return session
}

func (s *Service) UpsertSession(session Session) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ID = normalizeSessionID(session.ID)
	if strings.TrimSpace(session.SessionID) == "" {
		session.SessionID = session.ID
	}
	if strings.TrimSpace(session.ConversationID) == "" {
		session.ConversationID = "conversation_" + session.SessionID
	}
	session.WorkspaceID = normalizeWorkspaceID(session.WorkspaceID)
	session.UserID = normalizeUserID(session.UserID)
	if strings.TrimSpace(session.Status) == "" {
		session.Status = "active"
	}
	if strings.TrimSpace(session.LastIntent) == "" {
		session.LastIntent = "unknown"
	}
	if strings.TrimSpace(session.ActiveIntent) == "" {
		session.ActiveIntent = session.LastIntent
	}
	if session.LastSeenAt.IsZero() {
		session.LastSeenAt = time.Now().UTC()
	}
	s.sessions[session.ID] = session
	return session
}

func (s *Service) GetSession(sessionID string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID = normalizeSessionID(sessionID)
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *Service) ListActive(workspaceID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		if session.WorkspaceID != workspaceID {
			continue
		}
		if session.Status != "active" {
			continue
		}
		out = append(out, session)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastSeenAt.Equal(out[j].LastSeenAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})
	return out
}

func (s *Service) SetEntities(sessionID string, entities []Entity) []Entity {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID = normalizeSessionID(sessionID)
	out := make([]Entity, len(entities))
	copy(out, entities)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key == out[j].Key {
			return out[i].Value < out[j].Value
		}
		return out[i].Key < out[j].Key
	})
	s.entities[sessionID] = out
	return out
}

func (s *Service) GetEntities(sessionID string) []Entity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessionID = normalizeSessionID(sessionID)
	out := make([]Entity, len(s.entities[sessionID]))
	copy(out, s.entities[sessionID])
	return out
}

func (s *Service) UpsertIntent(sessionID, intentKey string, confidence float64) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID = normalizeSessionID(sessionID)
	session := s.sessions[sessionID]
	if session.ID == "" {
		session = Session{
			ID:             sessionID,
			SessionID:      sessionID,
			ConversationID: "conversation_" + sessionID,
			WorkspaceID:    "default",
			UserID:         "user_unknown",
			Status:         "active",
			LastIntent:     "unknown",
			ActiveIntent:   "unknown",
		}
	}
	intentKey = normalizeIntent(intentKey)
	if confidence <= 0 {
		confidence = 0.5
	}

	session.TurnCount++
	if session.ActiveIntent != intentKey {
		session.LastIntent = session.ActiveIntent
	}
	session.ActiveIntent = intentKey
	session.LastSeenAt = time.Now().UTC()
	s.sessions[sessionID] = session

	s.intents[sessionID] = append(s.intents[sessionID], Intent{
		IntentKey:  intentKey,
		Confidence: confidence,
		Status:     "active",
		ObservedAt: time.Now().UTC(),
	})

	return session
}

func (s *Service) ListIntents(sessionID string) []Intent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID = normalizeSessionID(sessionID)
	out := make([]Intent, len(s.intents[sessionID]))
	copy(out, s.intents[sessionID])
	sort.Slice(out, func(i, j int) bool {
		if out[i].ObservedAt.Equal(out[j].ObservedAt) {
			return out[i].IntentKey < out[j].IntentKey
		}
		return out[i].ObservedAt.Before(out[j].ObservedAt)
	})
	return out
}

func (s *Service) SessionContext(sessionID string) (map[string]any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessionID = normalizeSessionID(sessionID)
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}

	entityStrings := make([]string, 0, len(s.entities[sessionID]))
	for _, entity := range s.entities[sessionID] {
		entityStrings = append(entityStrings, fmt.Sprintf("%s:%s", entity.Key, entity.Value))
	}
	sort.Strings(entityStrings)

	return map[string]any{
		"id":              session.ID,
		"session_id":      session.SessionID,
		"conversation_id": session.ConversationID,
		"workspace_id":    session.WorkspaceID,
		"user_id":         session.UserID,
		"status":          session.Status,
		"turn_count":      session.TurnCount,
		"last_intent":     session.LastIntent,
		"active_intent":   session.ActiveIntent,
		"entities":        entityStrings,
	}, true
}

func normalizeSessionID(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "session_default"
	}
	return sessionID
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeUserID(userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "user_unknown"
	}
	return userID
}

func normalizeIntent(intent string) string {
	clean := strings.ToLower(strings.TrimSpace(intent))
	if clean == "" {
		return "unknown"
	}
	return clean
}
