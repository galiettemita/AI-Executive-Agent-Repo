package admin

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AdminSession represents an authenticated admin session.
type AdminSession struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
	Token       string    `json:"-"`
	TokenHash   string    `json:"token_hash"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

// AdminAuditEntry records an admin action for audit.
type AdminAuditEntry struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	UserID      string    `json:"user_id"`
	WorkspaceID string    `json:"workspace_id"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	IPAddress   string    `json:"ip_address"`
	CreatedAt   time.Time `json:"created_at"`
}

// SessionStore manages admin sessions. In production, this should be backed by
// a database; the in-memory implementation is for devtest.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*AdminSession // key: tokenHash
	auditLog []AdminAuditEntry
	secret   []byte
	now      func() time.Time
}

// NewSessionStore creates a new admin session store.
func NewSessionStore(secret []byte) *SessionStore {
	if len(secret) == 0 {
		secret = make([]byte, 32)
		_, _ = rand.Read(secret)
	}
	return &SessionStore{
		sessions: make(map[string]*AdminSession),
		auditLog: make([]AdminAuditEntry, 0),
		secret:   secret,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// IssueSession creates a new admin session for an authenticated user.
func (s *SessionStore) IssueSession(userID, workspaceID, role string) (*AdminSession, error) {
	if userID == "" || workspaceID == "" {
		return nil, fmt.Errorf("user_id and workspace_id are required")
	}
	if role != "admin" && role != "owner" {
		return nil, fmt.Errorf("only admin or owner roles can create admin sessions")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)
	tokenHash := s.hashToken(token)

	session := &AdminSession{
		ID:          uuid.Must(uuid.NewV7()).String(),
		UserID:      userID,
		WorkspaceID: workspaceID,
		Token:       token,
		TokenHash:   tokenHash,
		Role:        role,
		CreatedAt:   s.now(),
		ExpiresAt:   s.now().Add(8 * time.Hour),
	}

	s.mu.Lock()
	s.sessions[tokenHash] = session
	s.mu.Unlock()

	return session, nil
}

// ValidateSession validates a session token and returns the session if valid.
func (s *SessionStore) ValidateSession(token string) (*AdminSession, error) {
	if token == "" {
		return nil, fmt.Errorf("missing session token")
	}

	tokenHash := s.hashToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[tokenHash]
	if !ok {
		return nil, fmt.Errorf("invalid session token")
	}
	if s.now().After(session.ExpiresAt) {
		delete(s.sessions, tokenHash)
		return nil, fmt.Errorf("session expired")
	}

	return session, nil
}

// RevokeSession invalidates a session.
func (s *SessionStore) RevokeSession(token string) error {
	tokenHash := s.hashToken(token)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[tokenHash]; !ok {
		return fmt.Errorf("session not found")
	}
	delete(s.sessions, tokenHash)
	return nil
}

// AuditAction records an admin action for audit trail.
func (s *SessionStore) AuditAction(sessionID, userID, workspaceID, action, resource, ipAddress string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.auditLog = append(s.auditLog, AdminAuditEntry{
		ID:          uuid.Must(uuid.NewV7()).String(),
		SessionID:   sessionID,
		UserID:      userID,
		WorkspaceID: workspaceID,
		Action:      action,
		Resource:    resource,
		IPAddress:   ipAddress,
		CreatedAt:   s.now(),
	})
}

// GetAuditLog returns the audit log entries.
func (s *SessionStore) GetAuditLog() []AdminAuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]AdminAuditEntry, len(s.auditLog))
	copy(result, s.auditLog)
	return result
}

func (s *SessionStore) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

// AdminAuthMiddleware replaces X-User-Role header reliance with real session-based auth.
// It extracts the session token from the Authorization header (Bearer scheme),
// validates it, and injects the session into the request context.
func AdminAuthMiddleware(store *SessionStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header.
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Fallback: check X-Admin-Token header for backwards compat during migration.
				authHeader = "Bearer " + r.Header.Get("X-Admin-Token")
			}

			if !strings.HasPrefix(authHeader, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
				return
			}

			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "empty session token"})
				return
			}

			session, err := store.ValidateSession(token)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
				return
			}

			// Audit the access.
			store.AuditAction(session.ID, session.UserID, session.WorkspaceID, "access", r.URL.Path, r.RemoteAddr)

			// Inject session into context.
			ctx := context.WithValue(r.Context(), adminSessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type contextKey string

const adminSessionKey contextKey = "admin_session"

// SessionFromContext retrieves the admin session from the request context.
func SessionFromContext(ctx context.Context) *AdminSession {
	session, _ := ctx.Value(adminSessionKey).(*AdminSession)
	return session
}
