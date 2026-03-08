package admin

import (
	"testing"
	"time"
)

func TestOAuthMonitorRegisterToken(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	entry, err := svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1",
		UserID:      "u1",
		Provider:    "google",
		ExpiresAt:   time.Now().UTC().Add(48 * time.Hour),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID == "" {
		t.Fatal("expected generated ID")
	}
	if entry.Status != "active" {
		t.Fatalf("expected active status, got %s", entry.Status)
	}
}

func TestOAuthMonitorRegisterTokenMissingFields(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()

	_, err := svc.RegisterToken(OAuthTokenEntry{UserID: "u1", Provider: "google"})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
	_, err = svc.RegisterToken(OAuthTokenEntry{WorkspaceID: "ws1", Provider: "google"})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
	_, err = svc.RegisterToken(OAuthTokenEntry{WorkspaceID: "ws1", UserID: "u1"})
	if err == nil {
		t.Fatal("expected error for missing provider")
	}
}

func TestOAuthMonitorGetExpiringTokens(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	_, _ = svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: now.Add(12 * time.Hour),
	})
	_, _ = svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "slack",
		ExpiresAt: now.Add(48 * time.Hour),
	})

	expiring := svc.GetExpiringTokens(24 * time.Hour)
	if len(expiring) != 1 {
		t.Fatalf("expected 1 expiring token, got %d", len(expiring))
	}
	if expiring[0].Provider != "google" {
		t.Fatalf("expected google, got %s", expiring[0].Provider)
	}
}

func TestOAuthMonitorRefreshToken(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	entry, _ := svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	})

	newExpiry := time.Now().UTC().Add(72 * time.Hour)
	err := svc.RefreshToken(entry.ID, newExpiry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	all := svc.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 token, got %d", len(all))
	}
	if all[0].Status != "refreshed" {
		t.Fatalf("expected refreshed status, got %s", all[0].Status)
	}
}

func TestOAuthMonitorRefreshTokenNotFound(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	err := svc.RefreshToken("nonexistent", time.Now().UTC())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestOAuthMonitorUpsertExisting(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	_, _ = svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: now.Add(1 * time.Hour),
	})
	// Re-register same provider — should update
	_, _ = svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: now.Add(72 * time.Hour),
	})

	all := svc.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 token after upsert, got %d", len(all))
	}
	if all[0].Status != "active" {
		t.Fatalf("expected active after extending expiry, got %s", all[0].Status)
	}
}

func TestOAuthMonitorExpiredStatus(t *testing.T) {
	t.Parallel()
	svc := NewOAuthMonitorService()
	now := time.Now().UTC()
	svc.now = func() time.Time { return now }

	entry, _ := svc.RegisterToken(OAuthTokenEntry{
		WorkspaceID: "ws1", UserID: "u1", Provider: "google",
		ExpiresAt: now.Add(-1 * time.Hour), // already expired
	})
	if entry.Status != "expired" {
		t.Fatalf("expected expired status, got %s", entry.Status)
	}
}
