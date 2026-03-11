package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBlueprintCoverage validates that all 17 blueprint bodies have been
// implemented through normalized intent. This is the TASK-A blueprint
// extraction harness validation gate.
func TestBlueprintCoverage(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Gate A: All 17 blueprints ingested and honored
	t.Run("all_17_blueprints_present", func(t *testing.T) {
		blueprintDir := filepath.Join(root, "extracted-blueprints")
		if _, err := os.Stat(blueprintDir); os.IsNotExist(err) {
			t.Skip("extracted-blueprints directory not present (CI-only gate)")
		}
		entries, err := os.ReadDir(blueprintDir)
		if err != nil {
			t.Fatalf("failed to read blueprint directory: %v", err)
		}
		if len(entries) < 17 {
			t.Fatalf("expected 17 blueprint files, found %d", len(entries))
		}
	})

	// Gate B: Architecture coherence — one implementation per plane
	t.Run("architecture_coherence", func(t *testing.T) {
		// 5 authoritative Go plane binaries
		planes := []string{"gateway", "brain", "control", "executor", "canvas"}
		for _, plane := range planes {
			mainPath := filepath.Join(root, "cmd", plane, "main.go")
			assertFileNonEmpty(t, mainPath)
		}
		// Temporal worker binary
		assertFileNonEmpty(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"))
		// Doctor CLI binary
		assertFileNonEmpty(t, filepath.Join(root, "cmd", "brevioctl", "main.go"))
		// TS services quarantined
		assertFileNonEmpty(t, filepath.Join(root, "services", "NON_PRODUCTION.md"))
		assertFileNonEmpty(t, filepath.Join(root, "services", ".ci-quarantine.yaml"))
	})

	// Gate C: Data and contract integrity
	t.Run("data_contract_integrity", func(t *testing.T) {
		// All migrations present
		migrations := []string{
			"001_BREVIO_v9_init.sql",
			"002_BREVIO_v91_soft_intelligence.sql",
			"003_BREVIO_v92_production_hardening.sql",
			"004_BREVIO_ops_operational_systems.sql",
			"005_BREVIO_mcp_execution_oauth_hardening.sql",
			"006_BREVIO_v93_addendum_specification_closure.sql",
			"007_BREVIO_uuidv7_reconciliation.sql",
			"008_BREVIO_v10_gap_closure.sql",
			"009_BREVIO_v10_authorization_receipts.sql",
			"010_BREVIO_v101_admin_intelligence.sql",
			"011_BREVIO_v102_v103_intelligence.sql",
			"012_BREVIO_v104_voice_calls.sql",
			"013_BREVIO_openclaw_adoption.sql",
			"014_BREVIO_gateway_production_hardening.sql",
			"015_BREVIO_v101_cost_revenue_intelligence.sql",
			"016_BREVIO_v103_cognitive_architecture.sql",
		}
		for _, m := range migrations {
			assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", m))
		}
		// OpenAPI v10
		assertFileNonEmpty(t, filepath.Join(root, "api", "openapi", "v10.yaml"))
		// JSON schemas
		assertFileNonEmpty(t, filepath.Join(root, "api", "openapi", "v9.yaml"))
	})

	// Gate D: Security and policy integrity
	t.Run("security_policy_integrity", func(t *testing.T) {
		// OPA policies
		assertFileNonEmpty(t, filepath.Join(root, "policies", "base.rego"))
		assertFileNonEmpty(t, filepath.Join(root, "policies", "v10_gates.rego"))
		assertFileNonEmpty(t, filepath.Join(root, "policies", "v10_gates_test.rego"))
		assertFileNonEmpty(t, filepath.Join(root, "policies", "call_approval_gate.rego"))
		// Authorization receipts
		assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts_test.go"))
		// Identity
		assertFileNonEmpty(t, filepath.Join(root, "internal", "identity", "jwt_signer.go"))
	})

	// Gate E: Workflow/runtime integrity
	t.Run("workflow_runtime_integrity", func(t *testing.T) {
		// Temporal SDK
		assertFileContainsTokens(t, filepath.Join(root, "go.mod"), []string{
			"go.temporal.io/sdk",
		})
		// Temporal package
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "client.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "activities.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "worker.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows_test.go"))
		// Temporal worker registers workflows
		assertFileContainsTokens(t, filepath.Join(root, "cmd", "temporal-worker", "main.go"), []string{
			"breviotemporal",
		})
	})

	// Gate F: Build/test/verification
	t.Run("build_test_verification", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "Makefile"))
		assertFileNonEmpty(t, filepath.Join(root, "Dockerfile"))
		assertFileNonEmpty(t, filepath.Join(root, "docker-compose.yml"))
		assertFileNonEmpty(t, filepath.Join(root, ".github", "workflows", "ci.yml"))
	})

	// Gate G: Deployability
	t.Run("deployability", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "docker-compose.yml"))
		// Infra
		if _, err := os.Stat(filepath.Join(root, "infra", "terraform")); os.IsNotExist(err) {
			t.Fatal("infra/terraform must exist")
		}
		if _, err := os.Stat(filepath.Join(root, "infra", "helm")); os.IsNotExist(err) {
			t.Fatal("infra/helm must exist")
		}
	})

	// Gate H: Documentation accuracy
	t.Run("documentation_accuracy", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "ARCHITECTURE.md"))
		assertFileNonEmpty(t, filepath.Join(root, "DECISIONS.md"))
		assertFileNonEmpty(t, filepath.Join(root, "RUNBOOK.md"))
		// ARCHITECTURE.md must reference actual planes
		assertFileContainsTokens(t, filepath.Join(root, "ARCHITECTURE.md"), []string{
			"Gateway",
			"Brain",
			"Control",
			"Executor",
			"Canvas",
			"Temporal",
			"UUIDv7",
			"workspace_id",
		})
		// DECISIONS.md must contain binding decisions
		assertFileContainsTokens(t, filepath.Join(root, "DECISIONS.md"), []string{
			"D001",
			"negotiation_state",
			"federation_permission_type",
		})
		// RUNBOOK.md must contain operational procedures
		assertFileContainsTokens(t, filepath.Join(root, "RUNBOOK.md"), []string{
			"brevioctl doctor",
			"kill_switch",
		})
	})
}

