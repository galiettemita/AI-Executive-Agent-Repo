package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTypeScriptNoAnyInProductionClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	targetDirs := []string{"packages", "services", "edge"}
	disallowedPatterns := []*regexp.Regexp{
		regexp.MustCompile(`:\s*any\b`),
		regexp.MustCompile(`<any>`),
		regexp.MustCompile(`\bas\s+any\b`),
		regexp.MustCompile(`\b(Array|Promise)<\s*any\s*>`),
	}

	for _, rel := range targetDirs {
		base := filepath.Join(root, rel)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}

			if d.IsDir() {
				switch d.Name() {
				case "node_modules", "__tests__", "dist", "build", ".turbo", ".next":
					return filepath.SkipDir
				}
				return nil
			}

			if filepath.Ext(path) != ".ts" {
				return nil
			}
			if strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".spec.ts") {
				return nil
			}

			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			content := string(body)
			for _, pattern := range disallowedPatterns {
				if pattern.MatchString(content) {
					t.Fatalf("disallowed TypeScript any pattern %q in %s", pattern.String(), path)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan TypeScript tree %s: %v", base, err)
		}
	}
}
