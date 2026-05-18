package admin

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OAuthTokenEntry represents a monitored OAuth token.
type OAuthTokenEntry struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	UserID      string    `json:"user_id"`
	Provider    string    `json:"provider"`
	ExpiresAt   time.Time `json:"expires_at"`
	RefreshedAt time.Time `json:"refreshed_at,omitempty"`
	Status      string    `json:"status"` // active, expiring, expired, refreshed
}

// OAuthMonitorService monitors OAuth token expiry and refresh state.
type OAuthMonitorService struct {
	mu     sync.Mutex
	tokens map[string]*OAuthTokenEntry // key: ID
	index  map[string]string           // key: workspaceID:userID:provider -> ID
	now    func() time.Time
}

// NewOAuthMonitorService creates a new OAuthMonitorService.
func NewOAuthMonitorService() *OAuthMonitorService {
	return &OAuthMonitorService{
		tokens: map[string]*OAuthTokenEntry{},
		index:  map[string]string{},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func oauthTokenKey(workspaceID, userID, provider string) string {
	return workspaceID + ":" + userID + ":" + provider
}

// RegisterToken registers or updates an OAuth token for monitoring.
func (s *OAuthMonitorService) RegisterToken(entry OAuthTokenEntry) (OAuthTokenEntry, error) {
	if entry.WorkspaceID == "" {
		return OAuthTokenEntry{}, fmt.Errorf("workspace_id is required")
	}
	if entry.UserID == "" {
		return OAuthTokenEntry{}, fmt.Errorf("user_id is required")
	}
	if entry.Provider == "" {
		return OAuthTokenEntry{}, fmt.Errorf("provider is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key := oauthTokenKey(entry.WorkspaceID, entry.UserID, entry.Provider)
	if existingID, ok := s.index[key]; ok {
		// Update existing
		existing := s.tokens[existingID]
		existing.ExpiresAt = entry.ExpiresAt
		existing.Status = s.computeStatus(entry.ExpiresAt)
		return *existing, nil
	}

	entry.ID = uuid.Must(uuid.NewV7()).String()
	entry.Status = s.computeStatus(entry.ExpiresAt)
	s.tokens[entry.ID] = &entry
	s.index[key] = entry.ID
	return entry, nil
}

func (s *OAuthMonitorService) computeStatus(expiresAt time.Time) string {
	now := s.now()
	if expiresAt.Before(now) {
		return "expired"
	}
	if expiresAt.Before(now.Add(24 * time.Hour)) {
		return "expiring"
	}
	return "active"
}

// GetExpiringTokens returns tokens expiring within the given duration.
func (s *OAuthMonitorService) GetExpiringTokens(within time.Duration) []OAuthTokenEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	deadline := now.Add(within)
	var result []OAuthTokenEntry
	for _, t := range s.tokens {
		if t.ExpiresAt.After(now) && !t.ExpiresAt.After(deadline) {
			result = append(result, *t)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExpiresAt.Before(result[j].ExpiresAt)
	})
	return result
}

// RefreshToken marks a token as refreshed with a new expiry.
func (s *OAuthMonitorService) RefreshToken(id string, newExpiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tok, ok := s.tokens[id]
	if !ok {
		return fmt.Errorf("token %s not found", id)
	}
	tok.ExpiresAt = newExpiresAt
	tok.RefreshedAt = s.now()
	tok.Status = "refreshed"
	return nil
}

// GetAll returns all monitored tokens.
func (s *OAuthMonitorService) GetAll() []OAuthTokenEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]OAuthTokenEntry, 0, len(s.tokens))
	for _, t := range s.tokens {
		result = append(result, *t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}
