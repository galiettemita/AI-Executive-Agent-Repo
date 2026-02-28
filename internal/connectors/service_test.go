package connectors

import (
	"context"
	"path/filepath"
	"regexp"
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

func TestConnectorSeedIntegrityAndToolKeyFormat(t *testing.T) {
	t.Parallel()

	keyProvider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(keyProvider)
	seedPath := filepath.Join("seeds", "connectors.yaml")
	if err := svc.LoadSeedFile(seedPath); err != nil {
		t.Fatalf("load seed file: %v", err)
	}

	connectorKeyPattern := regexp.MustCompile(`^[a-z0-9_]+$`)
	toolKeyPattern := regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)
	allowedAutonomyFloors := map[string]struct{}{
		"A0": {},
		"A1": {},
		"A2": {},
		"A3": {},
		"A4": {},
	}

	if len(svc.connectors) < 40 {
		t.Fatalf("expected at least 40 connectors, got %d", len(svc.connectors))
	}
	if len(svc.tools) == 0 {
		t.Fatal("expected non-empty tool seed set")
	}

	for connectorKey := range svc.connectors {
		if !connectorKeyPattern.MatchString(connectorKey) {
			t.Fatalf("connector key violates snake_case constraint: %s", connectorKey)
		}
	}

	for toolKey, tool := range svc.tools {
		if !toolKeyPattern.MatchString(toolKey) {
			t.Fatalf("tool key violates connector_key.tool_key format: %s", toolKey)
		}
		if toolKey != tool.ToolKey {
			t.Fatalf("tool map key mismatch: map=%s payload=%s", toolKey, tool.ToolKey)
		}
		if _, ok := svc.connectors[tool.ConnectorKey]; !ok {
			t.Fatalf("tool references unknown connector: tool=%s connector=%s", toolKey, tool.ConnectorKey)
		}
		if len(toolKey) <= len(tool.ConnectorKey) {
			t.Fatalf("tool key too short for connector prefix check: tool=%s connector=%s", toolKey, tool.ConnectorKey)
		}
		prefix := toolKey[:len(tool.ConnectorKey)]
		if prefix != tool.ConnectorKey || toolKey[len(tool.ConnectorKey)] != '.' {
			t.Fatalf("tool key prefix must match connector key: tool=%s connector=%s", toolKey, tool.ConnectorKey)
		}
		if _, ok := allowedAutonomyFloors[tool.AutonomyFloor]; !ok {
			t.Fatalf("invalid autonomy floor for tool %s: %s", toolKey, tool.AutonomyFloor)
		}
	}
}
