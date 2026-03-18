package consent

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/google/uuid"
)

var testLogger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

func TestGrantAndRevoke(t *testing.T) {
	// Without a DB, grant returns an in-memory record.
	registry := NewConsentRegistry(nil, testLogger)

	wsID := uuid.New()
	userID := uuid.New()

	// Grant consent.
	rec, err := registry.GrantConsent(context.Background(), GrantConsentRequest{
		WorkspaceID: wsID,
		UserID:      userID,
		Purpose:     PurposeFineTuning,
		LawfulBasis: LawfulBasisConsent,
	})
	if err != nil {
		t.Fatalf("GrantConsent failed: %v", err)
	}
	if rec == nil {
		t.Fatal("Expected non-nil record")
	}
	if rec.Purpose != PurposeFineTuning {
		t.Errorf("Expected purpose=%s, got %s", PurposeFineTuning, rec.Purpose)
	}
	if rec.WorkspaceID != wsID {
		t.Errorf("Expected workspace_id=%s, got %s", wsID, rec.WorkspaceID)
	}

	// RevokeConsent without DB returns ErrConsentNotFound.
	err = registry.RevokeConsent(context.Background(), wsID, userID, PurposeFineTuning)
	if err != ErrConsentNotFound {
		t.Errorf("Expected ErrConsentNotFound without DB, got %v", err)
	}
}

func TestInvalidPurpose(t *testing.T) {
	registry := NewConsentRegistry(nil, testLogger)

	_, err := registry.GrantConsent(context.Background(), GrantConsentRequest{
		WorkspaceID: uuid.New(),
		UserID:      uuid.New(),
		Purpose:     ConsentPurpose("invalid_purpose"),
		LawfulBasis: LawfulBasisConsent,
	})
	if err == nil {
		t.Error("Expected error for invalid purpose")
	}
}

func TestInvalidLawfulBasis(t *testing.T) {
	registry := NewConsentRegistry(nil, testLogger)

	_, err := registry.GrantConsent(context.Background(), GrantConsentRequest{
		WorkspaceID: uuid.New(),
		UserID:      uuid.New(),
		Purpose:     PurposeAnalytics,
		LawfulBasis: LawfulBasis("invalid_basis"),
	})
	if err == nil {
		t.Error("Expected error for invalid lawful basis")
	}
}

func TestPurposeLimitationBlocks(t *testing.T) {
	registry := NewConsentRegistry(nil, testLogger)
	mw := NewPurposeLimitationMiddleware(registry, testLogger)

	wsID := uuid.New()
	userID := uuid.New()

	// Without consent, CheckToolConsent should return ErrConsentRequired.
	err := mw.CheckToolConsent(context.Background(), wsID, userID, "email.send", "marketing")
	if err == nil {
		t.Fatal("Expected ErrConsentRequired, got nil")
	}

	consentErr, ok := err.(ErrConsentRequired)
	if !ok {
		t.Fatalf("Expected ErrConsentRequired, got %T: %v", err, err)
	}
	if consentErr.Purpose != PurposeMarketing {
		t.Errorf("Expected purpose=marketing, got %s", consentErr.Purpose)
	}
	if consentErr.DataCategory != "marketing" {
		t.Errorf("Expected data_category=marketing, got %s", consentErr.DataCategory)
	}
}

func TestPurposeLimitationDefaultPurpose(t *testing.T) {
	registry := NewConsentRegistry(nil, testLogger)
	mw := NewPurposeLimitationMiddleware(registry, testLogger)

	wsID := uuid.New()
	userID := uuid.New()

	// Unknown data category defaults to executive_assistance.
	err := mw.CheckToolConsent(context.Background(), wsID, userID, "calendar.create", "general")
	if err == nil {
		t.Fatal("Expected ErrConsentRequired for default purpose")
	}

	consentErr, ok := err.(ErrConsentRequired)
	if !ok {
		t.Fatalf("Expected ErrConsentRequired, got %T: %v", err, err)
	}
	if consentErr.Purpose != PurposeExecutiveAssistance {
		t.Errorf("Expected purpose=executive_assistance, got %s", consentErr.Purpose)
	}
}

func TestConsentRecordIsActive(t *testing.T) {
	rec := ConsentRecord{
		Purpose: PurposeAnalytics,
	}
	if !rec.IsActive() {
		t.Error("Expected active consent with no revocation or expiry")
	}

	// Revoked consent should be inactive.
	now := rec.GrantedAt
	rec.RevokedAt = &now
	if rec.IsActive() {
		t.Error("Expected inactive consent after revocation")
	}
}

func TestErrConsentRequiredMessage(t *testing.T) {
	err := ErrConsentRequired{
		Purpose:      PurposeFineTuning,
		DataCategory: "training_data",
	}
	expected := "consent required for purpose=fine_tuning data_category=training_data"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestDeterminePurpose(t *testing.T) {
	tests := []struct {
		category string
		expected ConsentPurpose
	}{
		{"financial", PurposeExecutiveAssistance},
		{"health", PurposeExecutiveAssistance},
		{"analytics", PurposeAnalytics},
		{"marketing", PurposeMarketing},
		{"general", PurposeExecutiveAssistance},
		{"unknown", PurposeExecutiveAssistance},
	}

	for _, tt := range tests {
		got := determinePurpose(tt.category)
		if got != tt.expected {
			t.Errorf("determinePurpose(%q) = %s, want %s", tt.category, got, tt.expected)
		}
	}
}
