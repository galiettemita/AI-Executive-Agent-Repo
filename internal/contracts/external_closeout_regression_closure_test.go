package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalCloseoutRegressionGuardClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"external-closeout-regression-check:",
		"check_external_closeout_regressions.sh",
	})

	regressionScriptPath := filepath.Join(root, "scripts", "deploy", "check_external_closeout_regressions.sh")
	assertFileNonEmpty(t, regressionScriptPath)
	assertFileContainsTokens(t, regressionScriptPath, []string{
		"external_closeout_status.json",
		"external_closeout_status.last.json",
		"external_closeout_regression_report.json",
		"REGRESSION_DETECTED",
		"ALLOW_EXTERNAL_REGRESSIONS",
	})

	syncScriptPath := filepath.Join(root, "scripts", "deploy", "sync_external_phase_artifacts.sh")
	assertFileContainsTokens(t, syncScriptPath, []string{
		"EXTERNAL_REGRESSION_CHECK",
		"check_external_closeout_regressions.sh",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make external-closeout-regression-check",
		"external_closeout_regression_report.json",
		"EXTERNAL_REGRESSION_CHECK=1 make external-phase-sync",
	})
}
