package config

import "testing"

func TestConfigRegistry(t *testing.T) {
	t.Parallel()

	secrets := RequiredSecretKeys()
	if len(secrets) < 10 {
		t.Fatalf("unexpected secret key count: %d", len(secrets))
	}
	env := RequiredEnvVars()
	if len(env) < 10 {
		t.Fatalf("unexpected env var count: %d", len(env))
	}
}
