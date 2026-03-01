package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/mcp"
)

type wave1GoldenScenarioFile struct {
	Servers []wave1GoldenServerScenarios `json:"servers"`
}

type wave1GoldenServerScenarios struct {
	ConnectorKey string   `json:"connector_key"`
	ToolKey      string   `json:"tool_key"`
	Scenarios    []string `json:"scenarios"`
}

func TestPipelineEndToEndHappyPath(t *testing.T) {
	s := NewService("")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("whatsapp", "+15550001111", workspaceID)

	status, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550001111",
		UserChannelID:     "u1",
		Nonce:             "integration_nonce_1",
		Message:           "schedule a meeting for tomorrow",
	})
	if err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	result, err := s.ProcessNextQueuedTurn(context.Background(), false)
	if err != nil {
		t.Fatalf("process next queued turn: %v", err)
	}
	if result.GateDecision != "allow" {
		t.Fatalf("unexpected gate decision: %s", result.GateDecision)
	}
	if result.WorkflowState != "TERMINAL" {
		t.Fatalf("unexpected workflow state: %s", result.WorkflowState)
	}
	if !result.Simulated || !result.Committed {
		t.Fatalf("expected simulate+commit to both execute: %+v", result)
	}
	if result.OutboundCode != 202 {
		t.Fatalf("unexpected outbound status: %d", result.OutboundCode)
	}

	executorEvents := s.ExecutorAuditEventTypes()
	for _, event := range []string{
		"BREVIO.hands.tool.simulated.v1",
		"BREVIO.hands.tool.committed.v1",
		"BREVIO.trust.receipt.created.v1",
		"BREVIO.trust.evidence.attached.v1",
	} {
		if !containsString(executorEvents, event) {
			t.Fatalf("missing executor canonical event %s in %v", event, executorEvents)
		}
	}

	gatewayEvents := s.GatewayAuditEventTypes()
	if !containsString(gatewayEvents, "BREVIO.ingress.received.v1") {
		t.Fatalf("missing gateway canonical ingress event in %v", gatewayEvents)
	}
}

func TestPipelineEndToEndIMessagePath(t *testing.T) {
	s := NewService("")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("imessage", "imsg:user-77", workspaceID)

	status, err := s.IngestWebhook(WebhookPayload{
		Channel:           "imessage",
		ChannelIdentifier: "imsg:user-77",
		UserChannelID:     "imsg_u77",
		Nonce:             "integration_nonce_imsg_1",
		Message:           "confirm tomorrow itinerary",
	})
	if err != nil {
		t.Fatalf("ingest imessage webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	result, err := s.ProcessNextQueuedTurn(context.Background(), false)
	if err != nil {
		t.Fatalf("process queued imessage turn: %v", err)
	}
	if result.GateDecision != "allow" {
		t.Fatalf("unexpected gate decision for imessage: %s", result.GateDecision)
	}
	if !result.Simulated || !result.Committed {
		t.Fatalf("expected simulate+commit for imessage path: %+v", result)
	}
	if result.OutboundCode != 202 {
		t.Fatalf("unexpected outbound status: %d", result.OutboundCode)
	}
}

func TestPipelineBudgetExhaustionStopsBeforeCommit(t *testing.T) {
	s := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("whatsapp", "+15550002222", workspaceID)

	status, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550002222",
		UserChannelID:     "u2",
		Nonce:             "integration_nonce_2",
		Message:           "send invoice",
	})
	if err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	result, err := s.ProcessNextQueuedTurn(context.Background(), true)
	if err != nil {
		t.Fatalf("process next queued turn: %v", err)
	}
	if result.GateDecision != "deny" {
		t.Fatalf("expected deny gate decision, got %s", result.GateDecision)
	}
	if result.ReasonCode != "BUDGET_CALLS_EXHAUSTED" {
		t.Fatalf("expected budget reason code, got %s", result.ReasonCode)
	}
	if result.Committed || result.Simulated {
		t.Fatalf("expected no tool execution when budget exhausted: %+v", result)
	}
}

