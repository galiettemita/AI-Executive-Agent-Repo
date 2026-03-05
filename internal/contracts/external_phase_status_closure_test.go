package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalPhaseStatusClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"phase-status:",
		"print_phase_status.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "print_phase_status.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"phase_closure_manifest.json",
		"phase_handoff_bundle.json",
		"phase_status.txt",
		"overall_status",
		"canary_status",
		"next_action",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make phase-status",
		"phase_status.txt",
	})
}
