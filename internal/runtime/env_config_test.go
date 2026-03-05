package runtime

import "testing"

func TestLoadServiceEnvConfigLocalDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadServiceEnvConfig(func(string) string { return "" }, ServiceEnvOptions{
		ServiceName:       "brain",
		DefaultListenAddr: ":18081",
	})
	if err != nil {
		t.Fatalf("load service env config: %v", err)
	}
	if cfg.Environment != "local" {
		t.Fatalf("unexpected environment: %s", cfg.Environment)
	}
	if cfg.ListenAddr != ":18081" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.ServiceVersion != "0.1.0" {
		t.Fatalf("unexpected service version: %s", cfg.ServiceVersion)
	}
}

func TestLoadServiceEnvConfigNonLocalRequiresKeys(t *testing.T) {
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

func TestResolveSecretWithLocalDefault(t *testing.T) {
	t.Parallel()

	got, err := ResolveSecretWithLocalDefault(func(string) string { return "" }, "CONTROL_APP_SECRET", "local", "dev-secret")
	if err != nil {
		t.Fatalf("resolve secret: %v", err)
	}
	if got != "dev-secret" {
		t.Fatalf("unexpected local default: %s", got)
	}

	_, err = ResolveSecretWithLocalDefault(func(string) string { return "" }, "CONTROL_APP_SECRET", "production", "dev-secret")
	if err == nil {
		t.Fatal("expected production error for missing secret")
	}
}
