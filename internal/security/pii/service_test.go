package pii

import (
	"strings"
	"testing"
	"time"
)

func TestPIIEncryptionLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()
	record, err := s.EncryptField("email", "user@example.com")
	if err != nil {
		t.Fatalf("encrypt field: %v", err)
	}
	plaintext, err := s.DecryptField(record)
	if err != nil {
		t.Fatalf("decrypt field: %v", err)
	}
	if plaintext != "user@example.com" {
		t.Fatalf("unexpected plaintext: %s", plaintext)
	}
	redacted := s.Redact("user@example.com")
	if redacted == "user@example.com" {
		t.Fatalf("expected redacted value")
	}
}

func TestPIIKeyRotationWindow(t *testing.T) {
	t.Parallel()

	s := NewService()
	base := time.Date(2026, 2, 28, 11, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	record, err := s.EncryptField("ssn", "123-45-6789")
	if err != nil {
		t.Fatalf("encrypt before rotation: %v", err)
	}
	if err := s.RotateKey("v2", []byte("abcdef0123456789abcdef0123456789")); err != nil {
		t.Fatalf("rotate key: %v", err)
	}

	if _, err := s.DecryptField(record); err != nil {
		t.Fatalf("expected decrypt during dual-key window: %v", err)
	}

	s.now = func() time.Time { return base.Add(11 * time.Minute) }
	if _, err := s.DecryptField(record); err == nil {
		t.Fatal("expected decrypt failure after dual-key window expiry")
	}
}

func TestPIIRequiredFieldEncryptionPolicy(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.SetFieldPolicy(FieldPolicy{Field: "email", Required: true})
	s.SetFieldPolicy(FieldPolicy{Field: "nickname", Required: false})

	plaintext := map[string]string{
		"email":    "ceo@example.com",
		"nickname": "boss",
	}
	encrypted, err := s.EncryptPayload(plaintext)
	if err != nil {
		t.Fatalf("encrypt payload: %v", err)
	}
	if _, ok := encrypted["email"]; !ok {
		t.Fatalf("expected required email field to be encrypted")
	}
	if _, ok := encrypted["nickname"]; ok {
		t.Fatalf("did not expect optional field encryption by default")
	}
	if err := s.EnforceRequiredEncryption(plaintext, encrypted); err != nil {
		t.Fatalf("expected encryption policy enforcement success: %v", err)
	}

	delete(encrypted, "email")
	err = s.EnforceRequiredEncryption(plaintext, encrypted)
	if err == nil || !strings.Contains(err.Error(), "PII_ENCRYPTION_REQUIRED_email") {
		t.Fatalf("expected required encryption failure, got: %v", err)
	}
}
