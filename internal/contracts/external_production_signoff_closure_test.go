package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalProductionSignoffClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"production-deployment-signoff-check:",
		"check_production_deployment_signoff.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "check_production_deployment_signoff.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"external_phase_transition_check.json",
		"go_live_signoff_status.json",
		"external_closeout_regression_report.json",
		"production_deployment_signoff_check.json",
		"production-deployment-signoff",
		"conditional_manual_override",
		"pass_signoff",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-deployment-signoff-check",
		"production_deployment_signoff_check.json",
		"ALLOW_CONDITIONAL_MANUAL=1 make external-phase-transition-check",
	})
}
