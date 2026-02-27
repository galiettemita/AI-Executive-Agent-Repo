package sessions

import (
	"sort"
	"sync"
)

type Session struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Status      string `json:"status"`
	TurnCount   int    `json:"turn_count"`
	LastIntent  string `json:"last_intent"`
}

type Entity struct {
	Key        string  `json:"key"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

type Service struct {
	mu       sync.RWMutex
	sessions map[string]Session
	entities map[string][]Entity
}

func NewService() *Service {
	return &Service{
		sessions: map[string]Session{},
		entities: map[string][]Entity{},
	}
}

func (s *Service) EnsureSession(sessionID, workspaceID, userID string) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.sessions[sessionID]; ok {
		return existing
	}

	session := Session{
		ID:          sessionID,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Status:      "active",
		TurnCount:   0,
		LastIntent:  "unknown",
	}
	s.sessions[sessionID] = session
	return session
}

func (s *Service) UpsertSession(session Session) Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session.Status == "" {
		session.Status = "active"
	}
	s.sessions[session.ID] = session
	return session
}

func (s *Service) GetSession(sessionID string) (Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	return session, ok
}

func (s *Service) ListActive(workspaceID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) SetEntities(sessionID string, entities []Entity) []Entity {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	out := make([]Entity, len(s.entities[sessionID]))
	copy(out, s.entities[sessionID])
	return out
}
