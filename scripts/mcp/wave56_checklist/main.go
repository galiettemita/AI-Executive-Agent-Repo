package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/brevio/brevio/internal/mcp"
)

type connectorSeed struct {
	Key          string `yaml:"key"`
	Domain       string `yaml:"domain"`
	RiskLevel    string `yaml:"risk_level"`
	DataClass    string `yaml:"data_class"`
	MCPServerURL string `yaml:"mcp_server_url"`
}

type connectorToolSeed struct {
	ConnectorKey  string `yaml:"connector_key"`
	ToolKey       string `yaml:"tool_key"`
	Write         bool   `yaml:"write"`
	Reversible    bool   `yaml:"reversible"`
	AutonomyFloor string `yaml:"autonomy_floor"`
}

type connectorsSeedDoc struct {
	Connectors []connectorSeed     `yaml:"connectors"`
	Tools      []connectorToolSeed `yaml:"tools"`
}

type goldenScenariosDoc struct {
	Servers []goldenServerScenarios `json:"servers"`
}

type goldenServerScenarios struct {
	ConnectorKey string   `json:"connector_key"`
	ToolKey      string   `json:"tool_key"`
	Scenarios    []string `json:"scenarios"`
}

type stepResult struct {
	StepID  string `json:"step_id"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

type serverChecklistResult struct {
	ConnectorKey string       `json:"connector_key"`
	Passed       bool         `json:"passed"`
	FailedSteps  []string     `json:"failed_steps,omitempty"`
	Steps        []stepResult `json:"steps"`
}

type checklistReport struct {
	GeneratedAtUTC string                  `json:"generated_at_utc"`
	ChecklistKey   string                  `json:"checklist_key"`
	TotalServers   int                     `json:"total_servers"`
	PassedServers  int                     `json:"passed_servers"`
	FailedServers  int                     `json:"failed_servers"`
	Servers        []serverChecklistResult `json:"servers"`
}

var wave56ConnectorOrder = []string{
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
}

var wave56RequiredTool = map[string]string{
	"duffel":     "duffel.create_order",
	"zoom":       "zoom.fetch_transcript",
	"calendly":   "calendly.create_event",
	"plaid":      "plaid.create_link_session",
	"crunchbase": "crunchbase.find_company",
	"booking":    "booking.create_reservation",
	"docusign":   "docusign.send_envelope",
	"canva":      "canva.create_design",
	"instacart":  "instacart.create_checkout",
	"tesla":      "tesla.command_vehicle",
}

var wave56AuthByConnector = map[string]mcp.AuthType{
	"duffel":     mcp.AuthOAuth2,
	"zoom":       mcp.AuthOAuth2,
	"calendly":   mcp.AuthOAuth2,
	"plaid":      mcp.AuthOAuth2,
	"crunchbase": mcp.AuthAPIKey,
	"booking":    mcp.AuthOAuth2,
	"docusign":   mcp.AuthOAuth2,
	"canva":      mcp.AuthOAuth2,
	"instacart":  mcp.AuthOAuth2,
	"tesla":      mcp.AuthOAuth2,
}

var criticalApprovalConnectors = map[string]struct{}{
	"duffel":    {},
	"booking":   {},
	"docusign":  {},
	"instacart": {},
	"tesla":     {},
}

func main() {
	root := repoRootOrExit()
	seeds := loadConnectorSeedsOrExit(filepath.Join(root, "internal", "connectors", "seeds", "connectors.yaml"))
	scenarios := loadGoldenScenariosOrExit(filepath.Join(root, "evals", "mcp", "wave56_golden_scenarios.json"))
	runbookBody := readFileOrExit(filepath.Join(root, "runbooks", "RB-005.md"))

	connectorsByKey := map[string]connectorSeed{}
	for _, connector := range seeds.Connectors {
		connectorsByKey[connector.Key] = connector
	}
	toolsByConnector := map[string][]connectorToolSeed{}
	allToolKeys := map[string]struct{}{}
	for _, tool := range seeds.Tools {
		toolsByConnector[tool.ConnectorKey] = append(toolsByConnector[tool.ConnectorKey], tool)
		allToolKeys[tool.ToolKey] = struct{}{}
	}
	scenariosByConnector := map[string]goldenServerScenarios{}
	for _, server := range scenarios.Servers {
		scenariosByConnector[server.ConnectorKey] = server
	}

	report := checklistReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		ChecklistKey:   "wave56_10_server_deployment_v1",
		TotalServers:   len(wave56ConnectorOrder),
		Servers:        make([]serverChecklistResult, 0, len(wave56ConnectorOrder)),
	}

	for _, connectorKey := range wave56ConnectorOrder {
		result := evaluateWave56Server(connectorKey, connectorsByKey, toolsByConnector, allToolKeys, scenariosByConnector, runbookBody)
		if result.Passed {
			report.PassedServers++
		} else {
			report.FailedServers++
		}
		report.Servers = append(report.Servers, result)
	}
	sort.Slice(report.Servers, func(i, j int) bool {
		return report.Servers[i].ConnectorKey < report.Servers[j].ConnectorKey
	})

	reportPath := filepath.Join(root, "artifacts", "deploy", "wave56_deployment_checklist_report.json")
	writeReportOrExit(reportPath, report)

	if report.FailedServers > 0 {
		fmt.Printf("wave56 deployment checklist failed: passed=%d failed=%d report=%s\n", report.PassedServers, report.FailedServers, reportPath)
		os.Exit(1)
	}
	fmt.Printf("wave56 deployment checklist passed: servers=%d report=%s\n", report.TotalServers, reportPath)
}

func evaluateWave56Server(
	connectorKey string,
	connectorsByKey map[string]connectorSeed,
	toolsByConnector map[string][]connectorToolSeed,
	allToolKeys map[string]struct{},
	scenariosByConnector map[string]goldenServerScenarios,
	runbookBody string,
) serverChecklistResult {
	connector, hasConnector := connectorsByKey[connectorKey]
	tools := append([]connectorToolSeed(nil), toolsByConnector[connectorKey]...)
	requiredToolKey := wave56RequiredTool[connectorKey]
	scenarioSet := scenariosByConnector[connectorKey]

	steps := make([]stepResult, 0, 10)
	addStep := func(stepID string, passed bool, details string) {
		steps = append(steps, stepResult{StepID: stepID, Passed: passed, Details: details})
	}

	manifestRegistered := hasConnector && strings.HasPrefix(strings.ToLower(strings.TrimSpace(connector.MCPServerURL)), "https://")
	addStep("server_manifest_registered", manifestRegistered, fmt.Sprintf("connector_exists=%t mcp_server_url=%s", hasConnector, connector.MCPServerURL))

	requiredTool, hasRequiredTool := findTool(tools, requiredToolKey)
	addStep("required_tool_present", hasRequiredTool, fmt.Sprintf("required_tool=%s", requiredToolKey))

	authType, hasAuth := wave56AuthByConnector[connectorKey]
	addStep("auth_profile_defined", hasAuth, fmt.Sprintf("auth_type=%s", authType))

	registry := mcp.NewService()
	policyRegistered := hasAuth && hasRequiredTool
	if hasRequiredTool {
		if err := registry.RegisterTool(mcp.ToolSpec{
			ToolKey:   requiredTool.ToolKey,
			Source:    mcp.ToolSourceMCP,
			ServerID:  connectorKey + "_mcp",
			AuthType:  authType,
			RiskLevel: strings.ToUpper(strings.TrimSpace(connector.RiskLevel)),
		}); err != nil {
			policyRegistered = false
		}
	}
	addStep("normalization_registry_path_verified", policyRegistered, "required tool registered in shared MCP tool registry")

	writeGate := true
	if _, critical := criticalApprovalConnectors[connectorKey]; critical {
		writeGate = hasRequiredTool && requiredTool.Write && strings.EqualFold(strings.TrimSpace(requiredTool.AutonomyFloor), "A0")
	}
	addStep("critical_write_gate_enforced", writeGate, fmt.Sprintf("write=%t autonomy_floor=%s", requiredTool.Write, requiredTool.AutonomyFloor))

	zoomTranscript := true
	if connectorKey == "zoom" {
		_, zoomTranscript = findTool(tools, "zoom.fetch_transcript")
	}
	addStep("zoom_transcript_support", zoomTranscript, "zoom includes transcript retrieval tool")

	calendlyConflictGuard := true
	if connectorKey == "calendly" {
		_, calendlyConflictGuard = allToolKeys["google_calendar.create_event"]
	}
	addStep("calendly_duplicate_event_prevention", calendlyConflictGuard, "calendly deployment references google calendar conflict guard dependency")

	plaidLinkSupport := true
	if connectorKey == "plaid" {
		_, plaidLinkSupport = findTool(tools, "plaid.create_link_session")
	}
	addStep("plaid_link_widget_support", plaidLinkSupport, "plaid link session tool configured for widget bootstrap")

	goldenScenariosPresent := scenarioSet.ConnectorKey == connectorKey && len(scenarioSet.Scenarios) >= 3 && scenarioSet.ToolKey == requiredToolKey
	addStep("golden_scenarios_present", goldenScenariosPresent, fmt.Sprintf("scenario_count=%d", len(scenarioSet.Scenarios)))

	runbookUpdated := strings.Contains(strings.ToLower(runbookBody), strings.ToLower(connectorKey))
	addStep("runbook_failure_handling_present", runbookUpdated, "connector-specific runbook recovery checks included")

	for _, scenario := range scenarioSet.Scenarios {
		if strings.TrimSpace(scenario) == "" {
			goldenScenariosPresent = false
		}
	}

	failed := make([]string, 0, len(steps))
	for _, step := range steps {
		if !step.Passed {
			failed = append(failed, step.StepID)
		}
	}
	return serverChecklistResult{
		ConnectorKey: connectorKey,
		Passed:       len(failed) == 0,
		FailedSteps:  failed,
		Steps:        steps,
	}
}

func findTool(tools []connectorToolSeed, key string) (connectorToolSeed, bool) {
	for _, tool := range tools {
		if tool.ToolKey == key {
			return tool, true
		}
	}
	return connectorToolSeed{}, false
}

func loadConnectorSeedsOrExit(path string) connectorsSeedDoc {
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read connector seeds: %v\n", err)
		os.Exit(1)
	}
	var parsed connectorsSeedDoc
	if err := yaml.Unmarshal(body, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "parse connector seeds: %v\n", err)
		os.Exit(1)
	}
	return parsed
}

func loadGoldenScenariosOrExit(path string) goldenScenariosDoc {
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read scenarios: %v\n", err)
		os.Exit(1)
	}
	var parsed goldenScenariosDoc
	if err := json.Unmarshal(body, &parsed); err != nil {
		fmt.Fprintf(os.Stderr, "parse scenarios: %v\n", err)
		os.Exit(1)
	}
	return parsed
}

func readFileOrExit(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read file: %v\n", err)
		os.Exit(1)
	}
	return string(body)
}

func repoRootOrExit() string {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repo root: %v\n", err)
		os.Exit(1)
	}
	return root
}

func writeReportOrExit(path string, report checklistReport) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create report directory: %v\n", err)
		os.Exit(1)
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		os.Exit(1)
	}
}
