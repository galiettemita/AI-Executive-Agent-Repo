package contracts

import (
	"path/filepath"
	"testing"
)

func TestDocumentationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredDocs := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "docs", "ARCHITECTURE.md"),
		filepath.Join(root, "docs", "DEVELOPMENT.md"),
		filepath.Join(root, "docs", "DEPLOYMENT.md"),
	}

	for _, path := range requiredDocs {
		assertFileNonEmpty(t, path)
	}
}
