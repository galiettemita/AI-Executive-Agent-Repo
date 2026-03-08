package pii

import (
	"context"
	"testing"
	"time"
)

func TestPIIEncryptDecryptField(t *testing.T) {
	t.Parallel()

	piiSvc := NewService()
	svc := NewPIIEncryptionService(piiSvc)

	_ = svc.SetPolicy("ws-1", PIIEncryptionPolicy{
		WorkspaceID:  "ws-1",
		RetentionDays: 30,
		Fields: []PIIField{
			{FieldPath: "user.email", DataClass: "email", SensitivityLevel: "high"},
		},
	})

	ctx := context.Background()
	ct, err := svc.EncryptField(ctx, "ws-1", "user.email", []byte("alice@example.com"))
	if err != nil {
		t.Fatalf("encrypt error: %v", err)
	}
	if len(ct) == 0 {
		t.Fatalf("expected non-empty ciphertext")
	}

	pt, err := svc.DecryptField(ctx, "ws-1", "user.email", ct)
	if err != nil {
		t.Fatalf("decrypt error: %v", err)
	}
	if string(pt) != "alice@example.com" {
		t.Fatalf("expected alice@example.com, got %s", string(pt))
	}
}

func TestPIIEncryptFieldValidation(t *testing.T) {
	t.Parallel()

	piiSvc := NewService()
	svc := NewPIIEncryptionService(piiSvc)
	ctx := context.Background()

	_, err := svc.EncryptField(ctx, "", "field", []byte("data"))
	if err == nil {
		t.Fatalf("expected error for empty workspace_id")
	}
	_, err = svc.EncryptField(ctx, "ws", "", []byte("data"))
	if err == nil {
		t.Fatalf("expected error for empty field_path")
	}
}

func TestPIIRetentionExpiry(t *testing.T) {
	t.Parallel()

	piiSvc := NewService()
	svc := NewPIIEncryptionService(piiSvc)

	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return baseTime })

	_ = svc.SetPolicy("ws-ret", PIIEncryptionPolicy{
		WorkspaceID:  "ws-ret",
		RetentionDays: 7,
	})

	ctx := context.Background()
	_, _ = svc.EncryptField(ctx, "ws-ret", "user.ssn", []byte("123-45-6789"))

	// Advance time past retention.
	svc.SetNowFunc(func() time.Time { return baseTime.AddDate(0, 0, 8) })

	_, err := svc.DecryptField(ctx, "ws-ret", "user.ssn", nil)
	if err == nil {
		t.Fatalf("expected retention expiry error")
	}
}

func TestPIIEnforceRetention(t *testing.T) {
	t.Parallel()

	piiSvc := NewService()
	svc := NewPIIEncryptionService(piiSvc)

	baseTime := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	svc.SetNowFunc(func() time.Time { return baseTime })

	_ = svc.SetPolicy("ws-purge", PIIEncryptionPolicy{
		WorkspaceID:  "ws-purge",
		RetentionDays: 5,
	})

	ctx := context.Background()
	_, _ = svc.EncryptField(ctx, "ws-purge", "f1", []byte("v1"))
	_, _ = svc.EncryptField(ctx, "ws-purge", "f2", []byte("v2"))

	// Before expiry: nothing purged.
	purged, err := svc.EnforceRetention("ws-purge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if purged != 0 {
		t.Fatalf("expected 0 purged, got %d", purged)
	}

	// Advance past retention.
	svc.SetNowFunc(func() time.Time { return baseTime.AddDate(0, 0, 6) })

	purged, err = svc.EnforceRetention("ws-purge")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if purged != 2 {
		t.Fatalf("expected 2 purged, got %d", purged)
	}
}

func TestPIIPolicySetGet(t *testing.T) {
	t.Parallel()

	piiSvc := NewService()
	svc := NewPIIEncryptionService(piiSvc)

	err := svc.SetPolicy("", PIIEncryptionPolicy{})
	if err == nil {
		t.Fatalf("expected error for empty workspace_id")
	}

	err = svc.SetPolicy("ws-p", PIIEncryptionPolicy{RetentionDays: -1})
	if err == nil {
		t.Fatalf("expected error for negative retention_days")
	}

	_ = svc.SetPolicy("ws-p", PIIEncryptionPolicy{RetentionDays: 90, AutoRedact: true})
	p, err := svc.GetPolicy("ws-p")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.RetentionDays != 90 || !p.AutoRedact {
		t.Fatalf("unexpected policy: %+v", p)
	}

	_, err = svc.GetPolicy("nonexistent")
	if err == nil {
		t.Fatalf("expected error for missing policy")
	}
}

func TestScanForPII(t *testing.T) {
	t.Parallel()

	text := "Contact alice@example.com or call 555-123-4567. SSN is 123-45-6789."
	results := ScanForPII(text)

	types := map[string]bool{}
	for _, r := range results {
		types[r.Type] = true
	}
	if !types["email"] {
		t.Fatalf("expected email detection")
	}
	if !types["ssn"] {
		t.Fatalf("expected SSN detection")
	}
	if !types["phone"] {
		t.Fatalf("expected phone detection")
	}
}

func TestScanForPIINoMatch(t *testing.T) {
	t.Parallel()

	results := ScanForPII("This is a perfectly safe sentence with no personal data.")
	if len(results) != 0 {
		t.Fatalf("expected 0 detections, got %d", len(results))
	}
}

func TestRedactPII(t *testing.T) {
	t.Parallel()

	text := "Email alice@example.com and SSN 123-45-6789"
	redacted, detected := RedactPII(text)

	if len(detected) < 2 {
		t.Fatalf("expected at least 2 detections, got %d", len(detected))
	}
	if !containsString(redacted, "[REDACTED:email]") {
		t.Fatalf("expected email redaction marker in %q", redacted)
	}
	if !containsString(redacted, "[REDACTED:ssn]") {
		t.Fatalf("expected SSN redaction marker in %q", redacted)
	}
	if containsString(redacted, "alice@example.com") {
		t.Fatalf("original email should not appear in redacted text")
	}
}

func TestRedactPIINoMatch(t *testing.T) {
	t.Parallel()

	text := "Nothing to see here."
	redacted, detected := RedactPII(text)
	if redacted != text {
		t.Fatalf("expected unchanged text")
	}
	if detected != nil {
		t.Fatalf("expected nil detections")
	}
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		findSubstring(haystack, needle))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
