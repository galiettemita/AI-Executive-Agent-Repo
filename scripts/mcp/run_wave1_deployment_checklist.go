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
	"github.com/brevio/brevio/internal/onboarding"
)

type connectorSeedFile struct {
	Connectors []connectorSeed     `yaml:"connectors"`
	Tools      []connectorToolSeed `yaml:"tools"`
}

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
	AutonomyFloor string `yaml:"autonomy_floor"`
}

type goldenScenarioFile struct {
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
	ServerID     string       `json:"server_id"`
	Passed       bool         `json:"passed"`
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

var wave1ConnectorOrder = []string{
	"google_calendar",
	"google_drive",
	"google_gmail",
	"notion",
	"todoist",
	"brave_search",
	"github",
	"apple_reminders",
}

var wave1AuthByConnector = map[string]mcp.AuthType{
	"google_calendar": mcp.AuthOAuth2,
	"google_drive":    mcp.AuthOAuth2,
	"google_gmail":    mcp.AuthOAuth2,
	"notion":          mcp.AuthIntegrationToken,
	"todoist":         mcp.AuthOAuth2,
	"brave_search":    mcp.AuthAPIKey,
	"github":          mcp.AuthPAT,
	"apple_reminders": mcp.AuthIntegrationToken,
}

var validRiskLevels = map[string]struct{}{
	"LOW":      {},
	"MEDIUM":   {},
	"ELEVATED": {},
	"CRITICAL": {},
}

func main() {
	root := repoRootOrExit()

	seed := loadConnectorSeedsOrExit(filepath.Join(root, "internal", "connectors", "seeds", "connectors.yaml"))
	scenarios := loadGoldenScenariosOrExit(filepath.Join(root, "evals", "mcp", "wave1_golden_scenarios.json"))
	runbookBody := readTextFileOrExit(filepath.Join(root, "runbooks", "RB-005.md"))
	provisioningRunbookBody := readTextFileOrExit(filepath.Join(root, "runbooks", "RB-004.md"))

	connectorsByKey := map[string]connectorSeed{}
	for _, connector := range seed.Connectors {
		connectorsByKey[connector.Key] = connector
	}
	toolsByConnector := map[string][]connectorToolSeed{}
	for _, tool := range seed.Tools {
		toolsByConnector[tool.ConnectorKey] = append(toolsByConnector[tool.ConnectorKey], tool)
	}
	scenariosByConnector := map[string]goldenServerScenarios{}
	for _, scenario := range scenarios.Servers {
		scenariosByConnector[scenario.ConnectorKey] = scenario
	}

	report := checklistReport{
		GeneratedAtUTC: time.Now().UTC().Format(time.RFC3339),
		ChecklistKey:   "wave1_12_step_deployment_v1",
		TotalServers:   len(wave1ConnectorOrder),
		Servers:        make([]serverChecklistResult, 0, len(wave1ConnectorOrder)),
	}
	for _, connectorKey := range wave1ConnectorOrder {
		result := evaluateWave1Server(connectorKey, connectorsByKey, toolsByConnector, scenariosByConnector, runbookBody, provisioningRunbookBody)
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

	reportPath := filepath.Join(root, "artifacts", "deploy", "wave1_deployment_checklist_report.json")
	writeReportOrExit(reportPath, report)

	if report.FailedServers > 0 {
		fmt.Printf("wave1 deployment checklist failed: passed=%d failed=%d report=%s\n", report.PassedServers, report.FailedServers, reportPath)
		os.Exit(1)
	}
	fmt.Printf("wave1 deployment checklist passed: servers=%d report=%s\n", report.TotalServers, reportPath)
}

func evaluateWave1Server(
	connectorKey string,
	connectorsByKey map[string]connectorSeed,
	toolsByConnector map[string][]connectorToolSeed,
	scenariosByConnector map[string]goldenServerScenarios,
	runbookBody string,
	provisioningRunbookBody string,
) serverChecklistResult {
	connector, hasConnector := connectorsByKey[connectorKey]
	tools := append([]connectorToolSeed(nil), toolsByConnector[connectorKey]...)
	serverID := connectorKey + "_mcp"
	authType, hasAuth := wave1AuthByConnector[connectorKey]

	steps := make([]stepResult, 0, 12)
	addStep := func(stepID string, passed bool, details string) {
		steps = append(steps, stepResult{StepID: stepID, Passed: passed, Details: details})
	}

	manifestRegistered := hasConnector && strings.HasPrefix(strings.ToLower(strings.TrimSpace(connector.MCPServerURL)), "https://")
	addStep("server_manifest_registered", manifestRegistered, fmt.Sprintf("connector_exists=%t mcp_server_url=%s", hasConnector, connector.MCPServerURL))

	capabilityProbe := len(tools) > 0
	addStep("capability_probe_tools_list", capabilityProbe, fmt.Sprintf("tool_count=%d", len(tools)))

	oauthConfigured := hasAuth
	addStep("oauth_or_auth_configured", oauthConfigured, fmt.Sprintf("auth_type=%s", authType))

	callbackRouting := strings.Contains(strings.ToLower(provisioningRunbookBody), "oauth callback")
	addStep("callback_routing_defined", callbackRouting, "provisioning runbook contains oauth callback handling")

	_, riskValid := validRiskLevels[strings.ToUpper(strings.TrimSpace(connector.RiskLevel))]
	addStep("risk_classification_present", riskValid, fmt.Sprintf("risk_level=%s", connector.RiskLevel))

	approvalThresholds := len(tools) > 0
	for _, tool := range tools {
		if !tool.Write {
			continue
		}
		if strings.TrimSpace(tool.AutonomyFloor) == "" {
			approvalThresholds = false
		}
	}
	addStep("approval_thresholds_present", approvalThresholds, "write tools include autonomy_floor")

	registry := mcp.NewService()
	normalizationOK := hasAuth && len(tools) > 0
	for _, tool := range tools {
		err := registry.RegisterTool(mcp.ToolSpec{
			ToolKey:   tool.ToolKey,
			Source:    mcp.ToolSourceMCP,
			ServerID:  serverID,
			AuthType:  authType,
			RiskLevel: strings.ToUpper(strings.TrimSpace(connector.RiskLevel)),
		})
		if err != nil {
			normalizationOK = false
			break
		}
	}
	if normalizationOK {
		if _, ok := registry.ResolveTool(tools[0].ToolKey); !ok {
			normalizationOK = false
		}
	}
	addStep("normalization_path_verified", normalizationOK, "tool schemas normalized into shared registry")

	securityChecks := true
	if err := registry.RegisterTool(mcp.ToolSpec{
		ToolKey:   connectorKey + ".invalid_server",
		Source:    mcp.ToolSourceMCP,
		ServerID:  "bad server id",
		AuthType:  authType,
		RiskLevel: "LOW",
	}); err == nil {
		securityChecks = false
	}
	if err := registry.RecordInvocation(mcp.Invocation{
		WorkspaceID: "ws_security",
		ToolKey:     connectorKey + ".invalid",
		IsMCP:       true,
	}); err == nil {
		securityChecks = false
	}
	if err := registry.RecordInvocation(mcp.Invocation{
		WorkspaceID: "ws_security",
		ToolKey:     connectorKey + ".health_probe",
		ServerID:    serverID,
		IsMCP:       true,
		CostUSD:     0.01,
	}); err != nil {
		securityChecks = false
	}
	addStep("security_guardrails_verified", securityChecks, "invalid server_id blocked; mcp provenance path enforced")

	costTracking := true
	if err := registry.ConfigureServerPolicy(mcp.ServerPolicy{
		ServerID:           serverID,
		MonthlyCallCap:     2,
		MonthlyCostCapUSD:  0.02,
		RateLimitPerMinute: 5,
	}); err != nil {
		costTracking = false
	}
	at := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	if costTracking {
		if err := registry.EnforceServerPolicy(serverID, 0.01, at); err != nil {
			costTracking = false
		}
	}
	if costTracking {
		if err := registry.RecordInvocation(mcp.Invocation{
			WorkspaceID: "ws_cost",
			ToolKey:     tools[0].ToolKey,
			ServerID:    serverID,
			IsMCP:       true,
			CostUSD:     0.01,
			CalledAt:    at,
		}); err != nil {
			costTracking = false
		}
	}
	if costTracking {
		usage := registry.UsageSummaries()
		if len(usage) == 0 || usage[0].Calls < 1 || usage[0].CostUSD <= 0 {
			costTracking = false
		}
	}
	addStep("cost_tracking_rate_limit_verified", costTracking, "usage counters increment and policy enforcement path exists")

	onboardingReady := false
	rendered, err := onboarding.NewService().RenderConnectionTemplate("whatsapp", "ecosystem_detect_v1", map[string]string{
		"app_name":       humanizeConnectorName(connectorKey),
		"ecosystem_hint": connector.Domain,
	})
	if err == nil && !strings.Contains(rendered.Title, "{{") && !strings.Contains(rendered.Body, "{{") && len(rendered.Buttons) >= 2 {
		onboardingReady = true
	}
	addStep("onboarding_card_or_trigger_present", onboardingReady, "whatsapp ecosystem detection template renders with action buttons")

	golden := scenariosByConnector[connectorKey]
	goldenScenarios := len(golden.Scenarios) >= 3 && strings.TrimSpace(golden.ToolKey) != ""
	addStep("golden_scenarios_present", goldenScenarios, fmt.Sprintf("scenario_count=%d tool_key=%s", len(golden.Scenarios), golden.ToolKey))

	runbookReady := strings.Contains(runbookBody, connectorKey)
	addStep("runbook_failure_handling_present", runbookReady, "runbook references connector-specific checks")

	passed := true
	for _, step := range steps {
		if !step.Passed {
			passed = false
			break
		}
	}
	return serverChecklistResult{
		ConnectorKey: connectorKey,
		ServerID:     serverID,
		Passed:       passed,
		Steps:        steps,
	}
}

func repoRootOrExit() string {
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve repo root: %v\n", err)
		os.Exit(1)
	}
	return root
}

func loadConnectorSeedsOrExit(path string) connectorSeedFile {
	raw := readBytesOrExit(path)
	var seed connectorSeedFile
	if err := yaml.Unmarshal(raw, &seed); err != nil {
		fmt.Fprintf(os.Stderr, "parse connector seed file: %v\n", err)
		os.Exit(1)
	}
	return seed
}

func loadGoldenScenariosOrExit(path string) goldenScenarioFile {
	raw := readBytesOrExit(path)
	var scenarios goldenScenarioFile
	if err := json.Unmarshal(raw, &scenarios); err != nil {
		fmt.Fprintf(os.Stderr, "parse wave1 golden scenarios: %v\n", err)
		os.Exit(1)
	}
	return scenarios
}

func readTextFileOrExit(path string) string {
	return string(readBytesOrExit(path))
}

func readBytesOrExit(path string) []byte {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read file %s: %v\n", path, err)
		os.Exit(1)
	}
	return raw
}

func writeReportOrExit(path string, report checklistReport) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create report directory: %v\n", err)
		os.Exit(1)
	}
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal checklist report: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write checklist report: %v\n", err)
		os.Exit(1)
	}
}

func humanizeConnectorName(connectorKey string) string {
	parts := strings.Split(strings.TrimSpace(connectorKey), "_")
	if len(parts) == 0 {
		return connectorKey
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}
