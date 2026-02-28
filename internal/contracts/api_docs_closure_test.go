package contracts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedAPIDocumentationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	apiDocPath := filepath.Join(root, "docs", "API_REFERENCE.md")
	assertFileNonEmpty(t, apiDocPath)
	assertFileContainsTokens(t, apiDocPath, []string{
		"# API Reference",
		"Generated from `api/openapi/v9.yaml`.",
		"OpenAPI SHA-256:",
		"Total endpoints:",
		"| Method | Path | operation_id | Summary |",
	})

	content := readFileString(t, apiDocPath)
	doc := loadOpenAPIDoc(t)
	for path, methods := range doc.Paths {
		for rawMethod := range methods {
			method := strings.ToUpper(strings.TrimSpace(rawMethod))
			if method == "SUMMARY" || method == "DESCRIPTION" || method == "PARAMETERS" || method == "SERVERS" {
				continue
			}
			expectedRowPrefix := "| `" + method + "` | `" + path + "` |"
			if !strings.Contains(content, expectedRowPrefix) {
				t.Fatalf("api reference missing endpoint row for %s %s", method, path)
			}
		}
	}
}
