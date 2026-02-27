package connectors

import (
	"context"
	"path/filepath"
	"testing"
)

func TestSeedLoaderPopulatesAtLeast40Connectors(t *testing.T) {
	t.Parallel()

	keyProvider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(keyProvider)
	seedPath := filepath.Join("seeds", "connectors.yaml")
	if err := svc.LoadSeedFile(seedPath); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	if svc.ConnectorCount() < 40 {
		t.Fatalf("expected at least 40 connectors, got %d", svc.ConnectorCount())
	}
}

func TestOAuthEncryptDecryptRoundTripAndKeyVersion(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(provider)
	ctx := context.Background()

	envelope, err := svc.EncryptOAuthToken(ctx, "ws_1", "user_1", "gmail", "secret-token")
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	if envelope.KeyVersion != "v1" {
		t.Fatalf("unexpected key version: %s", envelope.KeyVersion)
	}

	plaintext, restoredEnvelope, err := svc.DecryptOAuthToken(ctx, "ws_1", "user_1", "gmail")
	if err != nil {
		t.Fatalf("decrypt token: %v", err)
	}
	if plaintext != "secret-token" {
		t.Fatalf("unexpected plaintext: %s", plaintext)
	}
	if restoredEnvelope.KeyVersion != "v1" {
		t.Fatalf("unexpected restored key version: %s", restoredEnvelope.KeyVersion)
	}

	provider.(*inMemoryKeyProvider).Rotate("v2", []byte("abcdef0123456789abcdef0123456789"))
	plaintextAfterRotate, restoredEnvelopeAfterRotate, err := svc.DecryptOAuthToken(ctx, "ws_1", "user_1", "gmail")
	if err != nil {
		t.Fatalf("decrypt token after rotation: %v", err)
	}
	if plaintextAfterRotate != "secret-token" {
		t.Fatalf("unexpected plaintext after rotation: %s", plaintextAfterRotate)
	}
	if restoredEnvelopeAfterRotate.KeyVersion != "v1" {
		t.Fatalf("expected stored envelope to keep original key version, got %s", restoredEnvelopeAfterRotate.KeyVersion)
	}
}
