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
	"strings"
	"sync"
	"time"

	"github.com/brevio/brevio/internal/audit"
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

type OAuthTokenMetadata struct {
	Provider        string
	ExpiresAt       time.Time
	UpdatedAt       time.Time
	LastRefreshedAt time.Time
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
	oauthRefresh          map[string]OAuthEnvelope
	oauthMeta             map[string]OAuthTokenMetadata
	health                map[string]ConnectorHealth
	userConnectorSettings map[string]UserConnectorSetting
	keyProvider           KeyProvider
	mutationAudit         *audit.Service
	now                   func() time.Time
}

func NewService(keyProvider KeyProvider) *Service {
	return &Service{
		connectors:            map[string]Connector{},
		tools:                 map[string]ConnectorTool{},
		oauth:                 map[string]OAuthEnvelope{},
		oauthRefresh:          map[string]OAuthEnvelope{},
		oauthMeta:             map[string]OAuthTokenMetadata{},
		health:                map[string]ConnectorHealth{},
		userConnectorSettings: map[string]UserConnectorSetting{},
		keyProvider:           keyProvider,
		now:                   func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetNow(now func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now == nil {
		s.now = func() time.Time { return time.Now().UTC() }
		return
	}
	s.now = now
}

func (s *Service) SetMutationAudit(auditSvc *audit.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mutationAudit = auditSvc
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
	// If MCP_BASE_URL is set, override placeholder URLs (.unconfigured.local).
	if base := strings.TrimSpace(os.Getenv("MCP_BASE_URL")); base != "" {
		base = strings.TrimRight(base, "/")
		for i := range seed.Connectors {
			if strings.Contains(seed.Connectors[i].MCPServerURL, ".unconfigured.local") {
				seed.Connectors[i].MCPServerURL = base + "/" + seed.Connectors[i].Key
			}
		}
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

// SeedToRepository loads the seed file and upserts all connectors and tools
// into the given ConnectorRegistryRepository (DB-backed).
func (s *Service) SeedToRepository(ctx context.Context, repo ConnectorRegistryRepository) (int, int, error) {
	s.mu.RLock()
	connectors := make([]Connector, 0, len(s.connectors))
	for _, c := range s.connectors {
		connectors = append(connectors, c)
	}
	tools := make([]ConnectorTool, 0, len(s.tools))
	for _, t := range s.tools {
		tools = append(tools, t)
	}
	s.mu.RUnlock()

	for _, c := range connectors {
		if err := repo.UpsertConnector(ctx, c); err != nil {
			return 0, 0, fmt.Errorf("seed connector %s: %w", c.Key, err)
		}
	}
	for _, t := range tools {
		if err := repo.UpsertTool(ctx, t); err != nil {
			return 0, 0, fmt.Errorf("seed tool %s: %w", t.ToolKey, err)
		}
	}
	return len(connectors), len(tools), nil
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

// ListAllTools returns all tools sorted by tool_key.
func (s *Service) ListAllTools() []ConnectorTool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.tools))
	for key := range s.tools {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	tools := make([]ConnectorTool, 0, len(keys))
	for _, key := range keys {
		tools = append(tools, s.tools[key])
	}
	return tools
}

// ToolKeys returns a sorted list of all tool keys.
func (s *Service) ToolKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.tools))
	for key := range s.tools {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// HasTool returns true if the tool_key exists in the registry.
func (s *Service) HasTool(toolKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.tools[toolKey]
	return ok
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

	if err := s.ensureConnectorExists(connectorKey); err != nil {
		return OAuthEnvelope{}, err
	}

	envelope, err := s.encryptWithCurrentKey(ctx, token)
	if err != nil {
		return OAuthEnvelope{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := oauthStorageKey(workspaceID, userID, connectorKey)
	s.oauth[key] = envelope
	if _, ok := s.oauthMeta[key]; !ok {
		s.oauthMeta[key] = OAuthTokenMetadata{
			Provider:  connectorKey,
			UpdatedAt: s.now(),
		}
	}
	return envelope, nil
}

func (s *Service) StoreOAuthTokenSet(
	ctx context.Context,
	workspaceID, userID, connectorKey, provider, accessToken, refreshToken string,
	expiresAt time.Time,
) (OAuthEnvelope, OAuthTokenMetadata, error) {
	if workspaceID == "" {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, fmt.Errorf("user_id is required")
	}
	if connectorKey == "" {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, fmt.Errorf("connector_key is required")
	}
	if strings.TrimSpace(provider) == "" {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, fmt.Errorf("provider is required")
	}
	if strings.TrimSpace(accessToken) == "" {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, fmt.Errorf("access_token is required")
	}
	if err := s.ensureConnectorExists(connectorKey); err != nil {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, err
	}

	accessEnvelope, err := s.encryptWithCurrentKey(ctx, accessToken)
	if err != nil {
		return OAuthEnvelope{}, OAuthTokenMetadata{}, err
	}
	var refreshEnvelope OAuthEnvelope
	if refreshToken != "" {
		refreshEnvelope, err = s.encryptWithCurrentKey(ctx, refreshToken)
		if err != nil {
			return OAuthEnvelope{}, OAuthTokenMetadata{}, err
		}
	}

	metadata := OAuthTokenMetadata{
		Provider:  provider,
		UpdatedAt: s.now(),
	}
	if !expiresAt.IsZero() {
		metadata.ExpiresAt = expiresAt.UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := oauthStorageKey(workspaceID, userID, connectorKey)
	s.oauth[key] = accessEnvelope
	if refreshToken != "" {
		s.oauthRefresh[key] = refreshEnvelope
	} else {
		delete(s.oauthRefresh, key)
	}
	s.oauthMeta[key] = metadata
	return accessEnvelope, metadata, nil
}

func (s *Service) GetOAuthTokenMetadata(workspaceID, userID, connectorKey string) (OAuthTokenMetadata, error) {
	key := oauthStorageKey(workspaceID, userID, connectorKey)
	s.mu.RLock()
	defer s.mu.RUnlock()
	metadata, ok := s.oauthMeta[key]
	if !ok {
		return OAuthTokenMetadata{}, fmt.Errorf("oauth token metadata not found")
	}
	return metadata, nil
}

func (s *Service) GetOAuthTokenForUse(
	ctx context.Context,
	workspaceID, userID, connectorKey string,
	refreshWindow time.Duration,
	refresher func(refreshToken string) (newAccessToken string, newExpiresAt time.Time, err error),
) (string, OAuthTokenMetadata, error) {
	key := oauthStorageKey(workspaceID, userID, connectorKey)
	s.mu.RLock()
	accessEnvelope, accessOK := s.oauth[key]
	metadata, metaOK := s.oauthMeta[key]
	refreshEnvelope, refreshOK := s.oauthRefresh[key]
	now := s.now()
	s.mu.RUnlock()
	if !accessOK {
		return "", OAuthTokenMetadata{}, fmt.Errorf("oauth token not found")
	}
	if !metaOK {
		metadata = OAuthTokenMetadata{
			Provider:  connectorKey,
			UpdatedAt: now,
		}
	}

	accessToken, err := s.decryptEnvelope(ctx, accessEnvelope)
	if err != nil {
		return "", OAuthTokenMetadata{}, err
	}
	if refreshWindow <= 0 || metadata.ExpiresAt.IsZero() || metadata.ExpiresAt.After(now.Add(refreshWindow)) {
		return accessToken, metadata, nil
	}
	if !refreshOK {
		return "", metadata, fmt.Errorf("refresh token not found")
	}
	if refresher == nil {
		return "", metadata, fmt.Errorf("refresher is required when token is expiring")
	}

	refreshToken, err := s.decryptEnvelope(ctx, refreshEnvelope)
	if err != nil {
		return "", metadata, err
	}
	newAccessToken, newExpiresAt, err := refresher(refreshToken)
	if err != nil {
		return "", metadata, err
	}
	if strings.TrimSpace(newAccessToken) == "" {
		return "", metadata, fmt.Errorf("refreshed access token is empty")
	}
	if newExpiresAt.IsZero() || !newExpiresAt.After(now) {
		return "", metadata, fmt.Errorf("refreshed token expiry must be in the future")
	}

	newAccessEnvelope, err := s.encryptWithCurrentKey(ctx, newAccessToken)
	if err != nil {
		return "", metadata, err
	}
	previousExpiresAt := metadata.ExpiresAt
	metadata.ExpiresAt = newExpiresAt.UTC()
	metadata.UpdatedAt = now
	metadata.LastRefreshedAt = now
	if strings.TrimSpace(metadata.Provider) == "" {
		metadata.Provider = connectorKey
	}

	s.mu.Lock()
	s.oauth[key] = newAccessEnvelope
	s.oauthMeta[key] = metadata
	s.mu.Unlock()
	s.appendOAuthRefreshMutationAudit(workspaceID, userID, connectorKey, previousExpiresAt, metadata)
	return newAccessToken, metadata, nil
}

func (s *Service) DecryptOAuthToken(ctx context.Context, workspaceID, userID, connectorKey string) (string, OAuthEnvelope, error) {
	s.mu.RLock()
	envelope, ok := s.oauth[oauthStorageKey(workspaceID, userID, connectorKey)]
	s.mu.RUnlock()
	if !ok {
		return "", OAuthEnvelope{}, fmt.Errorf("oauth token not found")
	}

	plaintext, err := s.decryptEnvelope(ctx, envelope)
	if err != nil {
		return "", OAuthEnvelope{}, err
	}
	return plaintext, envelope, nil
}

func (s *Service) appendOAuthRefreshMutationAudit(workspaceID, userID, connectorKey string, beforeExpiresAt time.Time, metadata OAuthTokenMetadata) {
	s.mu.RLock()
	mutationAudit := s.mutationAudit
	s.mu.RUnlock()
	if mutationAudit == nil {
		return
	}
	before := map[string]any{}
	if !beforeExpiresAt.IsZero() {
		before["expires_at"] = beforeExpiresAt.UTC().Format(time.RFC3339)
	}
	after := map[string]any{
		"provider": metadata.Provider,
	}
	if !metadata.ExpiresAt.IsZero() {
		after["expires_at"] = metadata.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if !metadata.LastRefreshedAt.IsZero() {
		after["last_refreshed_at"] = metadata.LastRefreshedAt.UTC().Format(time.RFC3339)
	}
	mutationAudit.AppendMutation(audit.MutationInput{
		WorkspaceID: workspaceID,
		Actor:       userID,
		Action:      "oauth.token.refresh",
		Resource:    "oauth:" + connectorKey,
		Before:      before,
		After:       after,
	})
}

func (s *Service) ensureConnectorExists(connectorKey string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.connectors[connectorKey]; !ok {
		return fmt.Errorf("connector not found: %s", connectorKey)
	}
	return nil
}

func (s *Service) encryptWithCurrentKey(ctx context.Context, token string) (OAuthEnvelope, error) {
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
	return OAuthEnvelope{
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce:       base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:  version,
		EncryptedAt: s.now(),
	}, nil
}

func (s *Service) decryptEnvelope(ctx context.Context, envelope OAuthEnvelope) (string, error) {
	key, err := s.keyProvider.KeyByVersion(ctx, envelope.KeyVersion)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce, err := base64.StdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return "", err
	}
	ciphertext, err := base64.StdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return "", err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
