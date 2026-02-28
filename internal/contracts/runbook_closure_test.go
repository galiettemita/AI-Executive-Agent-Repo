package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunbookClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	for i := 1; i <= 9; i++ {
		path := filepath.Join(root, "runbooks", fmt.Sprintf("RB-%03d.md", i))
		assertFileNonEmpty(t, path)
		content := readRunbook(t, path)
		if strings.Contains(content, "Placeholder runbook") {
			t.Fatalf("runbook still placeholder: %s", path)
		}
		requireRunbookTokens(t, path, content, []string{
			"## Trigger",
			"## Detection",
			"## Immediate Actions",
			"## Mitigation",
			"## Recovery",
			"## Verification",
			"## Escalation",
		})
	}

	for i := 1; i <= 9; i++ {
		path := filepath.Join(root, "runbooks", fmt.Sprintf("RB-V92-%03d.md", i))
		assertFileNonEmpty(t, path)
		content := readRunbook(t, path)
		if strings.Contains(content, "Placeholder runbook") {
			t.Fatalf("runbook still placeholder: %s", path)
		}
		requireRunbookTokens(t, path, content, []string{
			"## Trigger",
			"## Detection",
			"## Immediate Actions",
			"## Mitigation",
			"## Recovery",
			"## Verification",
			"## Escalation",
		})
	}
}

func readRunbook(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read runbook %s: %v", path, err)
	}
	return string(body)
}

func requireRunbookTokens(t *testing.T, path, content string, tokens []string) {
	t.Helper()
	for _, token := range tokens {
		if !strings.Contains(content, token) {
			t.Fatalf("runbook missing token %q in %s", token, path)
		}
	}
}
