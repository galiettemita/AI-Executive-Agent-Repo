package encryption

import (
	"bytes"
	"crypto/rand"
	"testing"
)

func generateKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	return key
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	key := generateKey(t)

	if err := svc.AddKey("v1", key); err != nil {
		t.Fatalf("add key: %v", err)
	}
	if err := svc.SetActiveKey("v1"); err != nil {
		t.Fatalf("set active key: %v", err)
	}

	plaintext := []byte("oauth-token-secret-value-12345")
	ciphertext, version, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if version != "v1" {
		t.Fatalf("expected version=v1, got=%s", version)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should differ from plaintext")
	}

	decrypted, err := svc.Decrypt(ciphertext, version)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted mismatch: got=%s want=%s", decrypted, plaintext)
	}
}

func TestEncryptWithoutActiveKeyFails(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	_, _, err := svc.Encrypt([]byte("data"))
	if err == nil {
		t.Fatal("expected error encrypting without active key")
	}
}

func TestDecryptWithWrongKeyFails(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	key1 := generateKey(t)
	key2 := generateKey(t)

	svc.AddKey("v1", key1)
	svc.AddKey("v2", key2)
	svc.SetActiveKey("v1")

	ciphertext, _, err := svc.Encrypt([]byte("sensitive-data"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Decrypt with wrong key version should fail.
	_, err = svc.Decrypt(ciphertext, "v2")
	if err == nil {
		t.Fatal("expected decryption failure with wrong key")
	}
}

func TestDecryptWithUnknownVersionFails(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	_, err := svc.Decrypt([]byte("some-ciphertext-data-longer-than-12"), "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown key version")
	}
}

func TestKeyRotationDualRead(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	key1 := generateKey(t)
	key2 := generateKey(t)

	svc.AddKey("v1", key1)
	svc.SetActiveKey("v1")

	plaintext := []byte("token-encrypted-with-v1")
	ct1, ver1, err := svc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt v1: %v", err)
	}
	if ver1 != "v1" {
		t.Fatalf("expected ver=v1 got=%s", ver1)
	}

	// Rotate to v2.
	if err := svc.RotateKey("v2", key2); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	// New encryptions should use v2.
	ct2, ver2, err := svc.Encrypt([]byte("token-encrypted-with-v2"))
	if err != nil {
		t.Fatalf("encrypt v2: %v", err)
	}
	if ver2 != "v2" {
		t.Fatalf("expected ver=v2 got=%s", ver2)
	}

	// Old ciphertext can still be decrypted with v1.
	dec1, err := svc.Decrypt(ct1, "v1")
	if err != nil {
		t.Fatalf("decrypt v1: %v", err)
	}
	if !bytes.Equal(dec1, plaintext) {
		t.Fatalf("v1 decryption mismatch")
	}

	// New ciphertext decrypted with v2.
	dec2, err := svc.Decrypt(ct2, "v2")
	if err != nil {
		t.Fatalf("decrypt v2: %v", err)
	}
	if !bytes.Equal(dec2, []byte("token-encrypted-with-v2")) {
		t.Fatalf("v2 decryption mismatch")
	}
}

func TestAddKeyInvalidSize(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	err := svc.AddKey("v1", []byte("too-short"))
	if err == nil {
		t.Fatal("expected error for invalid key size")
	}
}

func TestAddKeyEmptyVersion(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	err := svc.AddKey("", generateKey(t))
	if err == nil {
		t.Fatal("expected error for empty version")
	}
}

func TestListKeyVersions(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	svc.AddKey("v2", generateKey(t))
	svc.AddKey("v1", generateKey(t))
	svc.AddKey("v3", generateKey(t))

	versions := svc.ListKeyVersions()
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got=%d", len(versions))
	}
	// Should be sorted.
	if versions[0] != "v1" || versions[1] != "v2" || versions[2] != "v3" {
		t.Fatalf("versions not sorted: %v", versions)
	}
}

func TestCiphertextIncludesNonce(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	svc.AddKey("v1", generateKey(t))
	svc.SetActiveKey("v1")

	ct, _, err := svc.Encrypt([]byte("x"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Ciphertext should be at least nonce (12) + 1 byte encrypted + tag (16).
	if len(ct) < 12+1+16 {
		t.Fatalf("ciphertext too short: %d bytes", len(ct))
	}
}

func TestDecryptTooShortCiphertext(t *testing.T) {
	t.Parallel()

	svc := NewEncryptionService()
	svc.AddKey("v1", generateKey(t))

	_, err := svc.Decrypt([]byte("short"), "v1")
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}
