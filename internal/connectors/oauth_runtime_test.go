package connectors

import (
	"testing"
	"time"
)

func TestOAuthStateStoreConsume(t *testing.T) {
	t.Parallel()

	store := NewOAuthStateStore()
	now := time.Date(2026, 3, 1, 22, 0, 0, 0, time.UTC)
	store.Put(OAuthStateRecord{
		Nonce:        "nonce_1",
		WorkspaceID:  "ws_1",
		ConnectorKey: "google_calendar",
		ExpiresAt:    now.Add(10 * time.Minute),
	})
	record, ok := store.Consume("nonce_1", now)
	if !ok || record.ConnectorKey != "google_calendar" {
		t.Fatalf("unexpected state consume result: ok=%v record=%+v", ok, record)
	}
	if _, ok := store.Consume("nonce_1", now); ok {
		t.Fatal("expected nonce replay protection on second consume")
	}
}

func TestOAuthRevocationEndpointsAndEvents(t *testing.T) {
	t.Parallel()

	if OAuthRevocationEndpoint("google") == "" || OAuthRevocationEndpoint("github") == "" {
		t.Fatal("expected revocation endpoints for known providers")
	}
	if OAuthRefreshFailureEvent() != "BREVIO.oauth.refresh_failed.v1" {
		t.Fatalf("unexpected refresh-failure event: %s", OAuthRefreshFailureEvent())
	}
	if OAuthStateInvalidEvent() != "BREVIO.security.oauth.state_invalid.v1" {
		t.Fatalf("unexpected state-invalid event: %s", OAuthStateInvalidEvent())
	}
}
