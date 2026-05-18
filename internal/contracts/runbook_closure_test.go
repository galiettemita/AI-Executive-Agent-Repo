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

	// V10 runbooks — kill switch, calls, billing, brain ingress.
	v10Tokens := map[string]string{
		"RB-V10-001.md": "Kill Switch",
		"RB-V10-002.md": "Outbound Call",
		"RB-V10-003.md": "Billing",
		"RB-V10-004.md": "Brain Ingress",
	}
	for name, token := range v10Tokens {
		path := filepath.Join(root, "runbooks", name)
		assertFileNonEmpty(t, path)
		content := readRunbook(t, path)
		if !strings.Contains(content, token) {
			t.Fatalf("v10 runbook missing token %q in %s", token, name)
		}
	}

	v92TriggerTokens := map[string]string{
		"RB-V92-001.md": "more than 10%",
		"RB-V92-002.md": "BREVIO.tool_health.quarantined.v1",
		"RB-V92-003.md": "exceeds 20%",
		"RB-V92-004.md": "exceeds 500ms for more than 5%",
		"RB-V92-005.md": "correlated with recent feature-flag change",
		"RB-V92-006.md": "Conflict rate spikes",
		"RB-V92-007.md": "5-day-to-deadline",
		"RB-V92-008.md": "block rate exceeds 5%",
		"RB-V92-009.md": "unavailable or serving degraded responses",
	}
	for name, triggerToken := range v92TriggerTokens {
		path := filepath.Join(root, "runbooks", name)
		content := readRunbook(t, path)
		if !strings.Contains(content, triggerToken) {
			t.Fatalf("v9.2 runbook trigger mismatch in %s; missing token %q", path, triggerToken)
		}
	}
}

func expectedRunbookNames() []string {
	names := make([]string, 0, 22)
	for i := 1; i <= 9; i++ {
		names = append(names, fmt.Sprintf("RB-%03d.md", i))
	}
	for i := 1; i <= 9; i++ {
		names = append(names, fmt.Sprintf("RB-V92-%03d.md", i))
	}
	for i := 1; i <= 4; i++ {
		names = append(names, fmt.Sprintf("RB-V10-%03d.md", i))
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
