package connectors

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestOAuthStateRoundTrip(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	state, err := GenerateOAuthState([]byte("state-key"), "ws_1", "google_calendar", "nonce_1", now)
	if err != nil {
		t.Fatalf("generate state: %v", err)
	}
	payload, err := ValidateOAuthState([]byte("state-key"), state, now.Add(9*time.Minute), 10*time.Minute)
	if err != nil {
		t.Fatalf("validate state: %v", err)
	}
	if payload.WorkspaceID != "ws_1" || payload.ConnectorKey != "google_calendar" || payload.Nonce != "nonce_1" {
		t.Fatalf("unexpected state payload: %+v", payload)
	}
	if _, err := ValidateOAuthState([]byte("bad-key"), state, now, 10*time.Minute); !errors.Is(err, ErrOAuthStateInvalid) {
		t.Fatalf("expected invalid state error, got %v", err)
	}
	if _, err := ValidateOAuthState([]byte("state-key"), state, now.Add(11*time.Minute), 10*time.Minute); !errors.Is(err, ErrOAuthStateExpired) {
		t.Fatalf("expected expired state error, got %v", err)
	}
}

func TestPKCEChallengeAndAuthorizationURL(t *testing.T) {
	t.Parallel()

	verifier := GeneratePKCECodeVerifier([]byte("deterministic-seed-for-tests"))
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Fatalf("unexpected verifier length: %d", len(verifier))
	}
	challenge, err := BuildPKCECodeChallengeS256(verifier)
	if err != nil {
		t.Fatalf("build challenge: %v", err)
	}
	rawURL, err := BuildOAuthAuthorizationURL(
		"https://accounts.google.com/o/oauth2/v2/auth",
		"client-123",
		"https://api.brevio.local/v1/provision/callback",
		[]string{"openid", "email"},
		"state_123",
		challenge,
		map[string]string{"access_type": "offline", "prompt": "consent"},
	)
	if err != nil {
		t.Fatalf("build auth url: %v", err)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse built auth url: %v", err)
	}
	query := parsed.Query()
	if query.Get("code_challenge_method") != "S256" || query.Get("state") != "state_123" {
		t.Fatalf("unexpected auth query: %v", query)
	}
	if !strings.Contains(query.Get("scope"), "openid") {
		t.Fatalf("expected scope parameter in query: %v", query)
	}
}

func TestOAuthErrorActionsAndRefreshWindow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	if !TokenNeedsRefresh(now.Add(4*time.Minute), now) {
		t.Fatal("expected refresh window within 5 minutes")
	}
	if TokenNeedsRefresh(now.Add(10*time.Minute), now) {
		t.Fatal("did not expect refresh window outside 5 minutes")
	}
	if got := OAuthErrorAction("invalid_grant"); got != "mark_needs_reauth" {
		t.Fatalf("unexpected error action: %s", got)
	}
	if got := OAuthStateRedisKey("abc123"); got != "oauth_state:abc123" {
		t.Fatalf("unexpected redis key: %s", got)
	}
}
