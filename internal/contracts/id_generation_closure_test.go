package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUUIDGenerationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	internalDir := filepath.Join(root, "internal")

	err := filepath.WalkDir(internalDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		content := string(body)
		if strings.Contains(content, "uuid.New(") || strings.Contains(content, "uuid.NewString(") {
			t.Fatalf("runtime file uses non-v7 uuid generator: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan internal directory: %v", err)
	}

	assertFileContainsTokens(t, filepath.Join(root, "internal", "identity", "service.go"), []string{"determinism.NewUUIDv7()"})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "delegation", "service.go"), []string{"determinism.NewUUIDv7()"})
}
