package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalPhaseTransitionClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"external-phase-transition-check:",
		"check_external_phase_transition.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "check_external_phase_transition.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"go_live_signoff_status.json",
		"external_phase_transition_check.json",
		"ALLOW_CONDITIONAL_MANUAL",
		"production-deployment-signoff",
		"pass_transition",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make external-phase-transition-check",
		"external_phase_transition_check.json",
		"ALLOW_CONDITIONAL_MANUAL=1 make external-phase-transition-check",
	})
}
