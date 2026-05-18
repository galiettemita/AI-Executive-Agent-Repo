package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoExclusionClosure enforces the production boundary: demo artifacts
// must not exist in the repository. This test fails CI if demo code reappears.
//
// Governed by: Prompt 1 production boundary decision.
// Covers: REQ-GOV-001 (production boundary enforcement).
func TestDemoExclusionClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Forbidden paths that must not exist in a production-bounded repository.
	forbiddenPaths := []string{
		filepath.Join("apps", "web-demo"),
		filepath.Join("apps", "web-demo", "package.json"),
		filepath.Join("apps", "web-demo", "src"),
		filepath.Join("apps", "web-demo", "src", "brevio-live-demo.jsx"),
	}

	t.Run("forbidden_demo_paths_absent", func(t *testing.T) {
		for _, rel := range forbiddenPaths {
			abs := filepath.Join(root, rel)
			if _, err := os.Stat(abs); err == nil {
				t.Errorf("forbidden demo artifact exists: %s", rel)
			}
		}
	})

	// Scan Go source files for import or reference to demo paths.
	t.Run("no_demo_references_in_go_source", func(t *testing.T) {
		forbidden := []string{"apps/web-demo", "web-demo", "brevio-live-demo"}
		err := filepath.Walk(filepath.Join(root, "internal"), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Errorf("failed to read %s: %v", path, readErr)
				return nil
			}
			content := string(data)
			rel, _ := filepath.Rel(root, path)
			for _, term := range forbidden {
				if strings.Contains(content, term) {
					// Allow references in this test file itself and in test files that
					// assert demo absence (contain "must not exist" or "forbidden").
					if strings.Contains(content, "must not exist") || strings.Contains(content, "forbidden demo artifact") {
						continue
					}
					t.Errorf("%s contains forbidden demo reference %q", rel, term)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed: %v", err)
		}
	})
}
