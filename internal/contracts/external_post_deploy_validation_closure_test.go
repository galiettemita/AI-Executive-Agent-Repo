package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalPostDeployValidationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"production-post-deploy-validation:",
		"check_production_post_deploy_validation.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "check_production_post_deploy_validation.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"production_deployment_signoff_check.json",
		"production_post_deploy_validation.json",
		"ALLOW_CONDITIONAL_MANUAL",
		"CANARY_ERROR_RATE_PCT",
		"CANARY_P99_RATIO",
		"deployment-complete",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-post-deploy-validation",
		"production_post_deploy_validation.json",
		"CANARY_ERROR_RATE_PCT",
	})
}
