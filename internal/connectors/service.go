package connectors

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Connector struct {
	Key          string `yaml:"key" json:"key"`
	Domain       string `yaml:"domain" json:"domain"`
	RiskLevel    string `yaml:"risk_level" json:"risk_level"`
	DataClass    string `yaml:"data_class" json:"data_class"`
	MCPServerURL string `yaml:"mcp_server_url" json:"mcp_server_url"`
}

type ConnectorTool struct {
	ConnectorKey  string                 `yaml:"connector_key" json:"connector_key"`
	ToolKey       string                 `yaml:"tool_key" json:"tool_key"`
	Write         bool                   `yaml:"write" json:"write"`
	Reversible    bool                   `yaml:"reversible" json:"reversible"`
	AutonomyFloor string                 `yaml:"autonomy_floor" json:"autonomy_floor"`
	InputSchema   map[string]interface{} `yaml:"input_schema" json:"input_schema"`
	OutputSchema  map[string]interface{} `yaml:"output_schema" json:"output_schema"`
}

type ConnectorHealth struct {
	WorkspaceID  string
	ConnectorKey string
	P95LatencyMS int
	ErrorRate    float64
	UpdatedAt    time.Time
}

type UserConnectorSetting struct {
	WorkspaceID     string
	UserID          string
	ConnectorKey    string
	Enabled         bool
	CustomRateLimit *int
	UpdatedAt       time.Time
}

type OAuthEnvelope struct {
	Ciphertext  string
	Nonce       string
	KeyVersion  string
	EncryptedAt time.Time
}

type seedFile struct {
	Connectors []Connector     `yaml:"connectors" json:"connectors"`
	Tools      []ConnectorTool `yaml:"tools" json:"tools"`
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

var connectorKeyPattern = regexp.MustCompile(`^[a-z0-9_]+$`)
var toolKeyPattern = regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)
var validRiskLevels = map[string]struct{}{"LOW": {}, "MEDIUM": {}, "ELEVATED": {}, "CRITICAL": {}}
var validDataClasses = map[string]struct{}{"public": {}, "internal": {}, "confidential": {}, "restricted": {}}
var validAutonomyFloors = map[string]struct{}{"A0": {}, "A1": {}, "A2": {}, "A3": {}, "A4": {}}

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
	mu                    sync.RWMutex
	connectors            map[string]Connector
	tools                 map[string]ConnectorTool
	oauth                 map[string]OAuthEnvelope
	health                map[string]ConnectorHealth
	userConnectorSettings map[string]UserConnectorSetting
	keyProvider           KeyProvider
}

func NewService(keyProvider KeyProvider) *Service {
	return &Service{
		connectors:            map[string]Connector{},
		tools:                 map[string]ConnectorTool{},
		oauth:                 map[string]OAuthEnvelope{},
		health:                map[string]ConnectorHealth{},
		userConnectorSettings: map[string]UserConnectorSetting{},
		keyProvider:           keyProvider,
	}
}

func (s *Service) LoadSeedFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	seed, err := parseSeed(filepath.Ext(path), content)
	if err != nil {
		return err
	}
	return s.LoadSeed(seed)
}

