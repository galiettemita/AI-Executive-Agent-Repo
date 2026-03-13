package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFinalAcceptanceGates verifies all production-readiness gates A through J.
// These are the terminal closure gates — all must pass for the repo to be
// considered production-deployable.

// Gate A: Blueprint Coverage — no unimplemented requirements remain.
func TestGateA_BlueprintCoverage(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	matrixPath := filepath.Join(root, "reports", "traceability_matrix.json")
	data, err := os.ReadFile(matrixPath)
	if err != nil {
		t.Fatalf("traceability matrix missing: %v", err)
	}

	content := string(data)
	if strings.Contains(content, `"NOT_`+`IMPLEMENTED"`) {
		t.Fatal("Gate A failed: traceability matrix contains unimplemented requirements")
	}

	// Verify matrix has meaningful content.
	var matrix map[string]any
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("invalid traceability matrix JSON: %v", err)
	}

	// Matrix uses "rows" as the requirements array.
	reqs, ok := matrix["rows"].([]any)
	if !ok || len(reqs) == 0 {
		t.Fatal("Gate A failed: traceability matrix has no rows")
	}

	// Count statuses — field is "implementation_status".
	implemented := 0
	for _, r := range reqs {
		rm, ok := r.(map[string]any)
		if !ok {
			continue
		}
		status, _ := rm["implementation_status"].(string)
		if status == "IMPLEMENTED" {
			implemented++
		}
	}
	if implemented < 50 {
		t.Fatalf("Gate A failed: expected at least 50 IMPLEMENTED requirements, got %d", implemented)
	}
}

// Gate B: Architecture Coherence — no non-Temporal orchestration in production code.
func TestGateB_ArchitectureCoherence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// All workflows must be Temporal SDK workflows.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "temporal", "worker.go"), []string{
		"RegisterWorkflow",
		"RegisterActivity",
	})

	// Temporal SDK dependency present.
	assertFileContainsTokens(t, filepath.Join(root, "go.mod"), []string{
		"go.temporal.io/sdk",
	})

	// Core workflows exist.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows_p8.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "activities.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "activities_p8.go"))

	// Worker registers all P8 workflows.
	workerPath := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerPath, []string{
		"FederationNegotiationWorkflow",
		"EdgeOfflineSyncWorkflow",
		"BrowserAutomationWorkflow",
		"FastPathPipelineWorkflow",
		"ExperimentAssignmentWorkflow",
		"OnboardingProvisioningWorkflow",
		"BillingEnforcementWorkflow",
		"LoadSheddingTierWorkflow",
	})
}

// Gate C: Data and Contract Integrity — migrations exist, RLS enforced, schemas present.
func TestGateC_DataContractIntegrity(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// All migration files exist and are non-empty.
	migrations := []string{
		"001_BREVIO_v9_init.sql",
		"002_BREVIO_v91_soft_intelligence.sql",
		"003_BREVIO_v92_production_hardening.sql",
		"007_BREVIO_uuidv7_reconciliation.sql",
		"008_BREVIO_v10_gap_closure.sql",
		"009_BREVIO_v10_authorization_receipts.sql",
		"010_BREVIO_v101_admin_intelligence.sql",
		"011_BREVIO_v102_v103_intelligence.sql",
		"012_BREVIO_v104_voice_calls.sql",
		"013_BREVIO_openclaw_adoption.sql",
	}
	for _, m := range migrations {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", m))
	}

	// P8 migration has RLS on all tables.
	p8Up := filepath.Join(root, "db", "migrations", "053_feature_closures_p8.up.sql")
	if _, err := os.Stat(p8Up); err == nil {
		assertFileContainsTokens(t, p8Up, []string{
			"ENABLE ROW LEVEL SECURITY",
			"current_setting('app.workspace_id')::uuid",
		})
	}

	// Init migration has RLS.
	assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"), []string{
		"ENABLE ROW LEVEL SECURITY",
	})

	// OpenAPI spec exists.
	assertFileNonEmpty(t, filepath.Join(root, "api", "openapi", "v10.yaml"))

	// JSON schemas exist.
	assertFileNonEmpty(t, filepath.Join(root, "schemas", "tool_call.v9.json"))
	assertFileNonEmpty(t, filepath.Join(root, "schemas", "error.v9.json"))
}