func TestPipelineApprovalFlowRequiresThenAllowsWithToken(t *testing.T) {
	s := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("whatsapp", "+15550003333", workspaceID)

	status, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550003333",
		UserChannelID:     "u3",
		Nonce:             "integration_nonce_3a",
		Message:           "wire payment",
	})
	if err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	blocked, err := s.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolRiskLevel: "CRITICAL",
		AutonomyLevel: "A3",
	})
	if err != nil {
		t.Fatalf("process turn without approval: %v", err)
	}
	if blocked.GateDecision != "require_approval" || !blocked.ApprovalChallenged {
		t.Fatalf("expected approval challenge, got %+v", blocked)
	}
	if blocked.Committed || blocked.Simulated {
		t.Fatalf("expected no execution without approval, got %+v", blocked)
	}

	status, err = s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550003333",
		UserChannelID:     "u3",
		Nonce:             "integration_nonce_3b",
		Message:           "wire payment",
	})
	if err != nil {
		t.Fatalf("ingest webhook second request: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	allowed, err := s.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolRiskLevel: "CRITICAL",
		AutonomyLevel: "A3",
		AutoApprove:   true,
		Now:           time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("process turn with approval: %v", err)
	}
	if allowed.GateDecision != "allow" || !allowed.ApprovalAccepted {
		t.Fatalf("expected approved execution path, got %+v", allowed)
	}
	if !allowed.Committed || allowed.ToolsCommitted != 1 {
		t.Fatalf("expected single committed tool after approval, got %+v", allowed)
	}
}

func TestPipelineMultiToolPlanCommitsAllTools(t *testing.T) {
	s := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("whatsapp", "+15550004444", workspaceID)

	status, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550004444",
		UserChannelID:     "u4",
		Nonce:             "integration_nonce_4",
		Message:           "schedule and send follow-up",
	})
	if err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected webhook status: %d", status)
	}

	result, err := s.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolKeys: []string{"calendar.create_event", "tasks.create_task"},
	})
	if err != nil {
		t.Fatalf("process multi-tool turn: %v", err)
	}
	if result.GateDecision != "allow" || !result.Committed {
		t.Fatalf("expected allowed committed path, got %+v", result)
	}
	if result.ToolsCommitted != 2 || result.ToolsSimulated != 2 {
		t.Fatalf("expected 2 simulate + 2 commit executions, got %+v", result)
	}
	if result.OutboundCode != 202 {
		t.Fatalf("unexpected outbound code: %d", result.OutboundCode)
	}

	events := s.ExecutorAuditEventTypes()
	if countString(events, "BREVIO.hands.tool.simulated.v1") < 2 {
		t.Fatalf("expected >=2 simulated events, got %v", events)
	}
	if countString(events, "BREVIO.hands.tool.committed.v1") < 2 {
		t.Fatalf("expected >=2 committed events, got %v", events)
	}
}

func TestPipelineRateCapAndBudgetCapEnforcedAcrossTurns(t *testing.T) {
	s := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	s.BindWorkspace("whatsapp", "+15550005555", workspaceID)

	if err := s.SetToolRateCap(workspaceID.String(), "calendar.create_event", 1); err != nil {
		t.Fatalf("set tool rate cap: %v", err)
	}
	if err := s.SetMonthlyBudgetCap(workspaceID.String(), 1); err != nil {
		t.Fatalf("set monthly budget cap: %v", err)
	}

	firstStatus, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550005555",
		UserChannelID:     "u5",
		Nonce:             "integration_nonce_5a",
		Message:           "create meeting",
	})
	if err != nil || firstStatus != 202 {
		t.Fatalf("ingest first webhook: status=%d err=%v", firstStatus, err)
	}

	first, err := s.ProcessNextQueuedTurn(context.Background(), false)
	if err != nil {
		t.Fatalf("process first turn: %v", err)
	}
	if first.GateDecision != "allow" || !first.Committed {
		t.Fatalf("expected first turn allowed/committed, got %+v", first)
	}

	secondStatus, err := s.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550005555",
		UserChannelID:     "u5",
		Nonce:             "integration_nonce_5b",
		Message:           "create another meeting",
	})
	if err != nil || secondStatus != 202 {
		t.Fatalf("ingest second webhook: status=%d err=%v", secondStatus, err)
	}

	second, err := s.ProcessNextQueuedTurn(context.Background(), false)
	if err != nil {
		t.Fatalf("process second turn: %v", err)
	}
	if second.GateDecision != "deny" {
		t.Fatalf("expected second turn denied by caps, got %+v", second)
	}
	if second.ReasonCode != "TOOL_RATE_CAP_EXCEEDED" {
		t.Fatalf("expected tool rate cap reason code, got %+v", second)
	}
	if second.Committed || second.Simulated {
		t.Fatalf("expected no execution when cap exceeded, got %+v", second)
	}
}