// TestBlueprintRequirementMapping validates that each blueprint's normalized
// requirements map to implemented artifacts.
func TestBlueprintRequirementMapping(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// BP01: 4 Features Blueprint — demo-only, excluded from production boundary.
	// Demo artifacts must NOT exist in the repository.
	t.Run("BP01_demo_excluded", func(t *testing.T) {
		demoPath := filepath.Join(root, "apps", "web-demo")
		if _, err := os.Stat(demoPath); err == nil {
			t.Fatalf("demo artifact must not exist in production repo: %s", demoPath)
		}
	})

	// BP02: Admin Blueprint → V10.1 migration
	t.Run("BP02_admin", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "010_BREVIO_v101_admin_intelligence.sql"))
	})

	// BP03: Intelligence Addendum → V10.2 migration
	t.Run("BP03_intelligence", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "011_BREVIO_v102_v103_intelligence.sql"), []string{
			"eq_strategy_matrix",
			"confidence_calibration",
		})
	})

	// BP04: Cognitive Intelligence → V10.3 migration
	t.Run("BP04_cognitive", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "011_BREVIO_v102_v103_intelligence.sql"), []string{
			"prospective_memory",
			"metacognitive_monitors",
		})
	})

	// BP05: V10.4 Blueprint → voice/calls migration
	t.Run("BP05_voice", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql"))
	})

	// BP06: V10 Complete → gap closure migration
	t.Run("BP06_v10", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"))
	})

	// BP07: V9.1 Soft Intelligence → migration 002
	t.Run("BP07_v91", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql"))
	})

	// BP08: V9.2 Production Hardening → migration 003
	t.Run("BP08_v92", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"))
	})

	// BP09: V9 Consolidated → migration 001
	t.Run("BP09_v9", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"))
	})

	// BP10: Addendum → migration 006
	t.Run("BP10_addendum", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "006_BREVIO_v93_addendum_specification_closure.sql"))
	})

	// BP11: Blueprint Addendum → operational systems
	t.Run("BP11_ops", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "004_BREVIO_ops_operational_systems.sql"))
	})

	// BP12-BP15: Copy JSX → onboarding copy templates
	t.Run("BP12_15_copy", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "copy_templates.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "copy_templates_test.go"))
	})

	// BP16: OpenClaw Blueprint → skills integration
	t.Run("BP16_openclaw", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "013_BREVIO_openclaw_adoption.sql"))
	})

	// BP17: OpenClaw Unified → adoption features + doctor CLI
	t.Run("BP17_adoption", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "cmd", "brevioctl", "main.go"))
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "013_BREVIO_openclaw_adoption.sql"), []string{
			"pipeline_hooks",
			"a2a_messages",
			"sandbox_configs",
			"auth_profiles",
			"no_reply_config",
			"dm_pairing_config",
		})
	})
}
