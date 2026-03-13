package gateway

import "testing"

func TestLoadEnvConfigUnsetEnvWithoutOverrideErrors(t *testing.T) {
	t.Parallel()

	_, err := LoadEnvConfig(func(string) string { return "" })
	if err == nil {
		t.Fatal("expected error when BREVIO_ENV is unset without ALLOW_DEFAULT_BREVIO_ENV")
	}
}

func TestLoadEnvConfigUnsetEnvWithOverrideDefaultsToLocal(t *testing.T) {
	t.Parallel()

	cfg, err := LoadEnvConfig(func(key string) string {
		if key == "ALLOW_DEFAULT_BREVIO_ENV" {
			return "1"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("load env config: %v", err)
	}
	if cfg.Environment != "local" {
		t.Fatalf("unexpected environment: %s", cfg.Environment)
	}
	if cfg.WebhookSecret != "dev-secret" {
		t.Fatalf("unexpected webhook secret default: %s", cfg.WebhookSecret)
	}
	if cfg.IMessageWebhookAPIKey != "dev-imessage-key" {
		t.Fatalf("unexpected imessage key default: %s", cfg.IMessageWebhookAPIKey)
	}
	if cfg.ListenAddr != ":18080" {
		t.Fatalf("unexpected listen addr default: %s", cfg.ListenAddr)
	}
}

func TestLoadEnvConfigProductionRequiresSecrets(t *testing.T) {
	t.Parallel()

	_, err := LoadEnvConfig(func(key string) string {
		if key == "BREVIO_ENV" {
			return "production"
		}
		return ""
	})
	if err == nil {
		t.Fatal("expected error for missing required production secrets")
	}
}

func TestLoadEnvConfigProductionWithSecrets(t *testing.T) {
	t.Parallel()

	cfg, err := LoadEnvConfig(func(key string) string {
		switch key {
		case "BREVIO_ENV":
			return "production"
		case "GATEWAY_WEBHOOK_SECRET":
			return "prod-secret"
		case "IMESSAGE_WEBHOOK_API_KEY":
			return "prod-imsg-key"
		case "GATEWAY_LISTEN_ADDR":
			return ":9443"
		case "SERVICE_VERSION":
			return "1.2.3"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("load env config: %v", err)
	}
	if cfg.WebhookSecret != "prod-secret" || cfg.IMessageWebhookAPIKey != "prod-imsg-key" {
		t.Fatalf("unexpected production secrets: %+v", cfg)
	}
	if cfg.ListenAddr != ":9443" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.ServiceVersion != "1.2.3" {
		t.Fatalf("unexpected service version: %s", cfg.ServiceVersion)
	}
}
