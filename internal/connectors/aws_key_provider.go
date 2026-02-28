package connectors

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type AWSSecretsManagerClient interface {
	GetSecretValue(ctx context.Context, secretID string) (string, error)
}

type awsSecretsManagerSecret struct {
	CurrentKeyVersion string            `json:"current_key_version"`
	Keys              map[string]string `json:"keys"`
}

type AWSSecretsManagerKeyProvider struct {
	mu       sync.RWMutex
	client   AWSSecretsManagerClient
	secretID string
	current  string
	keys     map[string][]byte
}

func NewAWSSecretsManagerKeyProvider(client AWSSecretsManagerClient, secretID string) (*AWSSecretsManagerKeyProvider, error) {
	if client == nil {
		return nil, fmt.Errorf("secrets manager client is required")
	}
	if strings.TrimSpace(secretID) == "" {
		return nil, fmt.Errorf("secret_id is required")
	}
	return &AWSSecretsManagerKeyProvider{
		client:   client,
		secretID: secretID,
		keys:     map[string][]byte{},
	}, nil
}

func (p *AWSSecretsManagerKeyProvider) CurrentKey(ctx context.Context) (string, []byte, error) {
	if err := p.ensureLoaded(ctx); err != nil {
		return "", nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	key, ok := p.keys[p.current]
	if !ok {
		return "", nil, fmt.Errorf("current key version not found: %s", p.current)
	}
	copyKey := make([]byte, len(key))
	copy(copyKey, key)
	return p.current, copyKey, nil
}

func (p *AWSSecretsManagerKeyProvider) KeyByVersion(ctx context.Context, version string) ([]byte, error) {
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("version is required")
	}
	if err := p.ensureLoaded(ctx); err != nil {
		return nil, err
	}

	p.mu.RLock()
	key, ok := p.keys[version]
	p.mu.RUnlock()
	if ok {
		copyKey := make([]byte, len(key))
		copy(copyKey, key)
		return copyKey, nil
	}

	if err := p.Refresh(ctx); err != nil {
		return nil, err
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	key, ok = p.keys[version]
	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}
	copyKey := make([]byte, len(key))
	copy(copyKey, key)
	return copyKey, nil
}

func (p *AWSSecretsManagerKeyProvider) Refresh(ctx context.Context) error {
	secret, err := p.client.GetSecretValue(ctx, p.secretID)
	if err != nil {
		return fmt.Errorf("get secret value: %w", err)
	}
	parsed, err := parseAWSSecretsManagerSecret(secret)
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = parsed.CurrentKeyVersion
	p.keys = parsed.Keys
	return nil
}

func (p *AWSSecretsManagerKeyProvider) ensureLoaded(ctx context.Context) error {
	p.mu.RLock()
	loaded := p.current != "" && len(p.keys) > 0
	p.mu.RUnlock()
	if loaded {
		return nil
	}
	return p.Refresh(ctx)
}

type parsedSecrets struct {
	CurrentKeyVersion string
	Keys              map[string][]byte
}

func parseAWSSecretsManagerSecret(secret string) (parsedSecrets, error) {
	payload := awsSecretsManagerSecret{}
	if err := json.Unmarshal([]byte(secret), &payload); err != nil {
		return parsedSecrets{}, fmt.Errorf("parse aws secrets manager key payload: %w", err)
	}
	if strings.TrimSpace(payload.CurrentKeyVersion) == "" {
		return parsedSecrets{}, fmt.Errorf("current_key_version is required")
	}
	if len(payload.Keys) == 0 {
		return parsedSecrets{}, fmt.Errorf("keys must be non-empty")
	}
	parsed := map[string][]byte{}
	for version, encoded := range payload.Keys {
		if strings.TrimSpace(version) == "" {
			return parsedSecrets{}, fmt.Errorf("key version must be non-empty")
		}
		decoded, err := decodeKeyMaterial(encoded)
		if err != nil {
			return parsedSecrets{}, fmt.Errorf("decode key version %s: %w", version, err)
		}
		parsed[version] = decoded
	}
	if _, ok := parsed[payload.CurrentKeyVersion]; !ok {
		return parsedSecrets{}, fmt.Errorf("current_key_version %s is missing from keys", payload.CurrentKeyVersion)
	}
	return parsedSecrets{CurrentKeyVersion: payload.CurrentKeyVersion, Keys: parsed}, nil
}

func decodeKeyMaterial(value string) ([]byte, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("key material is empty")
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(trimmed); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	if len(trimmed) == 32 {
		return []byte(trimmed), nil
	}
	return nil, fmt.Errorf("key material must decode to 32 bytes")
}
