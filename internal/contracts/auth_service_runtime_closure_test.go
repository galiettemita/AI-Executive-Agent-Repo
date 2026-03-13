package contracts

import (
	"path/filepath"
	"testing"
)

func TestAuthServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	serverPath := filepath.Join(root, "deprecated", "brevio-auth", "src", "server.ts")
	indexPath := filepath.Join(root, "deprecated", "brevio-auth", "src", "index.ts")
	readmePath := filepath.Join(root, "deprecated", "brevio-auth", "README.md")

	assertFileContainsTokens(t, serverPath, []string{
		"/health",
		"/health/deep",
		"segments[2] === 'providers'",
		"segments[2] === 'oauth'",
		"authorize",
		"exchange",
		"refresh",
		"segments[0] === 'callback'",
		"code_challenge_method",
		"pkce_required",
		"auth_config_storage",
		"shutdown_start",
		"shutdown_complete",
	})

	assertFileContainsTokens(t, indexPath, []string{
		"startAuthService",
		"installSignalHandlers",
		"service_started",
	})

	assertFileContainsTokens(t, readmePath, []string{
		"POST /api/v1/oauth/{service}/authorize",
		"POST /api/v1/oauth/{service}/exchange",
		"POST /api/v1/oauth/{service}/refresh",
		"GET /callback/{service}",
	})
}
