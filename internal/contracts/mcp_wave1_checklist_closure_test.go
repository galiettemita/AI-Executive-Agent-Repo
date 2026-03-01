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
		"mcp-fleet-validate:",
		"./scripts/mcp/fleet_validation/main.go",
		"ci: lint build test migrate api-docs-check tools-md-check mcp-wave1-checklist mcp-fleet-validate contracts acceptance",
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

	fleetScriptPath := filepath.Join(root, "scripts", "mcp", "fleet_validation", "main.go")
	assertFileNonEmpty(t, fleetScriptPath)
	assertFileContainsTokens(t, fleetScriptPath, []string{
		"mcp_fleet_validation_v1",
		"Concurrent100CallsPassed",
		"FailoverKillFivePassed",
		"mcp_fleet_validation_report.json",
		"pickDeterministicServers",
	})
}
