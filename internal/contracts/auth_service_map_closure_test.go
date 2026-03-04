package contracts

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"
)

type authServiceMapFile struct {
	OAuthServices []struct {
		Service         string   `yaml:"service"`
		ProviderURL     string   `yaml:"provider_url"`
		SkillsUsing     []string `yaml:"skills_using"`
		RequiredScopes  []string `yaml:"required_scopes"`
		TokenType       string   `yaml:"token_type"`
		RefreshStrategy string   `yaml:"refresh_strategy"`
	} `yaml:"oauth_services"`
	APIKeyServices []struct {
		Service     string   `yaml:"service"`
		KeySource   string   `yaml:"key_source"`
		SkillsUsing []string `yaml:"skills_using"`
		RateLimit   string   `yaml:"rate_limit"`
		CostModel   string   `yaml:"cost_model"`
	} `yaml:"api_key_services"`
	NoAuthServices []struct {
		Service     string   `yaml:"service"`
		SkillsUsing []string `yaml:"skills_using"`
		Notes       string   `yaml:"notes"`
	} `yaml:"no_auth_services"`
	AuthConfigStorage struct {
		SecretNamingConvention []string `yaml:"secret_naming_convention"`
		OAuthRedirectURI       string   `yaml:"oauth_redirect_uri_pattern"`
		PKCERequired           bool     `yaml:"pkce_required"`
	} `yaml:"auth_config_storage"`
}

func TestAuthServiceMapClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "config", "auth-service-map.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read auth service map: %v", err)
	}
	var parsed authServiceMapFile
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse auth service map yaml: %v", err)
	}

	assertOAuthServices(t, parsed.OAuthServices)
	assertAPIKeyServices(t, parsed.APIKeyServices)
	assertNoAuthServices(t, parsed.NoAuthServices)
	assertAuthStorage(t, parsed.AuthConfigStorage)
}

func assertOAuthServices(t *testing.T, items []struct {
	Service         string   `yaml:"service"`
	ProviderURL     string   `yaml:"provider_url"`
	SkillsUsing     []string `yaml:"skills_using"`
	RequiredScopes  []string `yaml:"required_scopes"`
	TokenType       string   `yaml:"token_type"`
	RefreshStrategy string   `yaml:"refresh_strategy"`
}) {
	t.Helper()

	expected := map[string]struct{}{
		"google":              {},
		"spotify":             {},
		"microsoft":           {},
		"todoist":             {},
		"notion":              {},
		"withings":            {},
		"dexcom":              {},
		"ynab":                {},
		"monarch":             {},
		"plaid":               {},
		"ticktick":            {},
		"samsung-smartthings": {},
		"trello":              {},
		"reddit":              {},
		"slack":               {},
	}
	if len(items) != len(expected) {
		t.Fatalf("unexpected oauth service count: got=%d want=%d", len(items), len(expected))
	}

	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Service == "" || item.ProviderURL == "" || item.TokenType == "" || item.RefreshStrategy == "" {
			t.Fatalf("invalid oauth service entry: %+v", item)
		}
		if len(item.RequiredScopes) == 0 || len(item.SkillsUsing) == 0 {
			t.Fatalf("oauth service missing scopes or skills: %+v", item)
		}
		if _, ok := expected[item.Service]; !ok {
			t.Fatalf("unexpected oauth service: %s", item.Service)
		}
		if _, duplicate := seen[item.Service]; duplicate {
			t.Fatalf("duplicate oauth service: %s", item.Service)
		}
		seen[item.Service] = struct{}{}
	}

	for service := range expected {
		if _, ok := seen[service]; !ok {
			t.Fatalf("missing oauth service: %s", service)
		}
	}
}

