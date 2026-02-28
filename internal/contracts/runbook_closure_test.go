package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRunbookClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	assertExactRunbookSet(t, filepath.Join(root, "runbooks"), expectedRunbookNames())

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

func expectedRunbookNames() []string {
	names := make([]string, 0, 18)
	for i := 1; i <= 9; i++ {
		names = append(names, fmt.Sprintf("RB-%03d.md", i))
	}
	for i := 1; i <= 9; i++ {
		names = append(names, fmt.Sprintf("RB-V92-%03d.md", i))
	}
	return names
}

func assertExactRunbookSet(t *testing.T, dir string, expected []string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read runbook dir %s: %v", dir, err)
	}

	actualSet := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "RB-") && strings.HasSuffix(name, ".md") {
			actualSet[name] = struct{}{}
		}
	}

	expectedSet := map[string]struct{}{}
	for _, name := range expected {
		expectedSet[name] = struct{}{}
	}

	missing := make([]string, 0)
	for name := range expectedSet {
		if _, ok := actualSet[name]; !ok {
			missing = append(missing, name)
		}
	}
	extra := make([]string, 0)
	for name := range actualSet {
		if _, ok := expectedSet[name]; !ok {
			extra = append(extra, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	t.Fatalf("runbook file-set mismatch: missing=%v extra=%v", missing, extra)
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
