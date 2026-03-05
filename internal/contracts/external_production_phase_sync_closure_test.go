package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalProductionPhaseSyncClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"production-phase-sync:",
		"sync_production_phase_artifacts.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "sync_production_phase_artifacts.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"check_external_phase_transition.sh",
		"check_production_deployment_signoff.sh",
		"check_production_canary_window.sh",
		"generate_production_deployment_todo.sh",
		"check_production_post_deploy_validation.sh",
		"production phase artifacts synced",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-phase-sync",
		"sync_production_phase_artifacts.sh",
	})
}
