package contracts

import (
	"path/filepath"
	"testing"
)

func TestMCPWave1ChecklistAutomationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	scriptPath := filepath.Join(root, "scripts", "mcp", "wave1_checklist", "main.go")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"wave1_12_step_deployment_v1",
		"server_manifest_registered",
		"capability_probe_tools_list",
		"oauth_or_auth_configured",
		"callback_routing_defined",
		"risk_classification_present",
		"approval_thresholds_present",
		"normalization_path_verified",
		"security_guardrails_verified",
		"cost_tracking_rate_limit_verified",
		"onboarding_card_or_trigger_present",
		"golden_scenarios_present",
		"runbook_failure_handling_present",
		"google_calendar",
		"google_drive",
		"google_gmail",
		"notion",
		"todoist",
		"brave_search",
		"github",
		"apple_reminders",
		"wave1_deployment_checklist_report.json",
	})

	scenariosPath := filepath.Join(root, "evals", "mcp", "wave1_golden_scenarios.json")
	assertFileNonEmpty(t, scenariosPath)
	assertFileContainsTokens(t, scenariosPath, []string{
		"google_calendar",
		"google_drive",
		"google_gmail",
		"notion",
		"todoist",
		"brave_search",
		"github",
		"apple_reminders",
	})

	runbookPath := filepath.Join(root, "runbooks", "RB-005.md")
	assertFileNonEmpty(t, runbookPath)
	assertFileContainsTokens(t, runbookPath, []string{
		"Connector-Specific Checks (Wave 1)",
		"google_calendar",
		"google_drive",
		"google_gmail",
		"notion",
		"todoist",
		"brave_search",
		"github",
		"apple_reminders",
	})
	assertFileContainsTokens(t, filepath.Join(root, "runbooks", "RB-004.md"), []string{
		"OAuth callback",
	})

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"mcp-wave1-checklist:",
		"./scripts/mcp/wave1_checklist/main.go",
		"mcp-wave56-checklist:",
		"./scripts/mcp/wave56_checklist/main.go",
		"mcp-fleet-validate:",
		"./scripts/mcp/fleet_validation/main.go",
		"mcp-runtime-rollout:",
		"./scripts/mcp/runtime_rollout/main.go",
		"ci: lint build test migrate api-docs-check tools-md-check mcp-wave1-checklist mcp-wave56-checklist mcp-fleet-validate mcp-runtime-rollout policy-validate contracts acceptance",
	})

	fleetSpecPath := filepath.Join(root, "spec", "mcp", "fleet_servers_v1.txt")
	assertFileNonEmpty(t, fleetSpecPath)
	assertFileContainsTokens(t, fleetSpecPath, []string{
		"google_calendar",
		"google_drive",
		"google_gmail",
		"notion",
		"todoist",
		"brave_search",
		"github",
		"apple_reminders",
		"duffel",
		"tesla",
	})

	wave56ScenariosPath := filepath.Join(root, "evals", "mcp", "wave56_golden_scenarios.json")
	assertFileNonEmpty(t, wave56ScenariosPath)
	assertFileContainsTokens(t, wave56ScenariosPath, []string{
		"duffel",
		"zoom",
		"calendly",
		"plaid",
		"crunchbase",
		"booking",
		"docusign",
		"canva",
		"instacart",
		"tesla",
	})

	fleetScriptPath := filepath.Join(root, "scripts", "mcp", "fleet_validation", "main.go")
	assertFileNonEmpty(t, fleetScriptPath)
	assertFileContainsTokens(t, fleetScriptPath, []string{
		"mcp_fleet_validation_v1",
		"Concurrent100CallsPassed",
		"FailoverKillFivePassed",
		"mcp_fleet_validation_report.json",
		"pickDeterministicServers",
	})

	wave56ScriptPath := filepath.Join(root, "scripts", "mcp", "wave56_checklist", "main.go")
	assertFileNonEmpty(t, wave56ScriptPath)
	assertFileContainsTokens(t, wave56ScriptPath, []string{
		"wave56_10_server_deployment_v1",
		"duffel",
		"zoom",
		"calendly",
		"plaid",
		"crunchbase",
		"booking",
		"docusign",
		"canva",
		"instacart",
		"tesla",
		"wave56_deployment_checklist_report.json",
		"critical_write_gate_enforced",
	})

	runtimeRolloutScript := filepath.Join(root, "scripts", "mcp", "runtime_rollout", "main.go")
	assertFileNonEmpty(t, runtimeRolloutScript)
	assertFileContainsTokens(t, runtimeRolloutScript, []string{
		"mcp_runtime_rollout_v1",
		"EXECUTOR_IMAGE_REPOSITORY",
		"EXECUTOR_IMAGE_TAG",
		"mcp_runtime_rollout_plan.json",
		"flag.Bool(\"execute\"",
	})
	assertFileContainsTokens(t, filepath.Join(root, "internal", "mcp", "runtime_rollout.go"), []string{
		"MCP_SERVER_ALLOWLIST",
		"MCP_RUNTIME_MODE",
		"MCP_SERVER_COUNT",
		"executor-mcp-runtime-values.yaml",
	})
}
