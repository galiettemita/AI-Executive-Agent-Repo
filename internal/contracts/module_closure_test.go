package contracts

import (
	"path/filepath"
	"testing"
)

func TestGoModuleClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	goModPath := filepath.Join(root, "go.mod")

	assertFileContainsTokens(t, goModPath, []string{
		"module github.com/brevio/brevio",
		"go 1.22",
	})
}