func (s *Service) LoadSeed(seed seedFile) error {
	connectorMap := make(map[string]Connector, len(seed.Connectors))
	for _, connector := range seed.Connectors {
		if err := validateConnector(connector); err != nil {
			return err
		}
		connectorMap[connector.Key] = connector
	}

	for _, tool := range seed.Tools {
		tool = normalizeTool(tool)
		if err := validateTool(tool, connectorMap); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, connector := range seed.Connectors {
		s.connectors[connector.Key] = connector
	}
	for _, tool := range seed.Tools {
		tool = normalizeTool(tool)
		s.tools[tool.ToolKey] = tool
	}
	return nil
}

func parseSeed(ext string, content []byte) (seedFile, error) {
	var seed seedFile
	unmarshalErr := error(nil)
	switch ext {
	case ".json":
		unmarshalErr = json.Unmarshal(content, &seed)
	case ".yaml", ".yml", "":
		unmarshalErr = yaml.Unmarshal(content, &seed)
		if unmarshalErr != nil && ext == "" {
			unmarshalErr = json.Unmarshal(content, &seed)
		}
	default:
		unmarshalErr = yaml.Unmarshal(content, &seed)
		if unmarshalErr != nil {
			unmarshalErr = json.Unmarshal(content, &seed)
		}
	}
	if unmarshalErr != nil {
		return seedFile{}, fmt.Errorf("parse connector seed: %w", unmarshalErr)
	}
	return seed, nil
}

func validateConnector(connector Connector) error {
	if !connectorKeyPattern.MatchString(connector.Key) {
		return fmt.Errorf("invalid connector key %q", connector.Key)
	}
	if connector.Domain == "" {
		return fmt.Errorf("connector %s has empty domain", connector.Key)
	}
	if _, ok := validRiskLevels[connector.RiskLevel]; !ok {
		return fmt.Errorf("connector %s has invalid risk level %q", connector.Key, connector.RiskLevel)
	}
	if _, ok := validDataClasses[connector.DataClass]; !ok {
		return fmt.Errorf("connector %s has invalid data class %q", connector.Key, connector.DataClass)
	}
	if connector.MCPServerURL == "" {
		return fmt.Errorf("connector %s has empty mcp_server_url", connector.Key)
	}
	return nil
}

func validateTool(tool ConnectorTool, connectorMap map[string]Connector) error {
	if !toolKeyPattern.MatchString(tool.ToolKey) {
		return fmt.Errorf("invalid tool key %q", tool.ToolKey)
	}
	if _, ok := connectorMap[tool.ConnectorKey]; !ok {
		return fmt.Errorf("tool %s references unknown connector %s", tool.ToolKey, tool.ConnectorKey)
	}
	if _, ok := validAutonomyFloors[tool.AutonomyFloor]; !ok {
		return fmt.Errorf("tool %s has invalid autonomy floor %q", tool.ToolKey, tool.AutonomyFloor)
	}
	if expectedPrefix := tool.ConnectorKey + "."; len(tool.ToolKey) <= len(expectedPrefix) || tool.ToolKey[:len(expectedPrefix)] != expectedPrefix {
		return fmt.Errorf("tool key %s must start with connector prefix %s", tool.ToolKey, expectedPrefix)
	}
	return nil
}

func normalizeTool(tool ConnectorTool) ConnectorTool {
	if tool.InputSchema == nil {
		tool.InputSchema = map[string]interface{}{}
	}
	if tool.OutputSchema == nil {
		tool.OutputSchema = map[string]interface{}{}
	}
	return tool
}

func (s *Service) ConnectorCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connectors)
}

func (s *Service) ToolCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tools)
}

func (s *Service) ListConnectors() []Connector {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.connectors))
	for key := range s.connectors {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	connectors := make([]Connector, 0, len(keys))
	for _, key := range keys {
		connectors = append(connectors, s.connectors[key])
	}
	return connectors
}

func (s *Service) ListToolsByConnector(connectorKey string) []ConnectorTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	toolKeys := make([]string, 0, len(s.tools))
	for key, tool := range s.tools {
		if tool.ConnectorKey == connectorKey {
			toolKeys = append(toolKeys, key)
		}
	}
	sort.Strings(toolKeys)
	tools := make([]ConnectorTool, 0, len(toolKeys))
	for _, key := range toolKeys {
		tools = append(tools, s.tools[key])
	}
	return tools
}

func (s *Service) SetConnectorHealth(workspaceID, connectorKey string, p95LatencyMS int, errorRate float64) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if connectorKey == "" {
		return fmt.Errorf("connector_key is required")
	}
	if p95LatencyMS < 0 {
		return fmt.Errorf("p95_latency_ms must be >= 0")
	}
	if errorRate < 0 || errorRate > 1 {
		return fmt.Errorf("error_rate must be between 0 and 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.connectors[connectorKey]; !ok {
		return fmt.Errorf("connector not found: %s", connectorKey)
	}
	s.health[healthStorageKey(workspaceID, connectorKey)] = ConnectorHealth{
		WorkspaceID:  workspaceID,
		ConnectorKey: connectorKey,
		P95LatencyMS: p95LatencyMS,
		ErrorRate:    errorRate,
		UpdatedAt:    time.Now().UTC(),
	}
	return nil
}

