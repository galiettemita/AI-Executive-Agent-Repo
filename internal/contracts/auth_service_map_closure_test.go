package contracts

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

type authServiceMapFile struct {
	OAuthServices []struct {
		Service        string   `yaml:"service"`
		ProviderURL    string   `yaml:"provider_url"`
		SkillsUsing    []string `yaml:"skills_using"`
		RequiredScopes []string `yaml:"required_scopes"`
		TokenType      string   `yaml:"token_type"`
	} `yaml:"oauth_services"`
	APIKeyServices []struct {
		Service     string   `yaml:"service"`
		KeySource   string   `yaml:"key_source"`
		SkillsUsing []string `yaml:"skills_using"`
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

	if len(parsed.OAuthServices) != 15 {
		t.Fatalf("unexpected oauth service count: %d", len(parsed.OAuthServices))
	}
	if len(parsed.APIKeyServices) != 18 {
		t.Fatalf("unexpected api key service count: %d", len(parsed.APIKeyServices))
	}
	if len(parsed.NoAuthServices) != 6 {
		t.Fatalf("unexpected no-auth service count: %d", len(parsed.NoAuthServices))
	}

	for _, item := range parsed.OAuthServices {
		if item.Service == "" || item.ProviderURL == "" || item.TokenType == "" {
			t.Fatalf("invalid oauth service entry: %+v", item)
		}
		if len(item.RequiredScopes) == 0 || len(item.SkillsUsing) == 0 {
			t.Fatalf("oauth service missing scopes or skills: %+v", item)
		}
	}
	for _, item := range parsed.APIKeyServices {
		if item.Service == "" || item.KeySource == "" || item.CostModel == "" {
			t.Fatalf("invalid api key service entry: %+v", item)
		}
		if len(item.SkillsUsing) == 0 {
			t.Fatalf("api key service missing skills: %+v", item)
		}
	}
	for _, item := range parsed.NoAuthServices {
		if item.Service == "" || item.Notes == "" || len(item.SkillsUsing) == 0 {
			t.Fatalf("invalid no-auth service entry: %+v", item)
		}
	}

	if len(parsed.AuthConfigStorage.SecretNamingConvention) != 2 {
		t.Fatalf("unexpected secret naming convention entries: %d", len(parsed.AuthConfigStorage.SecretNamingConvention))
	}
	if parsed.AuthConfigStorage.OAuthRedirectURI == "" {
		t.Fatalf("missing oauth redirect uri pattern")
	}
	if !parsed.AuthConfigStorage.PKCERequired {
		t.Fatalf("pkce_required must be true")
	}
}
