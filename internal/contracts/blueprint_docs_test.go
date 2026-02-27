package contracts

import (
	"os/exec"
	"path/filepath"
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
	for _, doc := range requiredDocs {
		assertFileNonEmpty(t, filepath.Join(root, doc))
		assertGitTracked(t, root, doc)
	}
}

func assertGitTracked(t *testing.T, root, relativePath string) {
	t.Helper()

	cmd := exec.Command("git", "ls-files", "--error-unmatch", relativePath)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("required blueprint doc is not git-tracked: %s (%v): %s", relativePath, err, string(out))
	}
}