func TestMCPArchitectureInvariantsRegistryAndAuthMatrix(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	registry := svc.MCPToolRegistry()
	if len(registry) == 0 {
		t.Fatal("expected non-empty shared mcp/native tool registry")
	}
	coverage := svc.MCPAuthMatrixCoverage()
	for _, authType := range []mcp.AuthType{
		mcp.AuthOAuth2,
		mcp.AuthAPIKey,
		mcp.AuthPAT,
		mcp.AuthIntegrationToken,
	} {
		if !coverage[authType] {
			t.Fatalf("expected auth matrix coverage for %s", authType)
		}
	}
}

func TestMCPInvocationRecordedWithSharedToolExecutionPath(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d3f")
	svc.BindWorkspace("whatsapp", "+15550006666", workspaceID)

	status, err := svc.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550006666",
		UserChannelID:     "u6",
		Nonce:             "integration_nonce_6",
		Message:           "run card charge",
	})
	if err != nil || status != 202 {
		t.Fatalf("ingest webhook: status=%d err=%v", status, err)
	}

	result, err := svc.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolKeys: []string{"stripe.create_payment"},
		Now:      time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("process mcp turn: %v", err)
	}
	if result.GateDecision != "allow" || !result.Committed {
		t.Fatalf("expected committed mcp turn: %+v", result)
	}

	executions := svc.ExecutorExecutions()
	foundMCP := false
	for _, execution := range executions {
		if execution.ToolKey != "stripe.create_payment" || execution.Phase != "commit" {
			continue
		}
		if !execution.IsMCP {
			t.Fatalf("expected mcp execution record, got %+v", execution)
		}
		if execution.MCPServerID == "" {
			t.Fatalf("expected non-empty mcp_server_id, got %+v", execution)
		}
		if execution.ContentProvenance != "mcp_result" {
			t.Fatalf("expected mcp_result provenance, got %+v", execution)
		}
		foundMCP = true
	}
	if !foundMCP {
		t.Fatalf("expected mcp commit execution in %+v", executions)
	}
}

func TestSensitiveFinancialPIIRoutesToLocalModel(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d4f")
	svc.BindWorkspace("whatsapp", "+15550007777", workspaceID)

	status, err := svc.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550007777",
		UserChannelID:     "u7",
		Nonce:             "integration_nonce_7",
		Message:           "analyze sensitive account transactions",
	})
	if err != nil || status != 202 {
		t.Fatalf("ingest webhook: status=%d err=%v", status, err)
	}

	result, err := svc.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolKeys:      []string{"plaid.fetch_transactions"},
		PIIContent:    true,
		Now:           time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC),
		ToolRiskLevel: "CRITICAL",
		AutoApprove:   true,
	})
	if err != nil {
		t.Fatalf("process pii financial turn: %v", err)
	}
	if result.GateDecision != "allow" {
		t.Fatalf("expected allowed pii financial turn, got %+v", result)
	}

	executions := svc.ExecutorExecutions()
	for _, execution := range executions {
		if execution.ToolKey != "plaid.fetch_transactions" || execution.Phase != "commit" {
			continue
		}
		if execution.Provider != "local-model" {
			t.Fatalf("expected local-model provider for pii financial path, got %+v", execution)
		}
		if !execution.PIIContent {
			t.Fatalf("expected pii_content flag for financial path, got %+v", execution)
		}
		return
	}
	t.Fatalf("expected commit execution for plaid.fetch_transactions in %+v", executions)
}

func TestMCPPerServerBudgetGate(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	if err := svc.ConfigureMCPServerPolicy(mcp.ServerPolicy{
		ServerID:           "stripe_mcp",
		MonthlyCallCap:     1,
		MonthlyCostCapUSD:  0.02,
		RateLimitPerMinute: 10,
	}); err != nil {
		t.Fatalf("configure stripe policy: %v", err)
	}

	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d5f")
	svc.BindWorkspace("whatsapp", "+15550008888", workspaceID)

	for _, nonce := range []string{"integration_nonce_8a", "integration_nonce_8b"} {
		status, err := svc.IngestWebhook(WebhookPayload{
			Channel:           "whatsapp",
			ChannelIdentifier: "+15550008888",
			UserChannelID:     "u8",
			Nonce:             nonce,
			Message:           "attempt charge",
		})
		if err != nil || status != 202 {
			t.Fatalf("ingest webhook: status=%d err=%v", status, err)
		}
	}

	first, err := svc.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolKeys: []string{"stripe.create_payment"},
		Now:      time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("process first mcp billing turn: %v", err)
	}
	if first.GateDecision != "allow" {
		t.Fatalf("expected first mcp call allow, got %+v", first)
	}

	second, err := svc.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
		ToolKeys: []string{"stripe.create_payment"},
		Now:      time.Date(2026, time.March, 1, 12, 1, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("process second mcp billing turn: %v", err)
	}
	if second.GateDecision != "deny" || second.ReasonCode != "MCP_SERVER_BUDGET_EXCEEDED" {
		t.Fatalf("expected mcp budget deny, got %+v", second)
	}
}

