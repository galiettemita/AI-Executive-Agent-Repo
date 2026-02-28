package connectors

import (
	"context"
	"fmt"
	"testing"
)

type fakeAWSSecretsManagerClient struct {
	secret string
	err    error
}

func (f *fakeAWSSecretsManagerClient) GetSecretValue(_ context.Context, secretID string) (string, error) {
	if secretID == "" {
		return "", fmt.Errorf("secret_id required")
	}
	if f.err != nil {
		return "", f.err
	}
	return f.secret, nil
}

func TestAWSSecretsManagerKeyProviderCurrentAndVersion(t *testing.T) {
	t.Parallel()

	client := &fakeAWSSecretsManagerClient{secret: `{"current_key_version":"v2","keys":{"v1":"MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=","v2":"YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODk="}}`}
	provider, err := NewAWSSecretsManagerKeyProvider(client, "oauth-key-secret")
	if err != nil {
		t.Fatalf("new aws key provider: %v", err)
	}

	version, key, err := provider.CurrentKey(context.Background())
	if err != nil {
		t.Fatalf("current key: %v", err)
	}
	if version != "v2" {
		t.Fatalf("unexpected current version: %s", version)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	v1, err := provider.KeyByVersion(context.Background(), "v1")
	if err != nil {
		t.Fatalf("key by version v1: %v", err)
	}
	if len(v1) != 32 {
		t.Fatalf("expected 32-byte v1 key, got %d", len(v1))
	}
}

func TestAWSSecretsManagerKeyProviderRefreshAndVersionLookup(t *testing.T) {
	t.Parallel()

	client := &fakeAWSSecretsManagerClient{secret: `{"current_key_version":"v1","keys":{"v1":"0123456789abcdef0123456789abcdef"}}`}
	provider, err := NewAWSSecretsManagerKeyProvider(client, "oauth-key-secret")
	if err != nil {
		t.Fatalf("new aws key provider: %v", err)
	}

	if _, _, err := provider.CurrentKey(context.Background()); err != nil {
		t.Fatalf("current key first load: %v", err)
	}

	client.secret = `{"current_key_version":"v2","keys":{"v1":"0123456789abcdef0123456789abcdef","v2":"abcdef0123456789abcdef0123456789"}}`
	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	version, key, err := provider.CurrentKey(context.Background())
	if err != nil {
		t.Fatalf("current key after refresh: %v", err)
	}
	if version != "v2" || len(key) != 32 {
		t.Fatalf("unexpected refreshed key: version=%s len=%d", version, len(key))
	}

	if _, err := provider.KeyByVersion(context.Background(), "v1"); err != nil {
		t.Fatalf("expected dual-version lookup to include v1 after refresh: %v", err)
	}
}

func TestAWSSecretsManagerKeyProviderRejectsInvalidPayload(t *testing.T) {
	t.Parallel()

	client := &fakeAWSSecretsManagerClient{secret: `{"current_key_version":"v1","keys":{"v1":"short"}}`}
	provider, err := NewAWSSecretsManagerKeyProvider(client, "oauth-key-secret")
	if err != nil {
		t.Fatalf("new aws key provider: %v", err)
	}

	if _, _, err := provider.CurrentKey(context.Background()); err == nil {
		t.Fatal("expected invalid key payload to fail")
	}
}
