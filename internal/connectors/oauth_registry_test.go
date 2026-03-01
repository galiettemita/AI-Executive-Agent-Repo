package connectors

import "testing"

func TestOAuthProviderRegistryAndConnectorScopes(t *testing.T) {
	t.Parallel()

	registry := OAuthProviderRegistry()
	if len(registry) != 6 {
		t.Fatalf("unexpected oauth provider count: %d", len(registry))
	}
	if registry["google"].DiscoveryURL == "" || !registry["google"].RequiresPKCES256 {
		t.Fatalf("unexpected google oauth config: %+v", registry["google"])
	}
	if registry["github"].AuthorizeURL == "" {
		t.Fatalf("expected github authorize url, got %+v", registry["github"])
	}

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
