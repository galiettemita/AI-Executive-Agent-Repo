package connectors

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/audit"
)

func TestSeedLoaderPopulatesAtLeast40Connectors(t *testing.T) {
	t.Parallel()

	svc := newSeededService(t)
	if svc.ConnectorCount() < 40 {
		t.Fatalf("expected at least 40 connectors, got %d", svc.ConnectorCount())
	}
	if svc.ToolCount() == 0 {
		t.Fatal("expected tools to be loaded")
	}
}

func TestSeedLoaderSupportsJSONFile(t *testing.T) {
	t.Parallel()

	keyProvider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(keyProvider)

	jsonSeed := seedFile{
		Connectors: []Connector{{
			Key:          "json_connector",
			Domain:       "documents",
			RiskLevel:    "LOW",
			DataClass:    "internal",
			MCPServerURL: "https://mcp.test.internal/json_connector",
		}},
		Tools: []ConnectorTool{{
			ConnectorKey:  "json_connector",
			ToolKey:       "json_connector.parse",
			Write:         false,
			Reversible:    false,
			AutonomyFloor: "A1",
		}},
	}
	content, err := json.Marshal(jsonSeed)
	if err != nil {
		t.Fatalf("marshal json seed: %v", err)
	}

	seedPath := filepath.Join(t.TempDir(), "connectors.json")
	if err := os.WriteFile(seedPath, content, 0o600); err != nil {
		t.Fatalf("write json seed: %v", err)
	}
	if err := svc.LoadSeedFile(seedPath); err != nil {
		t.Fatalf("load json seed file: %v", err)
	}

	if svc.ConnectorCount() != 1 {
		t.Fatalf("expected exactly one connector from json seed, got %d", svc.ConnectorCount())
	}
	tools := svc.ListToolsByConnector("json_connector")
	if len(tools) != 1 || tools[0].ToolKey != "json_connector.parse" {
		t.Fatalf("unexpected tools from json seed: %+v", tools)
	}
}

func TestOAuthEncryptDecryptRoundTripAndKeyVersion(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(provider)
	if err := svc.LoadSeedFile(filepath.Join("seeds", "connectors.yaml")); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	ctx := context.Background()

	envelope, err := svc.EncryptOAuthToken(ctx, "ws_1", "user_1", "google_gmail", "secret-token")
	if err != nil {
		t.Fatalf("encrypt token: %v", err)
	}
	if envelope.KeyVersion != "v1" {
		t.Fatalf("unexpected key version: %s", envelope.KeyVersion)
	}

	plaintext, restoredEnvelope, err := svc.DecryptOAuthToken(ctx, "ws_1", "user_1", "google_gmail")
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
	plaintextAfterRotate, restoredEnvelopeAfterRotate, err := svc.DecryptOAuthToken(ctx, "ws_1", "user_1", "google_gmail")
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

func TestOAuthTokenSetSafeRefreshWithinWindow(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(provider)
	if err := svc.LoadSeedFile(filepath.Join("seeds", "connectors.yaml")); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	mutationAudit := audit.NewService()
	svc.SetMutationAudit(mutationAudit)
	base := time.Date(2026, time.March, 1, 13, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return base })

	_, storedMeta, err := svc.StoreOAuthTokenSet(
		context.Background(),
		"ws_refresh",
		"user_refresh",
		"google_gmail",
		"google_gmail_mcp",
		"access_old",
		"refresh_secret",
		base.Add(5*time.Minute),
	)
	if err != nil {
		t.Fatalf("store oauth token set: %v", err)
	}
	if storedMeta.Provider != "google_gmail_mcp" {
		t.Fatalf("unexpected provider in stored metadata: %+v", storedMeta)
	}

	refreshedToken, refreshedMeta, err := svc.GetOAuthTokenForUse(
		context.Background(),
		"ws_refresh",
		"user_refresh",
		"google_gmail",
		10*time.Minute,
		func(refreshToken string) (string, time.Time, error) {
			if refreshToken != "refresh_secret" {
				t.Fatalf("unexpected refresh token value: %s", refreshToken)
			}
			return "access_new", base.Add(2 * time.Hour), nil
		},
	)
	if err != nil {
		t.Fatalf("get oauth token for use with refresh: %v", err)
	}
	if refreshedToken != "access_new" {
		t.Fatalf("unexpected refreshed token: %s", refreshedToken)
	}
	if refreshedMeta.LastRefreshedAt.IsZero() {
		t.Fatalf("expected non-zero last_refreshed_at after refresh: %+v", refreshedMeta)
	}
	if mutationAudit.Count("ws_refresh") != 1 {
		t.Fatalf("expected one oauth mutation audit entry, got=%d", mutationAudit.Count("ws_refresh"))
	}
	entries := mutationAudit.ListMutations("ws_refresh")
	if len(entries) != 1 || entries[0].Action != "oauth.token.refresh" {
		t.Fatalf("unexpected oauth mutation audit entries: %+v", entries)
	}
}

