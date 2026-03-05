package connectors

import "testing"

func TestCanonicalAuthSecretName(t *testing.T) {
	t.Parallel()

	name, err := CanonicalAuthSecretName("production", "google", "client_id")
	if err != nil {
		t.Fatalf("canonical auth secret name: %v", err)
	}
	if name != "brevio/production/google/client_id" {
		t.Fatalf("unexpected secret name: %s", name)
	}

	name, err = CanonicalAuthSecretName("staging", "samsung-smartthings", "client_secret")
	if err != nil {
		t.Fatalf("canonical auth secret name hyphenated service: %v", err)
	}
	if name != "brevio/staging/samsung-smartthings/client_secret" {
		t.Fatalf("unexpected hyphenated secret name: %s", name)
	}
}

func TestCanonicalAuthSecretNameRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, err := CanonicalAuthSecretName("prod!", "google", "client_id"); err == nil {
		t.Fatalf("expected invalid environment failure")
	}
	if _, err := CanonicalAuthSecretName("production", "google_workspace", "client_id"); err == nil {
		t.Fatalf("expected invalid service failure")
	}
	if _, err := CanonicalAuthSecretName("production", "google", "token"); err == nil {
		t.Fatalf("expected invalid field failure")
	}
}

func TestOAuthRedirectURI(t *testing.T) {
	t.Parallel()

	uri, err := OAuthRedirectURI("google")
	if err != nil {
		t.Fatalf("oauth redirect uri: %v", err)
	}
	if uri != "https://auth.brevio.app/callback/google" {
		t.Fatalf("unexpected oauth redirect uri: %s", uri)
	}

	if _, err := OAuthRedirectURI("bad/service"); err == nil {
		t.Fatalf("expected invalid service segment error")
	}
}

func TestPKCERequiredForOAuthFlows(t *testing.T) {
	t.Parallel()

	if !PKCERequiredForOAuthFlows() {
		t.Fatalf("pkce must be required for oauth flows")
	}
}
