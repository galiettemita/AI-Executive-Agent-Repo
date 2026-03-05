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
		"SLO_WINDOW_MINUTES",
		"SLO_P50_LATENCY_SECONDS",
		"SLO_P99_LATENCY_SECONDS",
		"SLO_SKILL_SUCCESS_RATE_PCT",
		"SLO_DELIVERY_SUCCESS_RATE_PCT",
		"slo_window_1h",
		"deployment-complete",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-post-deploy-validation",
		"production_post_deploy_validation.json",
		"CANARY_ERROR_RATE_PCT",
	})

	ciWorkflowPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	assertFileContainsTokens(t, ciWorkflowPath, []string{
		"Production post-deploy validation gate",
		"check_external_phase_transition.sh",
		"check_production_deployment_signoff.sh",
		"check_production_post_deploy_validation.sh",
		"Production phase closure bundle",
		"generate_phase_closure_manifest.sh",
		"create_phase_handoff_bundle.sh",
		"print_phase_status.sh",
		"SLO_P50_LATENCY_SECONDS",
		"SLO_P99_LATENCY_SECONDS",
		"SLO_SKILL_SUCCESS_RATE_PCT",
		"SLO_DELIVERY_SUCCESS_RATE_PCT",
		"Upload production gate artifacts",
		"production_post_deploy_validation.json",
		"phase_closure_manifest.json",
		"phase_handoff_bundle.json",
		"phase_status.txt",
	})

	prodWorkflowPath := filepath.Join(root, ".github", "workflows", "deploy-production.yml")
	assertFileContainsTokens(t, prodWorkflowPath, []string{
		"Production post-deploy validation gate",
		"check_external_phase_transition.sh",
		"check_production_deployment_signoff.sh",
		"check_production_post_deploy_validation.sh",
		"Production phase closure bundle",
		"generate_phase_closure_manifest.sh",
		"create_phase_handoff_bundle.sh",
		"print_phase_status.sh",
		"SLO_P50_LATENCY_SECONDS",
		"SLO_P99_LATENCY_SECONDS",
		"SLO_SKILL_SUCCESS_RATE_PCT",
		"SLO_DELIVERY_SUCCESS_RATE_PCT",
		"Upload production gate artifacts",
		"production_post_deploy_validation.json",
		"phase_closure_manifest.json",
		"phase_handoff_bundle.json",
		"phase_status.txt",
	})
}
