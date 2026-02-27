package connectors

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Connector struct {
	Key          string `yaml:"key"`
	Domain       string `yaml:"domain"`
	RiskLevel    string `yaml:"risk_level"`
	DataClass    string `yaml:"data_class"`
	MCPServerURL string `yaml:"mcp_server_url"`
}

type ConnectorTool struct {
	ConnectorKey  string `yaml:"connector_key"`
	ToolKey       string `yaml:"tool_key"`
	Write         bool   `yaml:"write"`
	Reversible    bool   `yaml:"reversible"`
	AutonomyFloor string `yaml:"autonomy_floor"`
}

type ConnectorHealth struct {
	P95LatencyMS int
	ErrorRate    float64
	UpdatedAt    time.Time
}

type OAuthEnvelope struct {
	Ciphertext  string
	Nonce       string
	KeyVersion  string
	EncryptedAt time.Time
}

type seedFile struct {
	Connectors []Connector     `yaml:"connectors"`
	Tools      []ConnectorTool `yaml:"tools"`
}

type KeyProvider interface {
	CurrentKey(ctx context.Context) (version string, key []byte, err error)
	KeyByVersion(ctx context.Context, version string) ([]byte, error)
}

type inMemoryKeyProvider struct {
	mu      sync.RWMutex
	current string
	keys    map[string][]byte
}

func NewInMemoryKeyProvider(initialVersion string, initialKey []byte) KeyProvider {
	keyCopy := make([]byte, len(initialKey))
	copy(keyCopy, initialKey)
	return &inMemoryKeyProvider{
		current: initialVersion,
		keys:    map[string][]byte{initialVersion: keyCopy},
	}
}

func (p *inMemoryKeyProvider) CurrentKey(_ context.Context) (string, []byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key := p.keys[p.current]
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return p.current, keyCopy, nil
}

func (p *inMemoryKeyProvider) KeyByVersion(_ context.Context, version string) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	key, ok := p.keys[version]
	if !ok {
		return nil, fmt.Errorf("key version not found: %s", version)
	}
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return keyCopy, nil
}

func (p *inMemoryKeyProvider) Rotate(version string, key []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := make([]byte, len(key))
	copy(k, key)
	p.keys[version] = k
	p.current = version
}

type Service struct {
	mu          sync.RWMutex
	connectors  map[string]Connector
	tools       map[string]ConnectorTool
	oauth       map[string]OAuthEnvelope
	health      map[string]ConnectorHealth
	keyProvider KeyProvider
}

func NewService(keyProvider KeyProvider) *Service {
	return &Service{
		connectors:  map[string]Connector{},
		tools:       map[string]ConnectorTool{},
		oauth:       map[string]OAuthEnvelope{},
		health:      map[string]ConnectorHealth{},
		keyProvider: keyProvider,
	}
}

func (s *Service) LoadSeedFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var seed seedFile
	if err := yaml.Unmarshal(content, &seed); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, connector := range seed.Connectors {
		s.connectors[connector.Key] = connector
	}
	for _, tool := range seed.Tools {
		s.tools[tool.ToolKey] = tool
	}
	return nil
}

func (s *Service) ConnectorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connectors)
}

func (s *Service) SetConnectorHealth(connectorKey string, p95LatencyMS int, errorRate float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health[connectorKey] = ConnectorHealth{P95LatencyMS: p95LatencyMS, ErrorRate: errorRate, UpdatedAt: time.Now().UTC()}
}

func oauthStorageKey(workspaceID, userID, connectorKey string) string {
	return workspaceID + "::" + userID + "::" + connectorKey
}

func (s *Service) EncryptOAuthToken(ctx context.Context, workspaceID, userID, connectorKey, token string) (OAuthEnvelope, error) {
	version, key, err := s.keyProvider.CurrentKey(ctx)
	if err != nil {
		return OAuthEnvelope{}, err
	}
	if len(key) != 32 {
		return OAuthEnvelope{}, fmt.Errorf("aes-256-gcm requires 32-byte key")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return OAuthEnvelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return OAuthEnvelope{}, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return OAuthEnvelope{}, err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(token), nil)
	envelope := OAuthEnvelope{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  version,
		EncryptedAt: time.Now().UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.oauth[oauthStorageKey(workspaceID, userID, connectorKey)] = envelope
	return envelope, nil
}

func (s *Service) DecryptOAuthToken(ctx context.Context, workspaceID, userID, connectorKey string) (string, OAuthEnvelope, error) {
	s.mu.RLock()
	envelope, ok := s.oauth[oauthStorageKey(workspaceID, userID, connectorKey)]
	s.mu.RUnlock()
	if !ok {
		return "", OAuthEnvelope{}, fmt.Errorf("oauth token not found")
	}

	key, err := s.keyProvider.KeyByVersion(ctx, envelope.KeyVersion)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}

	return string(plaintext), envelope, nil
}
