package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceCommentHygieneClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	targetDirs := []string{
		"cmd",
		"internal",
		"db/migrations",
		"api/openapi",
		"policies",
		"terraform",
		"helm",
	}
	disallowed := []string{"TODO", "FIXME", "HACK", "DEPRECATED"}

	for _, dir := range targetDirs {
		dirPath := filepath.Join(root, dir)
		err := filepath.WalkDir(dirPath, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if d.Name() == ".terraform" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"contracts"+string(filepath.Separator)) {
				return nil
			}

			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(body)
			for _, token := range disallowed {
				if strings.Contains(content, token) {
					t.Fatalf("disallowed marker %q found in %s", token, path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan source dir %s: %v", dirPath, err)
		}
	}
}
