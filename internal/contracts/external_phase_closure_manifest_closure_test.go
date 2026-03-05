package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalPhaseClosureManifestClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"phase-closure-manifest:",
		"generate_phase_closure_manifest.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "generate_phase_closure_manifest.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"external_closeout_status.json",
		"go_live_signoff_status.json",
		"external_phase_transition_check.json",
		"production_deployment_signoff_check.json",
		"production_post_deploy_validation.json",
		"phase_closure_manifest.json",
		"overall_status",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make phase-closure-manifest",
		"phase_closure_manifest.json",
	})
}
