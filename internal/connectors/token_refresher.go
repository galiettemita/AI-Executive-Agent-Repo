package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// TokenRefresher manages OAuth 2.0 token refresh (RFC 6749 section 6).
// It works with the encrypted token store via the connectors.Service.
type TokenRefresher struct {
	mu         sync.Mutex
	httpClient *http.Client
	svc        *Service
}

// NewTokenRefresher creates a refresher backed by the given connector service.
func NewTokenRefresher(svc *Service) *TokenRefresher {
	return &TokenRefresher{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		svc:        svc,
	}
}

// RefreshIfExpired retrieves the OAuth token for workspaceID/userID/connectorKey,
// refreshing it if it expires within 5 minutes. Returns the valid access token.
func (r *TokenRefresher) RefreshIfExpired(ctx context.Context, workspaceID, userID, connectorKey string) (string, error) {
	if r.svc == nil {
		return "", fmt.Errorf("token_refresher: connector service is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	token, _, err := r.svc.GetOAuthTokenForUse(ctx, workspaceID, userID, connectorKey,
		5*time.Minute, // refresh window
		func(refreshToken string) (string, time.Time, error) {
			return r.callTokenEndpoint(ctx, connectorKey, refreshToken)
		},
	)
	if err != nil {
		return "", fmt.Errorf("token_refresher: %w", err)
	}
	return token, nil
}

// callTokenEndpoint performs the RFC 6749 token refresh HTTP call.
// The tokenURL, clientID, and clientSecret would come from connector configuration
// in a full implementation. For now, this returns an error since no token endpoint
// is configured — the GetOAuthTokenForUse path only calls this when the token is
// within the refresh window AND a refresh token exists.
func (r *TokenRefresher) callTokenEndpoint(ctx context.Context, connectorKey, refreshToken string) (string, time.Time, error) {
	// Look up the token endpoint from environment or connector config.
	tokenURL := connectorTokenEndpoint(connectorKey)
	if tokenURL == "" {
		return "", time.Time{}, fmt.Errorf("no token endpoint configured for connector %s", connectorKey)
	}

	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL,
		strings.NewReader(data.Encode()))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", time.Time{}, fmt.Errorf("parse token response: %w", err)
	}

	expiresAt := time.Now().Add(time.Duration(parsed.ExpiresIn) * time.Second)
	if parsed.ExpiresIn == 0 {
		expiresAt = time.Now().Add(60 * time.Minute)
	}

	return parsed.AccessToken, expiresAt, nil
}

// connectorTokenEndpoint returns the OAuth token endpoint URL for a connector.
// In production this would be stored in the connector configuration.
func connectorTokenEndpoint(connectorKey string) string {
	endpoints := map[string]string{
		"google_calendar": "https://oauth2.googleapis.com/token",
		"gmail":           "https://oauth2.googleapis.com/token",
		"slack":           "https://slack.com/api/oauth.v2.access",
	}
	return endpoints[connectorKey]
}