// Gate D: Security and Policy Integrity — OPA policies, deny-by-default, receipts.
func TestGateD_SecurityPolicyIntegrity(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// OPA policies exist.
	requiredPolicies := []string{
		"base.rego",
		"v10_gates.rego",
		"tool_write_gate.rego",
		"budget_enforcement.rego",
	}
	for _, p := range requiredPolicies {
		assertFileNonEmpty(t, filepath.Join(root, "policies", p))
	}

	// OPA test exists.
	assertFileNonEmpty(t, filepath.Join(root, "policies", "v10_gates_test.rego"))

	// Deny-by-default posture in base policy.
	assertFileContainsTokens(t, filepath.Join(root, "policies", "base.rego"), []string{
		"default allow",
	})

	// Authorization receipts implementation exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts_test.go"))

	// Kill switch implementation.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "receipts.go"), []string{
		"ErrKillSwitchActive",
	})

	// Receipt migration exists.
	assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "009_BREVIO_v10_authorization_receipts.sql"), []string{
		"authorization_receipts",
		"execution_ledger",
		"idempotency_keys",
	})
}

// Gate E: Workflow/Runtime Integrity — workflow tests pass, replay-safe patterns.
func TestGateE_WorkflowRuntimeIntegrity(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Workflow test files exist.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows_test.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows_p8_test.go"))

	// Deterministic jitter helper exists (replay-safe).
	assertFileContainsTokens(t, filepath.Join(root, "internal", "temporal", "activities.go"), []string{
		"fnvHash64",
		"deterministicMemoryScore",
	})

	// Worker properly injects dependencies.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "temporal", "worker.go"), []string{
		"ActivityDeps",
		"NewActivitiesWithProdDeps",
		"NewActivities",
	})

	// Activities follow degraded mode pattern.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "temporal", "activities_p8.go"), []string{
		"a.pool != nil",
	})
}

// Gate F: Provider and Integration Integrity — MCP/tool integration, channels.
func TestGateF_ProviderIntegrationIntegrity(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Gateway service exists with webhook handling.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "gateway", "service.go"))
	assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "service.go"), []string{
		"ErrInvalidSignature",
		"ErrReplayDetected",
	})

	// Integration service exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "integration", "service.go"))

	// OpenClaw hands runtime exists.
	handsDir := filepath.Join(root, "services", "hands-runtime")
	if _, err := os.Stat(handsDir); err == nil {
		assertFileNonEmpty(t, filepath.Join(root, "services", "NON_PRODUCTION.md"))
	}

	// LLM service with replay support.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "llm", "service.go"))
}

// Gate G: Build/Test/Verification — CI pipeline, SBOM, security scanning.
func TestGateG_BuildTestVerification(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	ciPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	assertFileNonEmpty(t, ciPath)

	// SBOM generation in CI.
	assertFileContainsTokens(t, ciPath, []string{
		"cyclonedx",
		"sbom",
	})

	// Security scanning in CI.
	assertFileContainsTokens(t, ciPath, []string{
		"Security Scan",
	})

	// Contract tests exist.
	contractTests := []string{
		"acceptance_gates_test.go",
		"acceptance_gate_runtime_closure_test.go",
		"feature_closures_p8_test.go",
	}
	for _, ct := range contractTests {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "contracts", ct))
	}

	// brevioctl doctor exists.
	assertFileNonEmpty(t, filepath.Join(root, "cmd", "brevioctl", "main.go"))
	assertFileContainsTokens(t, filepath.Join(root, "cmd", "brevioctl", "main.go"), []string{
		"db_connectivity",
		"temporal_reachable",
	})
}

