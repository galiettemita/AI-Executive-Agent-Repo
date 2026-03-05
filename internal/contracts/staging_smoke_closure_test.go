package contracts

import (
	"path/filepath"
	"testing"
)

func TestStagingSmokeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"staging-smoke-tests:",
		"run_staging_smoke_tests.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "run_staging_smoke_tests.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"staging_smoke_test_report.json",
		"brevio-gateway",
		"brevio-temporal-worker",
		"gateway_health",
		"gateway_webhook_route",
		"temporal_message_workflow_start",
	})

	ciWorkflow := filepath.Join(root, ".github", "workflows", "ci.yml")
	assertFileContainsTokens(t, ciWorkflow, []string{
		"Staging smoke tests",
		"run_staging_smoke_tests.sh",
		"Upload staging smoke artifacts",
		"staging_smoke_test_report.json",
	})

	stagingWorkflow := filepath.Join(root, ".github", "workflows", "deploy-staging.yml")
	assertFileContainsTokens(t, stagingWorkflow, []string{
		"Staging smoke tests",
		"run_staging_smoke_tests.sh",
		"Upload staging smoke artifacts",
		"staging_smoke_test_report.json",
	})
}
