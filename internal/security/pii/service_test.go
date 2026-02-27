package pii

import "testing"

func TestPIIEncryptionLifecycle(t *testing.T) {
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
