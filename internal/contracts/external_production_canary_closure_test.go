package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalProductionCanaryClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"production-canary-check:",
		"check_production_canary_window.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "check_production_canary_window.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"production_canary_check.json",
		"CANARY_TRAFFIC_PCT",
		"CANARY_DURATION_MINUTES",
		"CANARY_ERROR_RATE_PCT",
		"CANARY_P99_RATIO",
		"pass_canary",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-canary-check",
		"production_canary_check.json",
		"CANARY_ERROR_RATE_PCT",
	})
}