func TestWave1GoldenScenariosThreePerServer(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "evals", "mcp", "wave1_golden_scenarios.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wave1 golden scenarios: %v", err)
	}
	var scenarios wave1GoldenScenarioFile
	if err := json.Unmarshal(raw, &scenarios); err != nil {
		t.Fatalf("decode wave1 golden scenarios: %v", err)
	}
	if len(scenarios.Servers) == 0 {
		t.Fatalf("expected non-empty wave1 scenarios from %s", path)
	}

	for serverIdx, server := range scenarios.Servers {
		server := server
		t.Run(server.ConnectorKey, func(t *testing.T) {
			t.Parallel()

			if len(server.Scenarios) < 3 {
				t.Fatalf("expected at least 3 scenarios for %s, got %d", server.ConnectorKey, len(server.Scenarios))
			}
			if server.ToolKey == "" {
				t.Fatalf("missing tool_key for %s", server.ConnectorKey)
			}

			svc := NewService("integration-secret")
			workspaceID := uuid.MustParse(fmt.Sprintf("018f3f6a-9a0f-7cc6-8f2f-%012x", serverIdx+100))
			channelIdentifier := fmt.Sprintf("wave1-%s", server.ConnectorKey)
			svc.BindWorkspace("whatsapp", channelIdentifier, workspaceID)

			for scenarioIdx, message := range server.Scenarios[:3] {
				nonce := fmt.Sprintf("wave1_%s_%d", server.ConnectorKey, scenarioIdx)
				status, err := svc.IngestWebhook(WebhookPayload{
					Channel:           "whatsapp",
					ChannelIdentifier: channelIdentifier,
					UserChannelID:     fmt.Sprintf("wave1_u_%s", server.ConnectorKey),
					Nonce:             nonce,
					Message:           message,
				})
				if err != nil || status != 202 {
					t.Fatalf("ingest wave1 scenario webhook: status=%d err=%v", status, err)
				}

				result, err := svc.ProcessNextQueuedTurnWithOptions(context.Background(), ProcessOptions{
					ToolKeys:      []string{server.ToolKey},
					AutoApprove:   true,
					ToolRiskLevel: "CRITICAL",
					Now:           time.Date(2026, time.March, 1, 14, scenarioIdx, 0, 0, time.UTC),
				})
				if err != nil {
					t.Fatalf("process wave1 scenario turn: %v", err)
				}
				if result.GateDecision != "allow" || !result.Committed || result.ToolsCommitted != 1 {
					t.Fatalf("expected committed allow result for %s scenario %d: %+v", server.ConnectorKey, scenarioIdx, result)
				}
			}

			foundCommit := false
			for _, execution := range svc.ExecutorExecutions() {
				if execution.ToolKey != server.ToolKey || execution.Phase != "commit" {
					continue
				}
				foundCommit = true
				if !execution.IsMCP {
					t.Fatalf("expected mcp commit execution for %s: %+v", server.ToolKey, execution)
				}
				if execution.ContentProvenance != "mcp_result" {
					t.Fatalf("expected mcp_result provenance for %s: %+v", server.ToolKey, execution)
				}
			}
			if !foundCommit {
				t.Fatalf("expected at least one commit execution for %s", server.ToolKey)
			}
		})
	}
}

func TestWorkflowIntegrationProvisioningCompensation(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	result := svc.RunProvisioningWorkflow(context.Background(), "DeployServer")
	if result.Status != "failed" {
		t.Fatalf("expected failed provisioning status, got %s", result.Status)
	}
	if len(result.CompensatedSteps) == 0 {
		t.Fatalf("expected reverse compensation steps, got %+v", result)
	}
	if result.CompensatedSteps[0] != "DeployServer" {
		t.Fatalf("expected failed step compensation first, got %v", result.CompensatedSteps)
	}
}