func TestOAuthTokenSetNoRefreshOutsideWindow(t *testing.T) {
	t.Parallel()

	provider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(provider)
	if err := svc.LoadSeedFile(filepath.Join("seeds", "connectors.yaml")); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	base := time.Date(2026, time.March, 1, 13, 0, 0, 0, time.UTC)
	svc.SetNow(func() time.Time { return base })

	if _, _, err := svc.StoreOAuthTokenSet(
		context.Background(),
		"ws_no_refresh",
		"user_no_refresh",
		"google_gmail",
		"google_gmail_mcp",
		"access_current",
		"refresh_current",
		base.Add(2*time.Hour),
	); err != nil {
		t.Fatalf("store oauth token set: %v", err)
	}

	token, meta, err := svc.GetOAuthTokenForUse(
		context.Background(),
		"ws_no_refresh",
		"user_no_refresh",
		"google_gmail",
		10*time.Minute,
		func(refreshToken string) (string, time.Time, error) {
			t.Fatalf("refresher must not run outside refresh window")
			return "", time.Time{}, nil
		},
	)
	if err != nil {
		t.Fatalf("get oauth token for use outside refresh window: %v", err)
	}
	if token != "access_current" {
		t.Fatalf("unexpected access token without refresh: %s", token)
	}
	if !meta.LastRefreshedAt.IsZero() {
		t.Fatalf("did not expect refresh metadata update: %+v", meta)
	}
}

func TestUserConnectorSettingsLifecycle(t *testing.T) {
	t.Parallel()

	svc := newSeededService(t)
	rateLimit := 120
	stored, err := svc.UpsertUserConnectorSetting("ws_100", "user_100", "google_gmail", true, &rateLimit)
	if err != nil {
		t.Fatalf("upsert user connector setting: %v", err)
	}
	if stored.CustomRateLimit == nil || *stored.CustomRateLimit != 120 {
		t.Fatalf("unexpected custom rate limit: %+v", stored)
	}

	fetched, err := svc.GetUserConnectorSetting("ws_100", "user_100", "google_gmail")
	if err != nil {
		t.Fatalf("get user connector setting: %v", err)
	}
	if !fetched.Enabled {
		t.Fatal("expected connector setting to remain enabled")
	}

	stored, err = svc.UpsertUserConnectorSetting("ws_100", "user_100", "google_gmail", false, nil)
	if err != nil {
		t.Fatalf("upsert setting disabled: %v", err)
	}
	if stored.Enabled {
		t.Fatal("expected connector setting to be disabled")
	}
}

func TestConnectorHealthTrackingLifecycle(t *testing.T) {
	t.Parallel()

	svc := newSeededService(t)
	if err := svc.SetConnectorHealth("ws_200", "slack", 250, 0.02); err != nil {
		t.Fatalf("set connector health: %v", err)
	}
	health, err := svc.GetConnectorHealth("ws_200", "slack")
	if err != nil {
		t.Fatalf("get connector health: %v", err)
	}
	if health.P95LatencyMS != 250 || health.ErrorRate != 0.02 {
		t.Fatalf("unexpected connector health: %+v", health)
	}
}

func TestConnectorSeedIntegrityAndToolKeyFormat(t *testing.T) {
	t.Parallel()

	svc := newSeededService(t)
	connectorKeyPattern := regexp.MustCompile(`^[a-z0-9_]+$`)
	toolKeyPattern := regexp.MustCompile(`^[a-z0-9_]+\.[a-z0-9_]+$`)

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
		if _, ok := validAutonomyFloors[tool.AutonomyFloor]; !ok {
			t.Fatalf("invalid autonomy floor for tool %s: %s", toolKey, tool.AutonomyFloor)
		}
		if tool.InputSchema == nil || tool.OutputSchema == nil {
			t.Fatalf("tool schema payloads must be non-nil maps: %s", toolKey)
		}
	}
}

func newSeededService(t *testing.T) *Service {
	t.Helper()
	keyProvider := NewInMemoryKeyProvider("v1", []byte("0123456789abcdef0123456789abcdef"))
	svc := NewService(keyProvider)
	if err := svc.LoadSeedFile(filepath.Join("seeds", "connectors.yaml")); err != nil {
		t.Fatalf("load seed file: %v", err)
	}
	return svc
}
