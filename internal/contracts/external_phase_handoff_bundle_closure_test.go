package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalPhaseHandoffBundleClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"phase-handoff-bundle:",
		"create_phase_handoff_bundle.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "create_phase_handoff_bundle.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"phase_closure_manifest.json",
		"phase_handoff_bundle.json",
		"production_canary_check.json",
		"FINAL_VALIDATION_brevio_openclaw.md",
		"phase-handoff-",
		"tar -czf",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make phase-handoff-bundle",
		"phase_handoff_bundle.json",
	})
}