func TestWorkflowIntegrationProvisioningArtifactVerificationFailure(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	result := svc.RunProvisioningWorkflow(context.Background(), "VerifyArtifact")
	if result.Status != "failed" {
		t.Fatalf("expected failed provisioning status, got %s", result.Status)
	}
	if len(result.CompensatedSteps) == 0 {
		t.Fatalf("expected compensation steps, got %+v", result)
	}
	if result.CompensatedSteps[0] != "VerifyArtifact" {
		t.Fatalf("expected failed VerifyArtifact to compensate first, got %v", result.CompensatedSteps)
	}
}

func TestWorkflowIntegrationProvisioningFailureInjectionAllSteps(t *testing.T) {
	t.Parallel()

	steps := []string{
		"Preflight",
		"CreateRequest",
		"PolicyGate",
		"AllocateOrReuseServer",
		"VerifyArtifact",
		"DeployServer",
		"FetchToolSchemas",
		"HealthCheck",
		"CommitRegistry",
		"Active",
	}

	svc := NewService("integration-secret")
	for idx, step := range steps {
		result := svc.RunProvisioningWorkflow(context.Background(), step)
		if result.Status != "failed" {
			t.Fatalf("expected failed provisioning status at step=%s, got %s", step, result.Status)
		}
		if len(result.ExecutedSteps) != idx+1 {
			t.Fatalf("unexpected executed step count at step=%s: got=%d want=%d", step, len(result.ExecutedSteps), idx+1)
		}
		if len(result.CompensatedSteps) != len(result.ExecutedSteps) {
			t.Fatalf("expected full reverse compensation at step=%s, executed=%v compensated=%v", step, result.ExecutedSteps, result.CompensatedSteps)
		}
		if result.CompensatedSteps[0] != step {
			t.Fatalf("expected first compensation to be failed step at step=%s, got=%s", step, result.CompensatedSteps[0])
		}
	}
}

func TestWorkflowIntegrationOnboardingAndDrift(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	onboarding := svc.RunOnboardingWorkflow(context.Background(), map[string]string{
		"operator_profile_intake_v1":     "team=ops",
		"behavior_policy_calibration_v1": "strict",
		"codebase_map_ingestion_v1":      "repo=backend",
		"system_map_ingestion_v1":        "services=gateway,control,executor",
	})
	if onboarding.Status != "completed" || len(onboarding.CompletedStages) != 4 {
		t.Fatalf("expected onboarding completion, got %+v", onboarding)
	}

	drift := svc.RunDriftWorkflow(context.Background(), true)
	if drift != "quarantined" {
		t.Fatalf("expected drift quarantine result, got %s", drift)
	}
}

func TestWorkflowIntegrationV91AndV92Executions(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	if status := svc.RunDailyCaptureWorkflow(context.Background(), "cron"); status != "completed" {
		t.Fatalf("expected daily capture completion, got %s", status)
	}
	if status := svc.RunRAGEvalWorkflow(context.Background(), 0.82, 0.78); status != "passed" {
		t.Fatalf("expected rag eval pass status, got %s", status)
	}
}

func TestCrossCuttingKeyRotationDualWindow(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	before, during, expired, err := svc.VerifyPIIRotationDualKeyWindow(time.Date(2026, time.February, 28, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("verify pii rotation window: %v", err)
	}
	if !before || !during || !expired {
		t.Fatalf("expected key-rotation lifecycle coverage true/true/true, got before=%t during=%t expired=%t", before, during, expired)
	}
}

func TestCrossCuttingCircuitBreakerTransitions(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	opened, closed := svc.EvaluateCircuitBreakerTransition("ws_cb", "providerA", time.Date(2026, time.February, 28, 12, 0, 0, 0, time.UTC))
	if !opened {
		t.Fatal("expected circuit to open after failure threshold")
	}
	if !closed {
		t.Fatal("expected circuit to close after cooldown")
	}
}

func TestCrossCuttingSSRFBlockedCIDRs(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	blockedTargets := []string{
		"http://169.254.169.254/latest/meta-data",
		"http://127.0.0.1:8080/admin",
		"http://10.0.0.1/internal",
		"http://172.16.9.9/private",
		"http://192.168.1.44/private",
	}
	for _, target := range blockedTargets {
		if err := svc.ValidateExecutorSSRFBlock(target); err == nil {
			t.Fatalf("expected blocked ssrf target: %s", target)
		}
	}
}

func containsString(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func countString(items []string, needle string) int {
	count := 0
	for _, item := range items {
		if item == needle {
			count++
		}
	}
	return count
}
