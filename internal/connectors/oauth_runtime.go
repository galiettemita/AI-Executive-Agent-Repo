package connectors

import (
	"strings"
	"sync"
	"time"
)

type OAuthStateRecord struct {
	Nonce        string
	WorkspaceID  string
	ConnectorKey string
	ExpiresAt    time.Time
}

type OAuthStateStore struct {
	mu    sync.Mutex
	state map[string]OAuthStateRecord
}

func NewOAuthStateStore() *OAuthStateStore {
	return &OAuthStateStore{state: map[string]OAuthStateRecord{}}
}

func (s *OAuthStateStore) Put(record OAuthStateRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[record.Nonce] = record
}

func (s *OAuthStateStore) Consume(nonce string, now time.Time) (OAuthStateRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.state[nonce]
	if !ok {
		return OAuthStateRecord{}, false
	}
	delete(s.state, nonce)
	if now.UTC().After(record.ExpiresAt.UTC()) {
		return OAuthStateRecord{}, false
	}
	return record, true
}

func OAuthRevocationEndpoint(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google":
		return "https://oauth2.googleapis.com/revoke"
	case "microsoft":
		return "https://login.microsoftonline.com/common/oauth2/v2.0/logout"
	case "apple":
		return "https://appleid.apple.com/auth/revoke"
	case "slack":
		return "https://slack.com/api/auth.revoke"
	case "zoom":
		return "https://zoom.us/oauth/revoke"
	case "github":
		return "https://api.github.com/applications/{client_id}/token"
	default:
		return ""
	}
}

func OAuthRefreshFailureEvent() string {
	return "BREVIO.oauth.refresh_failed.v1"
}

func OAuthStateInvalidEvent() string {
	return "BREVIO.security.oauth.state_invalid.v1"
}
