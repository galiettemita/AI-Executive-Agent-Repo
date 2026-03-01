package contracts

import (
	"path/filepath"
	"testing"
)

func TestOperationsOwnershipClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "docs", "OPERATIONS_OWNERSHIP.md")
	assertFileNonEmpty(t, path)
	assertFileContainsTokens(t, path, []string{
		"# OPERATIONS OWNERSHIP",
		"## Plane Ownership Matrix",
		"## Connector Ownership Matrix",
		"## On-Call Rotation And Escalation Policy",
		"Severity Levels",
		"Escalation Chain",
		"Gateway",
		"Brain",
		"Hands/Executor",
		"Data",
		"Infra",
		"Security",
		"Observability",
	})
}

func TestSection33MetricsAndAlertThresholdClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	metricsPath := filepath.Join(root, "spec", "metrics", "section33_metrics_core.txt")
	alertsPath := filepath.Join(root, "spec", "alerts", "section33_alert_thresholds.yaml")

	assertFileNonEmpty(t, metricsPath)
	assertFileNonEmpty(t, alertsPath)

	assertFileContainsTokens(t, metricsPath, []string{
		"BREVIO_interactive_turn_latency_t1_ms_p95",
		"BREVIO_interactive_turn_latency_t2_ms_p95",
		"BREVIO_interactive_turn_latency_t3_ms_p95",
		"BREVIO_gateway_availability_pct",
		"BREVIO_error_rate_pct",
		"BREVIO_cache_hit_rate",
		"BREVIO_provider_failover_total",
	})

	assertFileContainsTokens(t, alertsPath, []string{
		"brevio_t1_latency_p95_breach",
		"brevio_t2_latency_p95_breach",
		"brevio_t3_latency_p95_breach",
		"brevio_error_rate_breach",
		"brevio_gateway_availability_breach",
		"brevio_provider_failover_spike",
		"page_primary_on_call",
	})
}

func TestSection34CoverageClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	// Unit test coverage anchors.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "model_tiers", "service_test.go"), []string{
		"TestEnforceTierDowngradesAndAudits",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "context", "service_test.go"), []string{
		"TestAllocateContextDeterministicAndOverflowGate",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "memory", "service_test.go"), []string{
		"TestConsolidationMergesDuplicatesAndExpiresStale",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "onboarding", "service_test.go"), []string{
		"TestRunStageReplayLockedExtraction",
		"TestConnectionTemplatesContainOnboardingButtons",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "llm", "service_test.go"), []string{
		"TestDeterminismSameInput20Runs",
	})

	// Integration and contract suite anchors.
	assertFileContainsTokens(t, filepath.Join(root, "internal", "integration", "service_test.go"), []string{
		"TestPipelineEndToEnd",
		"TestPipelineEndToEndIMessagePath",
		"TestPipelineMultiToolPlanCommitsAllTools",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "contracts", "schema_closure_test.go"), []string{
		"schemas",
		"additionalProperties",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "contracts", "openapi_closure_test.go"), []string{
		"TestOpenAPIV9EndpointParityClosure",
		"openapi path count mismatch",
	})
}

func TestToolsCatalogPipelineClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	assertFileNonEmpty(t, filepath.Join(root, "scripts", "tools", "generate_tools_md.go"))
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "tools", "generate_tools_md.go"), []string{
		"CONNECTED_CONNECTOR_KEYS",
		"internal/connectors/seeds/connectors.yaml",
		"## Connected Apps",
		"## Available But Not Connected",
		"## MCP Budget And Rate-Limit Matrix",
		"TOOLS.md",
	})

	assertFileContainsTokens(t, filepath.Join(root, "Makefile"), []string{
		"tools-md:",
		"tools-md-check:",
		"./scripts/tools/generate_tools_md.go",
	})
}