func (s *Service) GetConnectorHealth(workspaceID, connectorKey string) (ConnectorHealth, error) {
	if workspaceID == "" {
		return ConnectorHealth{}, fmt.Errorf("workspace_id is required")
	}
	if connectorKey == "" {
		return ConnectorHealth{}, fmt.Errorf("connector_key is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	health, ok := s.health[healthStorageKey(workspaceID, connectorKey)]
	if !ok {
		return ConnectorHealth{}, fmt.Errorf("connector health not found")
	}
	return health, nil
}

func (s *Service) UpsertUserConnectorSetting(workspaceID, userID, connectorKey string, enabled bool, customRateLimit *int) (UserConnectorSetting, error) {
	if workspaceID == "" {
		return UserConnectorSetting{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return UserConnectorSetting{}, fmt.Errorf("user_id is required")
	}
	if connectorKey == "" {
		return UserConnectorSetting{}, fmt.Errorf("connector_key is required")
	}
	if customRateLimit != nil && *customRateLimit <= 0 {
		return UserConnectorSetting{}, fmt.Errorf("custom_rate_limit must be > 0")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.connectors[connectorKey]; !ok {
		return UserConnectorSetting{}, fmt.Errorf("connector not found: %s", connectorKey)
	}
	setting := UserConnectorSetting{
		WorkspaceID:     workspaceID,
		UserID:          userID,
		ConnectorKey:    connectorKey,
		Enabled:         enabled,
		CustomRateLimit: customRateLimit,
		UpdatedAt:       time.Now().UTC(),
	}
	s.userConnectorSettings[userConnectorSettingsKey(workspaceID, userID, connectorKey)] = setting
	return setting, nil
}

func (s *Service) GetUserConnectorSetting(workspaceID, userID, connectorKey string) (UserConnectorSetting, error) {
	if workspaceID == "" {
		return UserConnectorSetting{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return UserConnectorSetting{}, fmt.Errorf("user_id is required")
	}
	if connectorKey == "" {
		return UserConnectorSetting{}, fmt.Errorf("connector_key is required")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	setting, ok := s.userConnectorSettings[userConnectorSettingsKey(workspaceID, userID, connectorKey)]
	if !ok {
		return UserConnectorSetting{}, fmt.Errorf("user connector setting not found")
	}
	return setting, nil
}

func healthStorageKey(workspaceID, connectorKey string) string {
	return workspaceID + "::" + connectorKey
}

func userConnectorSettingsKey(workspaceID, userID, connectorKey string) string {
	return workspaceID + "::" + userID + "::" + connectorKey
}

func oauthStorageKey(workspaceID, userID, connectorKey string) string {
	return workspaceID + "::" + userID + "::" + connectorKey
}

func (s *Service) EncryptOAuthToken(ctx context.Context, workspaceID, userID, connectorKey, token string) (OAuthEnvelope, error) {
	if workspaceID == "" {
		return OAuthEnvelope{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return OAuthEnvelope{}, fmt.Errorf("user_id is required")
	}
	if connectorKey == "" {
		return OAuthEnvelope{}, fmt.Errorf("connector_key is required")
	}
	if token == "" {
		return OAuthEnvelope{}, fmt.Errorf("token is required")
	}
	if s.keyProvider == nil {
		return OAuthEnvelope{}, fmt.Errorf("key provider is required")
	}

	s.mu.RLock()
	_, connectorExists := s.connectors[connectorKey]
	s.mu.RUnlock()
	if !connectorExists {
		return OAuthEnvelope{}, fmt.Errorf("connector not found: %s", connectorKey)
	}

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
