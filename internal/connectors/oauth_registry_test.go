package connectors

import "testing"

func TestOAuthProviderRegistryCoverage(t *testing.T) {
	t.Parallel()

	registry := OAuthProviderRegistry()
	if len(registry) != 15 {
		t.Fatalf("unexpected oauth provider count: %d", len(registry))
	}

	requiredKeys := []string{
		"google",
		"spotify",
		"microsoft",
		"todoist",
		"notion",
		"withings",
		"dexcom",
		"ynab",
		"monarch",
		"plaid",
		"ticktick",
		"samsung-smartthings",
		"trello",
		"reddit",
		"slack",
	}
	for _, key := range requiredKeys {
		config, ok := registry[key]
		if !ok {
			t.Fatalf("missing oauth provider config: %s", key)
		}
		if config.ProviderURL == "" || config.AuthorizeURL == "" {
			t.Fatalf("provider urls must be set for %s: %+v", key, config)
		}
		if len(config.RequiredScopes) == 0 {
			t.Fatalf("required scopes must be set for %s", key)
		}
		if !config.RequiresPKCES256 {
			t.Fatalf("pkce must be required for %s", key)
		}
	}
}

func TestOAuthScopesForConnector(t *testing.T) {
	t.Parallel()

	scopes := OAuthScopesForConnector("google", "google_calendar")
	expectedScope := "https://www.googleapis.com/auth/calendar"
	found := false
	for _, scope := range scopes {
		if scope == expectedScope {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected connector additional scope %q in %v", expectedScope, scopes)
	}
}

func TestServiceAuthMapsCoverage(t *testing.T) {
	t.Parallel()

	apiKeyRegistry := APIKeyServiceRegistry()
	if len(apiKeyRegistry) != 18 {
		t.Fatalf("unexpected api-key service count: %d", len(apiKeyRegistry))
	}
	for _, key := range []string{"tavily", "openai", "tmdb", "17track", "home-assistant-api-key"} {
		config, ok := apiKeyRegistry[key]
		if !ok {
			t.Fatalf("missing api-key service config: %s", key)
		}
		if config.KeySource == "" || config.CostModel == "" || len(config.SkillsUsing) == 0 {
			t.Fatalf("incomplete api-key service config for %s: %+v", key, config)
		}
	}

	noAuthRegistry := NoAuthServiceRegistry()
	if len(noAuthRegistry) != 6 {
		t.Fatalf("unexpected no-auth service count: %d", len(noAuthRegistry))
	}
	for _, key := range []string{"bluesky", "yahoo-finance", "lastfm", "trakt", "local-macos", "internal-llm-only"} {
		config, ok := noAuthRegistry[key]
		if !ok {
			t.Fatalf("missing no-auth service config: %s", key)
		}
		if config.Notes == "" || len(config.SkillsUsing) == 0 {
			t.Fatalf("incomplete no-auth service config for %s: %+v", key, config)
		}
	}
}
