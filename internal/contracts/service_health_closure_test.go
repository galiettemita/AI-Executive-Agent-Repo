package contracts

import (
	"path/filepath"
	"testing"
)

func TestServiceHealthEndpointClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	requiredHealthTokens := []string{
		"GET /healthz/ready",
		"GET /healthz/live",
	}

	assertFileContainsTokens(t, filepath.Join(root, "cmd", "brain", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "executor", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "server.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "mux.go"), requiredHealthTokens)
	assertFileContainsTokens(t, filepath.Join(root, "internal", "canvas", "service.go"), requiredHealthTokens)
}
