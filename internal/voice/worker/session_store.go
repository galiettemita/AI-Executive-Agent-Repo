package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/determinism"
	"github.com/redis/go-redis/v9"
)

// ErrSessionNotFound is returned when a session ID does not exist.
var ErrSessionNotFound = errors.New("voice session not found")

// ErrSessionNotActive is returned when an operation requires an active session.
var ErrSessionNotActive = errors.New("voice session is not active")

// SessionStore is the persistence interface for VoiceSession objects.
type SessionStore interface {
	// Create persists a new session. Returns the created session.
	Create(ctx context.Context, workspaceID, userID string) (*VoiceSession, error)

	// Get retrieves a session by ID.
	// Returns ErrSessionNotFound if the ID does not exist.
	Get(ctx context.Context, sessionID string) (*VoiceSession, error)

	// End marks a session as ended, sets EndedAt and DurationMs.
	// Returns ErrSessionNotFound if ID not found.
	// Returns ErrSessionNotActive if session is not in "active" state.
	End(ctx context.Context, sessionID string) (*VoiceSession, error)

	// AddTurn appends a TranscriptTurn to an active session.
	// Returns ErrSessionNotFound or ErrSessionNotActive as appropriate.
	AddTurn(ctx context.Context, sessionID string, turn TranscriptTurn) error

	// List returns all sessions for a workspace (active and ended).
	// May return an empty slice; never returns nil slice.
	List(ctx context.Context, workspaceID string) ([]VoiceSession, error)

	// Delete removes a session from the store (used for cleanup).
	Delete(ctx context.Context, sessionID string) error
}

// ---------------------------------------------------------------------------
// newSessionID generates a UUIDv7 string for session IDs.
// ---------------------------------------------------------------------------

func newSessionID() (string, error) {
	id, err := determinism.NewUUIDv7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// ---------------------------------------------------------------------------
// InMemorySessionStore — for tests and dev
// ---------------------------------------------------------------------------

// InMemorySessionStore implements SessionStore backed by an in-memory map.
type InMemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*VoiceSession
}

// NewInMemorySessionStore creates an InMemorySessionStore.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*VoiceSession),
	}
}

func (m *InMemorySessionStore) Create(_ context.Context, workspaceID, userID string) (*VoiceSession, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID must not be empty")
	}
	if userID == "" {
		return nil, fmt.Errorf("userID must not be empty")
	}

	id, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	session := &VoiceSession{
		ID:              id,
		WorkspaceID:     workspaceID,
		UserID:          userID,
		Status:          "active",
		StartedAt:       time.Now().UTC(),
		TranscriptTurns: []TranscriptTurn{},
	}

	m.mu.Lock()
	m.sessions[id] = session
	m.mu.Unlock()

	// Return a copy so callers can't mutate the internal state.
	cp := *session
	return &cp, nil
}

func (m *InMemorySessionStore) Get(_ context.Context, sessionID string) (*VoiceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	cp := *session
	cp.TranscriptTurns = make([]TranscriptTurn, len(session.TranscriptTurns))
	copy(cp.TranscriptTurns, session.TranscriptTurns)
	return &cp, nil
}

func (m *InMemorySessionStore) End(_ context.Context, sessionID string) (*VoiceSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	if session.Status != "active" {
		return nil, ErrSessionNotActive
	}

	now := time.Now().UTC()
	session.Status = "ended"
	session.EndedAt = now
	session.DurationMs = now.Sub(session.StartedAt).Milliseconds()

	cp := *session
	return &cp, nil
}

func (m *InMemorySessionStore) AddTurn(_ context.Context, sessionID string, turn TranscriptTurn) error {
	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now().UTC()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	if session.Status != "active" {
		return ErrSessionNotActive
	}

	session.TranscriptTurns = append(session.TranscriptTurns, turn)
	return nil
}

func (m *InMemorySessionStore) List(_ context.Context, workspaceID string) ([]VoiceSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]VoiceSession, 0)
	for _, session := range m.sessions {
		if session.WorkspaceID == workspaceID {
			result = append(result, *session)
		}
	}
	return result, nil
}

func (m *InMemorySessionStore) Delete(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[sessionID]; !ok {
		return ErrSessionNotFound
	}
	delete(m.sessions, sessionID)
	return nil
}

// ---------------------------------------------------------------------------
// RedisSessionStore — production store
// ---------------------------------------------------------------------------

const (
	// redisSessionTTL is how long a session persists in Redis after creation.
	redisSessionTTL = 35 * time.Minute

	// redisSessionKeyPrefix is the Redis key prefix for session objects.
	redisSessionKeyPrefix = "brevio:voice:session:"

	// redisWorkspaceIndexPrefix is the Redis set key for per-workspace session IDs.
	redisWorkspaceIndexPrefix = "brevio:voice:ws:"
)

// RedisSessionStore persists VoiceSessions in Redis with JSON encoding.
type RedisSessionStore struct {
	client redis.UniversalClient
	ttl    time.Duration
}

