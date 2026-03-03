package contracts

import (
	"path/filepath"
	"testing"
)

func TestGracefulShutdownClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	token := "ServeWithGracefulShutdown("

	assertFileContainsTokens(t, filepath.Join(root, "cmd", "gateway", "main.go"), []string{token})
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "brain", "main.go"), []string{token})
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "control", "main.go"), []string{token})
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "executor", "main.go"), []string{token})
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "canvas", "main.go"), []string{token})
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"), []string{token})
}
