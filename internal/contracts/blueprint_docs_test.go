package contracts

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestBlueprintDocsTracked(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	requiredDocs := []string{
		"Brevio_V9_Consolidated_Master_Blueprint.docx",
		"Brevio_V91_Addendum_Soft_Intelligence_Layer.docx",
		"Brevio_V92_Addendum_Production_Hardening.docx",
	}
	assertExactBlueprintDocSet(t, root, requiredDocs)
	for _, doc := range requiredDocs {
		assertFileNonEmpty(t, filepath.Join(root, doc))
		assertGitTracked(t, root, doc)
	}
}

func assertExactBlueprintDocSet(t *testing.T, root string, expected []string) {
	t.Helper()

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read repository root %s: %v", root, err)
	}

	actualSet := map[string]struct{}{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".docx") {
			actualSet[name] = struct{}{}
		}
	}

	expectedSet := map[string]struct{}{}
	for _, name := range expected {
		expectedSet[name] = struct{}{}
	}
	optionalSet := map[string]struct{}{
		"brevio-openclaw-blueprint.docx": {},
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
			if _, allowed := optionalSet[name]; allowed {
				continue
			}
			extra = append(extra, name)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return
	}

	t.Fatalf("blueprint docx file-set mismatch: missing=%v extra=%v", missing, extra)
}

func assertGitTracked(t *testing.T, root, relativePath string) {
	t.Helper()

	cmd := exec.Command("git", "ls-files", "--error-unmatch", relativePath)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("required blueprint doc is not git-tracked: %s (%v): %s", relativePath, err, string(out))
	}
}