// NewRedisSessionStore creates a RedisSessionStore.
// client must not be nil.
func NewRedisSessionStore(client redis.UniversalClient, ttl time.Duration) (*RedisSessionStore, error) {
	if client == nil {
		return nil, fmt.Errorf("redis session store: client must not be nil")
	}
	if ttl <= 0 {
		ttl = redisSessionTTL
	}
	return &RedisSessionStore{client: client, ttl: ttl}, nil
}

func (r *RedisSessionStore) sessionKey(id string) string {
	return redisSessionKeyPrefix + id
}

func (r *RedisSessionStore) workspaceKey(wsID string) string {
	return redisWorkspaceIndexPrefix + wsID
}

func (r *RedisSessionStore) Create(ctx context.Context, workspaceID, userID string) (*VoiceSession, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID must not be empty")
	}
	if userID == "" {
		return nil, fmt.Errorf("userID must not be empty")
	}

	id, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	session := &VoiceSession{
		ID:              id,
		WorkspaceID:     workspaceID,
		UserID:          userID,
		Status:          "active",
		StartedAt:       time.Now().UTC(),
		TranscriptTurns: []TranscriptTurn{},
	}

	data, err := json.Marshal(session)
	if err != nil {
		return nil, fmt.Errorf("redis session store: marshal session: %w", err)
	}

	pipe := r.client.Pipeline()
	pipe.Set(ctx, r.sessionKey(id), data, r.ttl)
	pipe.SAdd(ctx, r.workspaceKey(workspaceID), id)
	pipe.Expire(ctx, r.workspaceKey(workspaceID), r.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("redis session store: pipeline exec: %w", err)
	}

	return session, nil
}

func (r *RedisSessionStore) Get(ctx context.Context, sessionID string) (*VoiceSession, error) {
	data, err := r.client.Get(ctx, r.sessionKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("redis session store: get %s: %w", sessionID, err)
	}

	var session VoiceSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("redis session store: unmarshal session %s: %w", sessionID, err)
	}
	return &session, nil
}

func (r *RedisSessionStore) End(ctx context.Context, sessionID string) (*VoiceSession, error) {
	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		key := r.sessionKey(sessionID)
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			data, err := tx.Get(ctx, key).Bytes()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return ErrSessionNotFound
				}
				return err
			}

			var session VoiceSession
			if err := json.Unmarshal(data, &session); err != nil {
				return err
			}
			if session.Status != "active" {
				return ErrSessionNotActive
			}

			now := time.Now().UTC()
			session.Status = "ended"
			session.EndedAt = now
			session.DurationMs = now.Sub(session.StartedAt).Milliseconds()

			updated, err := json.Marshal(session)
			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, updated, r.ttl)
				return nil
			})
			return err
		}, key)

		if err == nil {
			return r.Get(ctx, sessionID)
		}
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionNotActive) {
			return nil, err
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return nil, fmt.Errorf("redis session store: end session: %w", err)
		}
	}
	return nil, fmt.Errorf("redis session store: end session: optimistic lock failed after %d retries", maxRetries)
}

func (r *RedisSessionStore) AddTurn(ctx context.Context, sessionID string, turn TranscriptTurn) error {
	if turn.Timestamp.IsZero() {
		turn.Timestamp = time.Now().UTC()
	}

	const maxRetries = 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		key := r.sessionKey(sessionID)
		err := r.client.Watch(ctx, func(tx *redis.Tx) error {
			data, err := tx.Get(ctx, key).Bytes()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return ErrSessionNotFound
				}
				return err
			}
			var session VoiceSession
			if err := json.Unmarshal(data, &session); err != nil {
				return err
			}
			if session.Status != "active" {
				return ErrSessionNotActive
			}
			session.TranscriptTurns = append(session.TranscriptTurns, turn)
			updated, err := json.Marshal(session)
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, updated, r.ttl)
				return nil
			})
			return err
		}, key)

		if err == nil {
			return nil
		}
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionNotActive) {
			return err
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return fmt.Errorf("redis session store: add turn: %w", err)
		}
	}
	return fmt.Errorf("redis session store: add turn: optimistic lock failed after %d retries", maxRetries)
}

func (r *RedisSessionStore) List(ctx context.Context, workspaceID string) ([]VoiceSession, error) {
	ids, err := r.client.SMembers(ctx, r.workspaceKey(workspaceID)).Result()
	if err != nil {
		return []VoiceSession{}, fmt.Errorf("redis session store: list workspace %s: %w", workspaceID, err)
	}

	sessions := make([]VoiceSession, 0, len(ids))
	for _, id := range ids {
		sess, err := r.Get(ctx, id)
		if err != nil {
			if errors.Is(err, ErrSessionNotFound) {
				r.client.SRem(ctx, r.workspaceKey(workspaceID), id) //nolint:errcheck
				continue
			}
			return nil, err
		}
		sessions = append(sessions, *sess)
	}
	return sessions, nil
}

func (r *RedisSessionStore) Delete(ctx context.Context, sessionID string) error {
	sess, err := r.Get(ctx, sessionID)
	if err != nil {
		return err
	}
	pipe := r.client.Pipeline()
	pipe.Del(ctx, r.sessionKey(sessionID))
	pipe.SRem(ctx, r.workspaceKey(sess.WorkspaceID), sessionID)
	_, err = pipe.Exec(ctx)
	return err
}
