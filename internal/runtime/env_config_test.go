package runtime

import "testing"

func TestLoadServiceEnvConfigUnsetEnvWithoutOverrideErrors(t *testing.T) {
	t.Parallel()

	_, err := LoadServiceEnvConfig(func(string) string { return "" }, ServiceEnvOptions{
		ServiceName:       "brain",
		DefaultListenAddr: ":18081",
	})
	if err == nil {
		t.Fatal("expected error when BREVIO_ENV is unset without ALLOW_DEFAULT_BREVIO_ENV")
	}
}

func TestLoadServiceEnvConfigUnsetEnvWithOverrideDefaultsToLocal(t *testing.T) {
	t.Parallel()

	cfg, err := LoadServiceEnvConfig(func(key string) string {
		if key == "ALLOW_DEFAULT_BREVIO_ENV" {
			return "1"
		}
		return ""
	}, ServiceEnvOptions{
		ServiceName:       "brain",
		DefaultListenAddr: ":18081",
	})
	if err != nil {
		t.Fatalf("load service env config: %v", err)
	}
	if cfg.Environment != "local" {
		t.Fatalf("expected local environment, got: %s", cfg.Environment)
	}
	if cfg.ListenAddr != ":18081" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.ServiceVersion != "0.1.0" {
		t.Fatalf("unexpected service version: %s", cfg.ServiceVersion)
	}
}

func TestLoadServiceEnvConfigExplicitProdNotLocal(t *testing.T) {
	t.Parallel()

	_, err := LoadServiceEnvConfig(func(key string) string {
		if key == "BREVIO_ENV" {
			return "production"
		}
		return ""
	}, ServiceEnvOptions{
		ServiceName:         "brain",
		DefaultListenAddr:   ":18081",
		RequiredNonLocalEnv: []string{"DATABASE_URL"},
	})
	if err == nil {
		t.Fatal("expected error for missing required non-local env key")
	}
}

func TestLoadServiceEnvConfigExplicitStagingNotLocal(t *testing.T) {
	t.Parallel()

	_, err := LoadServiceEnvConfig(func(key string) string {
		switch key {
		case "BREVIO_ENV":
			return "staging"
		case "DATABASE_URL":
			return "postgres://staging:5432/db"
		default:
			return ""
		}
	}, ServiceEnvOptions{
		ServiceName:         "brain",
		DefaultListenAddr:   ":18081",
		RequiredNonLocalEnv: []string{"DATABASE_URL"},
	})
	if err != nil {
		t.Fatalf("unexpected error for staging with required keys: %v", err)
	}
}

func TestResolveSecretWithLocalDefaultRequiresOverride(t *testing.T) {
	t.Parallel()

	_, err := ResolveSecretWithLocalDefault(
		func(string) string { return "" },
		"CONTROL_APP_SECRET", "local", "dev-secret",
	)
	if err == nil {
		t.Fatal("expected error when local env lacks ALLOW_DEFAULT_BREVIO_ENV override")
	}
}

func TestResolveSecretWithLocalDefaultWithOverride(t *testing.T) {
	t.Parallel()

	got, err := ResolveSecretWithLocalDefault(
		func(key string) string {
			if key == "ALLOW_DEFAULT_BREVIO_ENV" {
				return "1"
			}
			return ""
		},
		"CONTROL_APP_SECRET", "local", "dev-secret",
	)
	if err != nil {
		t.Fatalf("resolve secret: %v", err)
	}
	if got != "dev-secret" {
		t.Fatalf("unexpected local default: %s", got)
	}
}

func TestResolveSecretProductionRequiresSecret(t *testing.T) {
	t.Parallel()

	_, err := ResolveSecretWithLocalDefault(
		func(string) string { return "" },
		"CONTROL_APP_SECRET", "production", "dev-secret",
	)
	if err == nil {
		t.Fatal("expected production error for missing secret")
	}
}

func TestResolveSecretExplicitValueAlwaysUsed(t *testing.T) {
	t.Parallel()

	got, err := ResolveSecretWithLocalDefault(
		func(key string) string {
			if key == "CONTROL_APP_SECRET" {
				return "real-secret"
			}
			return ""
		},
		"CONTROL_APP_SECRET", "production", "dev-secret",
	)
	if err != nil {
		t.Fatalf("resolve secret: %v", err)
	}
	if got != "real-secret" {
		t.Fatalf("expected real-secret, got: %s", got)
	}
}