func assertAPIKeyServices(t *testing.T, items []struct {
	Service     string   `yaml:"service"`
	KeySource   string   `yaml:"key_source"`
	SkillsUsing []string `yaml:"skills_using"`
	RateLimit   string   `yaml:"rate_limit"`
	CostModel   string   `yaml:"cost_model"`
}) {
	t.Helper()

	expected := map[string]struct{}{
		"tavily":         {},
		"exa":            {},
		"firecrawl":      {},
		"serpapi":        {},
		"perplexity":     {},
		"kagi":           {},
		"brave-search":   {},
		"openai":         {},
		"elevenlabs":     {},
		"fal-ai":         {},
		"pollinations":   {},
		"krea":           {},
		"tmdb":           {},
		"opensky":        {},
		"aviationstack":  {},
		"17track":        {},
		"parcel":         {},
		"home-assistant": {},
	}
	if len(items) != len(expected) {
		t.Fatalf("unexpected api key service count: got=%d want=%d", len(items), len(expected))
	}

	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Service == "" || item.KeySource == "" || item.CostModel == "" || item.RateLimit == "" {
			t.Fatalf("invalid api key service entry: %+v", item)
		}
		if len(item.SkillsUsing) == 0 {
			t.Fatalf("api key service missing skills: %+v", item)
		}
		if _, ok := expected[item.Service]; !ok {
			t.Fatalf("unexpected api key service: %s", item.Service)
		}
		if _, duplicate := seen[item.Service]; duplicate {
			t.Fatalf("duplicate api key service: %s", item.Service)
		}
		seen[item.Service] = struct{}{}
	}

	for service := range expected {
		if _, ok := seen[service]; !ok {
			t.Fatalf("missing api key service: %s", service)
		}
	}
}

func assertNoAuthServices(t *testing.T, items []struct {
	Service     string   `yaml:"service"`
	SkillsUsing []string `yaml:"skills_using"`
	Notes       string   `yaml:"notes"`
}) {
	t.Helper()

	expected := map[string]struct{}{
		"bluesky":           {},
		"yahoo-finance":     {},
		"lastfm":            {},
		"trakt":             {},
		"local-macos":       {},
		"internal-llm-only": {},
	}
	if len(items) != len(expected) {
		t.Fatalf("unexpected no-auth service count: got=%d want=%d", len(items), len(expected))
	}

	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Service == "" || item.Notes == "" || len(item.SkillsUsing) == 0 {
			t.Fatalf("invalid no-auth service entry: %+v", item)
		}
		if _, ok := expected[item.Service]; !ok {
			t.Fatalf("unexpected no-auth service: %s", item.Service)
		}
		seen[item.Service] = struct{}{}
	}
	for service := range expected {
		if _, ok := seen[service]; !ok {
			t.Fatalf("missing no-auth service: %s", service)
		}
	}

	for _, item := range items {
		if item.Service != "local-macos" {
			continue
		}
		if len(item.SkillsUsing) != 24 {
			t.Fatalf("local-macos must list exactly 24 skills, got %d", len(item.SkillsUsing))
		}
		expectedLocal := []string{
			"alter-actions", "apple-contacts", "apple-mail", "apple-mail-search", "apple-media", "apple-music", "apple-notes", "apple-notes-skill", "apple-photos", "apple-remind-me", "bear-notes", "calctl", "craft", "gamma", "get-focus-mode", "healthkit-sync", "healthkit-sync-apple", "mole-mac-cleanup", "obsidian", "omnifocus", "shortcuts-generator", "spotify", "things-mac", "voice-wake-say",
		}
		sortedExpected := append([]string(nil), expectedLocal...)
		sortedActual := append([]string(nil), item.SkillsUsing...)
		sort.Strings(sortedExpected)
		sort.Strings(sortedActual)
		if len(sortedExpected) != len(sortedActual) {
			t.Fatalf("local-macos skills size mismatch: got=%d want=%d", len(sortedActual), len(sortedExpected))
		}
		for i := range sortedExpected {
			if sortedExpected[i] != sortedActual[i] {
				t.Fatalf("local-macos skills mismatch at %d: got=%s want=%s", i, sortedActual[i], sortedExpected[i])
			}
		}
	}
}

func assertAuthStorage(t *testing.T, storage struct {
	SecretNamingConvention []string `yaml:"secret_naming_convention"`
	OAuthRedirectURI       string   `yaml:"oauth_redirect_uri_pattern"`
	PKCERequired           bool     `yaml:"pkce_required"`
}) {
	t.Helper()

	if len(storage.SecretNamingConvention) != 2 {
		t.Fatalf("unexpected secret naming convention entries: %d", len(storage.SecretNamingConvention))
	}
	if storage.SecretNamingConvention[0] != "brevio/{environment}/{service}/client_id" {
		t.Fatalf("unexpected client_id secret pattern: %s", storage.SecretNamingConvention[0])
	}
	if storage.SecretNamingConvention[1] != "brevio/{environment}/{service}/client_secret" {
		t.Fatalf("unexpected client_secret secret pattern: %s", storage.SecretNamingConvention[1])
	}
	if storage.OAuthRedirectURI != "https://auth.brevio.app/callback/{service}" {
		t.Fatalf("unexpected oauth redirect uri pattern: %s", storage.OAuthRedirectURI)
	}
	if !storage.PKCERequired {
		t.Fatalf("pkce_required must be true")
	}
}
