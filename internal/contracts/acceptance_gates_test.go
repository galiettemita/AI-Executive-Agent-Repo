package contracts

import (
	"path/filepath"
	"testing"
)

func TestAcceptanceGatesV9(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("schema_closure", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "schemas", "tool_call.v9.json"))
		assertFileNonEmpty(t, filepath.Join(root, "schemas", "error.v9.json"))
	})
	t.Run("determinism", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "evals", "determinism_fixtures.json"))
	})
	t.Run("webhook_security", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "service.go"), []string{
			"ErrInvalidSignature",
			"ErrReplayDetected",
		})
	})
	t.Run("acceptance_suites_1_12", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, ".github", "workflows", "ci.yaml"))
	})
	t.Run("workspace_isolation", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"), []string{
			"ENABLE ROW LEVEL SECURITY",
			"current_setting(''app.workspace_id'')::uuid",
		})
	})
	t.Run("provisioning_pipeline", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "provisioning", "service_test.go"))
	})
	t.Run("onboarding_completion", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "service_test.go"))
	})
	t.Run("provisioning_recovery", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "workflows", "service_test.go"), []string{
			"TestProvisioningCompensationReverseOrder",
			"CompensatedSteps",
		})
	})
	t.Run("deterministic_llm", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "llm", "service_test.go"), []string{
			"TestDeterminismSameInput20Runs",
			"ReplayHitCount",
		})
	})
	t.Run("cve_scanning", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, ".github", "workflows", "ci.yaml"), []string{
			"trivy",
		})
	})
}

func TestAcceptanceGatesV91(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	requiredGates := []string{
		"goal_lifecycle",
		"trust_scoring",
		"trust_autonomy_upgrade",
		"learning_pipeline",
		"daily_capture",
		"mission_control",
		"self_modification_gate",
		"cross_repo_intelligence",
		"capability_exploration",
		"code_context_export",
		"adaptive_discovery",
	}
	for _, gate := range requiredGates {
		gate := gate
		t.Run(gate, func(t *testing.T) {
			assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql"))
			assertFileNonEmpty(t, filepath.Join(root, "prompts", "seed_prompts_v91.txt"))
			assertFileNonEmpty(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"))
		})
	}
}

func TestAcceptanceGatesV92(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	requiredGates := []string{
		"context_budget_enforcement",
		"rag_pipeline_functional",
		"rag_eval_gate",
		"session_management",
		"temporal_reasoning",
		"guardrails_runtime",
		"tool_health_scoring",
		"feature_flag_system",
		"crdt_conflict_resolution",
		"streaming_sla",
		"error_communication",
		"event_schema_registry",
		"compliance_automation",
		"caching_layers",
		"model_tier_enforcement",
		"react_early_exit",
		"security_hardening",
		"admin_backend",
		"structured_generation",
	}
	for _, gate := range requiredGates {
		gate := gate
		t.Run(gate, func(t *testing.T) {
			assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"))
			assertFileNonEmpty(t, filepath.Join(root, "prompts", "seed_prompts_v92.txt"))
			assertFileNonEmpty(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"))
			assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"), []string{
				"constrained_decoding_config",
			})
		})
	}
}