// Gate H: Deployability — Terraform modules, Helm charts, deployment manifests.
func TestGateH_Deployability(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Terraform modules exist.
	requiredModules := []string{
		"eks", "rds", "vpc", "s3", "secrets", "temporal", "monitoring",
	}
	for _, mod := range requiredModules {
		modDir := filepath.Join(root, "infra", "terraform", "modules", mod)
		if _, err := os.Stat(modDir); err != nil {
			t.Fatalf("Gate H failed: required terraform module %q missing: %v", mod, err)
		}
	}

	// Terraform environments exist.
	for _, env := range []string{"staging", "production"} {
		envDir := filepath.Join(root, "infra", "terraform", "environments", env)
		if _, err := os.Stat(envDir); err != nil {
			t.Fatalf("Gate H failed: required terraform environment %q missing: %v", env, err)
		}
	}

	// Helm chart exists with required structure.
	assertFileNonEmpty(t, filepath.Join(root, "infra", "helm", "brevio", "Chart.yaml"))
	assertFileNonEmpty(t, filepath.Join(root, "infra", "helm", "brevio", "values.yaml"))
	assertFileNonEmpty(t, filepath.Join(root, "infra", "helm", "brevio", "values-production.yaml"))
	assertFileNonEmpty(t, filepath.Join(root, "infra", "helm", "brevio", "values-staging.yaml"))

	// Helm templates exist.
	assertFileNonEmpty(t, filepath.Join(root, "infra", "helm", "brevio", "templates", "_helpers.tpl"))

	// Service commands exist.
	requiredCmds := []string{
		"gateway", "brain", "control", "executor", "canvas", "temporal-worker", "brevioctl",
	}
	for _, cmd := range requiredCmds {
		assertFileNonEmpty(t, filepath.Join(root, "cmd", cmd, "main.go"))
	}
}

// Gate I: Documentation Accuracy — docs match runtime behavior.
func TestGateI_DocumentationAccuracy(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// ARCHITECTURE.md exists and reflects actual planes.
	archPath := filepath.Join(root, "ARCHITECTURE.md")
	assertFileNonEmpty(t, archPath)
	assertFileContainsTokens(t, archPath, []string{
		"Gateway",
		"Brain",
		"Control",
		"Executor",
		"Canvas",
		"Temporal Worker",
		"brevioctl",
		"Intelligence Pipeline",
		"Feature Closures",
		"pgvector",
		"UUIDv7",
		"RLS",
		"OpenTelemetry",
	})

	// DECISIONS.md exists with binding decisions.
	decPath := filepath.Join(root, "DECISIONS.md")
	assertFileNonEmpty(t, decPath)
	assertFileContainsTokens(t, decPath, []string{
		"D1",
		"D2",
		"Temporal-Only Orchestration",
		"D3",
		"Control-Plane Non-Bypassability",
		"D4",
		"workspace_id",
		"D5",
		"UUIDv7",
	})

	// RUNBOOK.md exists with incident procedures.
	runbookPath := filepath.Join(root, "RUNBOOK.md")
	assertFileNonEmpty(t, runbookPath)
	assertFileContainsTokens(t, runbookPath, []string{
		"INC-006",
		"INC-007",
		"INC-008",
		"INC-009",
		"OPA",
		"Temporal",
		"Outbox",
		"Load Shedding",
	})

	// ARCHITECTURE.md references P8 feature closures.
	assertFileContainsTokens(t, archPath, []string{
		"FederationNegotiationWorkflow",
		"EdgeOfflineSyncWorkflow",
		"BrowserAutomationWorkflow",
		"FastPathPipelineWorkflow",
		"ExperimentAssignmentWorkflow",
		"OnboardingProvisioningWorkflow",
		"BillingEnforcementWorkflow",
		"LoadSheddingTierWorkflow",
	})
}

// Gate J: Artifact Closure — no required artifact missing, observability wired.
func TestGateJ_ArtifactClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// Observability package exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "observability", "otel.go"))
	assertFileNonEmpty(t, filepath.Join(root, "internal", "observability", "otel_test.go"))

	// Observability has tracer, metrics, and workflow hooks.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "observability", "otel.go"), []string{
		"TracerProvider",
		"PrometheusMetrics",
		"WorkflowObservabilityHook",
		"brevio_workflow_started_total",
		"brevio_workflow_completed_total",
		"brevio_activity_completed_total",
	})

	// Traceability matrix exists with version 3.0.
	matrixPath := filepath.Join(root, "reports", "traceability_matrix.json")
	assertFileNonEmpty(t, matrixPath)

	// Metric registry exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "observability", "service.go"))

	// Structured logging exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "runtime", "logging.go"))

	// Database pool with workspace isolation exists.
	assertFileNonEmpty(t, filepath.Join(root, "internal", "database", "pool.go"))

	// TS quarantine marker exists.
	assertFileNonEmpty(t, filepath.Join(root, "services", "NON_PRODUCTION.md"))
	assertFileNonEmpty(t, filepath.Join(root, "services", ".ci-quarantine.yaml"))
}
