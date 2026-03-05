package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/executor"
	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/llm"
	"github.com/brevio/brevio/internal/mcp"
	"github.com/brevio/brevio/internal/security/pii"
	"github.com/brevio/brevio/internal/workflows"
)

type WebhookPayload struct {
	Channel           string `json:"channel"`
	ChannelIdentifier string `json:"channel_identifier"`
	UserChannelID     string `json:"user_channel_id"`
	Nonce             string `json:"nonce"`
	Message           string `json:"message"`
}

type PipelineResult struct {
	GateDecision       string
	ReasonCode         string
	WorkflowState      string
	Simulated          bool
	Committed          bool
	ToolsSimulated     int
	ToolsCommitted     int
	ApprovalChallenged bool
	ApprovalAccepted   bool
	OutboundCode       int
}

type ProcessOptions struct {
	BudgetExhausted bool
	RateLimited     bool
	AutonomyLevel   string
	ToolRiskLevel   string
	ToolKeys        []string
	AutoApprove     bool
	PIIContent      bool
	Now             time.Time
}

type Service struct {
	secret      string
	gateway     *gateway.Service
	gatewayMux  *http.ServeMux
	control     *control.Service
	llm         *llm.Service
	workflows   *workflows.Service
	executor    *executor.Service
	mcp         *mcp.Service
	pii         *pii.Service
	defaultRisk string
}

func NewService(secret string) *Service {
	if secret == "" {
		secret = "dev-secret"
	}
	return &Service{
		secret:      secret,
		gateway:     gateway.NewService(secret),
		control:     control.NewService(secret),
		llm:         llm.NewService(),
		workflows:   workflows.NewService(),
		executor:    executor.NewService(),
		mcp:         seedDefaultMCPRegistry(),
		pii:         pii.NewService(),
		defaultRisk: "LOW",
	}
}

func seedDefaultMCPRegistry() *mcp.Service {
	registry := mcp.NewService()
	defaultTools := []mcp.ToolSpec{
		{ToolKey: "calendar.create_event", Source: mcp.ToolSourceNative, RiskLevel: "MEDIUM"},
		{ToolKey: "email.send", Source: mcp.ToolSourceNative, RiskLevel: "MEDIUM"},
		// Wave 1 MCP server set.
		{ToolKey: "google_calendar.create_event", Source: mcp.ToolSourceMCP, ServerID: "google_calendar_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "MEDIUM"},
		{ToolKey: "google_drive.upload_file", Source: mcp.ToolSourceMCP, ServerID: "google_drive_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "LOW"},
		{ToolKey: "google_gmail.send_email", Source: mcp.ToolSourceMCP, ServerID: "google_gmail_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "MEDIUM"},
		{ToolKey: "notion.create_page", Source: mcp.ToolSourceMCP, ServerID: "notion_mcp", AuthType: mcp.AuthIntegrationToken, RiskLevel: "LOW"},
		{ToolKey: "todoist.create_task", Source: mcp.ToolSourceMCP, ServerID: "todoist_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "MEDIUM"},
		{ToolKey: "brave_search.query", Source: mcp.ToolSourceMCP, ServerID: "brave_search_mcp", AuthType: mcp.AuthAPIKey, RiskLevel: "LOW"},
		{ToolKey: "github.create_issue", Source: mcp.ToolSourceMCP, ServerID: "github_mcp", AuthType: mcp.AuthPAT, RiskLevel: "ELEVATED"},
		{ToolKey: "apple_reminders.create_reminder", Source: mcp.ToolSourceMCP, ServerID: "apple_reminders_mcp", AuthType: mcp.AuthIntegrationToken, RiskLevel: "MEDIUM"},
		{ToolKey: "stripe.create_payment", Source: mcp.ToolSourceMCP, ServerID: "stripe_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "plaid.fetch_transactions", Source: mcp.ToolSourceMCP, ServerID: "plaid_mcp", AuthType: mcp.AuthAPIKey, RiskLevel: "CRITICAL"},
		{ToolKey: "plaid.create_link_session", Source: mcp.ToolSourceMCP, ServerID: "plaid_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "zoom.fetch_transcript", Source: mcp.ToolSourceMCP, ServerID: "zoom_mcp", AuthType: mcp.AuthPAT, RiskLevel: "MEDIUM"},
		{ToolKey: "calendly.create_event", Source: mcp.ToolSourceMCP, ServerID: "calendly_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "MEDIUM"},
		{ToolKey: "duffel.create_order", Source: mcp.ToolSourceMCP, ServerID: "duffel_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "crunchbase.find_company", Source: mcp.ToolSourceMCP, ServerID: "crunchbase_mcp", AuthType: mcp.AuthAPIKey, RiskLevel: "LOW"},
		{ToolKey: "booking.create_reservation", Source: mcp.ToolSourceMCP, ServerID: "booking_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "docusign.send_envelope", Source: mcp.ToolSourceMCP, ServerID: "docusign_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "canva.create_design", Source: mcp.ToolSourceMCP, ServerID: "canva_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "MEDIUM"},
		{ToolKey: "instacart.create_checkout", Source: mcp.ToolSourceMCP, ServerID: "instacart_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "tesla.command_vehicle", Source: mcp.ToolSourceMCP, ServerID: "tesla_mcp", AuthType: mcp.AuthOAuth2, RiskLevel: "CRITICAL"},
		{ToolKey: "slack.post_message", Source: mcp.ToolSourceMCP, ServerID: "slack_mcp", AuthType: mcp.AuthIntegrationToken, RiskLevel: "LOW"},
	}
	for _, tool := range defaultTools {
		_ = registry.RegisterTool(tool)
	}
	defaultPolicies := []mcp.ServerPolicy{
		{ServerID: "google_calendar_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "google_drive_mcp", MonthlyCallCap: 5000, MonthlyCostCapUSD: 40, RateLimitPerMinute: 120},
		{ServerID: "google_gmail_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "notion_mcp", MonthlyCallCap: 5000, MonthlyCostCapUSD: 40, RateLimitPerMinute: 120},
		{ServerID: "todoist_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "brave_search_mcp", MonthlyCallCap: 5000, MonthlyCostCapUSD: 40, RateLimitPerMinute: 120},
		{ServerID: "github_mcp", MonthlyCallCap: 1500, MonthlyCostCapUSD: 120, RateLimitPerMinute: 60},
		{ServerID: "apple_reminders_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "stripe_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "plaid_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "zoom_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "calendly_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "duffel_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "crunchbase_mcp", MonthlyCallCap: 5000, MonthlyCostCapUSD: 40, RateLimitPerMinute: 120},
		{ServerID: "booking_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "docusign_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "canva_mcp", MonthlyCallCap: 2000, MonthlyCostCapUSD: 80, RateLimitPerMinute: 90},
		{ServerID: "instacart_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "tesla_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
		{ServerID: "slack_mcp", MonthlyCallCap: 1000, MonthlyCostCapUSD: 200, RateLimitPerMinute: 30},
	}
	for _, policy := range defaultPolicies {
		_ = registry.ConfigureServerPolicy(policy)
	}
	return registry
}

func (s *Service) BindWorkspace(channel, identifier string, workspaceID uuid.UUID) {
	s.gateway.BindWorkspace(channel, identifier, workspaceID)
	if s.gatewayMux == nil {
		s.gatewayMux = gateway.NewMux(s.gateway)
	}
}

func (s *Service) IngestWebhook(payload WebhookPayload) (int, error) {
	if s.gatewayMux == nil {
		s.gatewayMux = gateway.NewMux(s.gateway)
	}
	blob, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	path := "/v1/gateway/webhook/whatsapp"
	imessage := false
	if strings.EqualFold(strings.TrimSpace(payload.Channel), "imessage") {
		path = "/webhooks/imessage"
		imessage = true
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(blob))
	req.Header.Set("X-Signature", signPayload([]byte(s.secret), blob))
	if imessage {
		req.Header.Set("X-API-Key", integrationIMessageAPIKey())
	}
	resp := httptest.NewRecorder()
	s.gatewayMux.ServeHTTP(resp, req)
	return resp.Code, nil
}

func (s *Service) IngestWebhookRaw(channel string, payload []byte, signatureOverride string) (int, error) {
	if s.gatewayMux == nil {
		s.gatewayMux = gateway.NewMux(s.gateway)
	}
	path := "/v1/gateway/webhook/whatsapp"
	imessage := false
	if strings.EqualFold(strings.TrimSpace(channel), "imessage") {
		path = "/webhooks/imessage"
		imessage = true
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	signature := strings.TrimSpace(signatureOverride)
	if signature == "" {
		signature = signPayload([]byte(s.secret), payload)
	}
	req.Header.Set("X-Signature", signature)
	if imessage {
		req.Header.Set("X-API-Key", integrationIMessageAPIKey())
	}
	resp := httptest.NewRecorder()
	s.gatewayMux.ServeHTTP(resp, req)
	return resp.Code, nil
}

func integrationIMessageAPIKey() string {
	apiKey := strings.TrimSpace(os.Getenv("IMESSAGE_WEBHOOK_API_KEY"))
	if apiKey == "" {
		return "dev-imessage-key"
	}
	return apiKey
}

func (s *Service) ProcessNextQueuedTurn(ctx context.Context, budgetExhausted bool) (PipelineResult, error) {
	return s.ProcessNextQueuedTurnWithOptions(ctx, ProcessOptions{BudgetExhausted: budgetExhausted})
}

func (s *Service) ProcessNextQueuedTurnWithOptions(ctx context.Context, options ProcessOptions) (PipelineResult, error) {
	msg, ok := s.gateway.PopQueueMessage()
	if !ok {
		return PipelineResult{}, fmt.Errorf("no queued turn")
	}

	envelope, err := gateway.DecodeMessageEnvelope(msg.Payload)
	if err != nil {
		return PipelineResult{}, err
	}
	inbound := WebhookPayload{
		Channel:           strings.ToLower(strings.TrimSpace(msg.Channel)),
		ChannelIdentifier: strings.TrimSpace(msg.ChannelIdentifier),
		UserChannelID:     strings.TrimSpace(msg.UserChannelID),
		Nonce:             envelope.Metadata.ChannelMessageID,
		Message:           envelope.ContentText(),
	}
	if inbound.Channel == "" {
		inbound.Channel = strings.ToLower(strings.TrimSpace(envelope.Channel))
	}
	if inbound.ChannelIdentifier == "" {
		inbound.ChannelIdentifier = inbound.UserChannelID
	}
	if inbound.Message == "" {
		inbound.Message = "[non-text message]"
	}

	firewall := s.control.FirewallCheck(inbound.Message)
	autonomyLevel := strings.ToUpper(strings.TrimSpace(options.AutonomyLevel))
	if autonomyLevel == "" {
		autonomyLevel = "A3"
	}
	toolRiskLevel := strings.ToUpper(strings.TrimSpace(options.ToolRiskLevel))
	if toolRiskLevel == "" {
		toolRiskLevel = s.defaultRisk
	}
	toolKeys := append([]string(nil), options.ToolKeys...)
	if len(toolKeys) == 0 {
		toolKeys = []string{"calendar.create_event"}
	}

	decision := s.control.EvaluateGate(control.DecisionInput{
		AutonomyLevel:          autonomyLevel,
		ToolRiskLevel:          toolRiskLevel,
		IsWrite:                true,
		RateLimited:            options.RateLimited,
		BudgetExhausted:        options.BudgetExhausted,
		FirewallAllowed:        firewall.Allowed,
		SemanticVerifierPassed: true,
		BlockedTool:            false,
	})
	result := PipelineResult{
		GateDecision: decision.Decision,
		ReasonCode:   decision.ReasonCode,
	}

	if decision.Decision == "require_approval" {
		result.ApprovalChallenged = true
		if options.AutoApprove {
			now := options.Now.UTC()
			if now.IsZero() {
				now = time.Now().UTC()
			}
			token, err := s.control.Approval().GenerateToken(
				"tool_commit",
				toolRiskLevel,
				"integration_"+envelope.ID,
				now,
			)
			if err != nil {
				return result, err
			}
			if err := s.control.Approval().ValidateToken(token, now.Add(time.Second)); err != nil {
				return result, err
			}
			result.ApprovalAccepted = true
			result.GateDecision = "allow"
			result.ReasonCode = "APPROVAL_TOKEN_VALID"
		}
	}
	if decision.Decision != "allow" {
		if !result.ApprovalAccepted {
			return result, nil
		}
	}
	if result.GateDecision != "allow" {
		return result, nil
	}

	_ = s.llm.Generate(llm.Request{
		WorkspaceID: msg.WorkspaceID.String(),
		PromptKey:   "brain.planner.v9",
		Input:       inbound.Message,
		Tier:        "T2",
		ModelID:     "model-a",
		ProviderID:  "provider-a",
	})

	workflowResult := s.workflows.InteractiveTurnV1(ctx, inbound.Message)
	result.WorkflowState = workflowResult.FinalState

	workspaceID := msg.WorkspaceID.String()
	now := options.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.mcp.SetNowFunc(func() time.Time { return now })
	for _, toolKey := range toolKeys {
		toolSpec, hasToolSpec := s.mcp.ResolveTool(toolKey)
		if !hasToolSpec {
			toolSpec = mcp.ToolSpec{ToolKey: toolKey, Source: mcp.ToolSourceNative, RiskLevel: "MEDIUM"}
		}
		isMCPInvocation := toolSpec.Source == mcp.ToolSourceMCP
		mcpServerID := ""
		if isMCPInvocation {
			mcpServerID = toolSpec.ServerID
		}
		executionProvider := "native"
		if isMCPInvocation {
			executionProvider = mcpServerID
			if err := s.mcp.EnforceServerPolicy(mcpServerID, 0.01, now); err != nil {
				result.GateDecision = "deny"
				result.ReasonCode = err.Error()
				return result, nil
			}
		}
		if options.PIIContent && isSensitiveFinancialTool(toolKey) {
			executionProvider = "local-model"
		}
		provenance := "native_result"
		if isMCPInvocation {
			provenance = "mcp_result"
		}

		if err := s.control.ConsumeToolCall(workspaceID, toolKey); err != nil {
			result.GateDecision = "deny"
			result.ReasonCode = "TOOL_RATE_CAP_EXCEEDED"
			return result, nil
		}
		if err := s.control.ConsumeBudget(workspaceID, 1); err != nil {
			result.GateDecision = "deny"
			result.ReasonCode = "BUDGET_CALLS_EXHAUSTED"
			return result, nil
		}
		if _, err := s.executor.Simulate(executor.ExecutionRequest{
			WorkspaceID:       workspaceID,
			ToolKey:           toolKey,
			Action:            inbound.Message,
			Provider:          executionProvider,
			TargetURL:         "https://api.example.com/action",
			IsMCP:             isMCPInvocation,
			MCPServerID:       mcpServerID,
			ContentProvenance: provenance,
			PIIContent:        options.PIIContent,
		}); err != nil {
			return result, err
		}
		result.ToolsSimulated++

		if _, _, err := s.executor.Commit(executor.ExecutionRequest{
			WorkspaceID:       workspaceID,
			ToolKey:           toolKey,
			Action:            inbound.Message,
			Provider:          executionProvider,
			TargetURL:         "https://api.example.com/action",
			IsMCP:             isMCPInvocation,
			MCPServerID:       mcpServerID,
			ContentProvenance: provenance,
			PIIContent:        options.PIIContent,
		}); err != nil {
			return result, err
		}
		if isMCPInvocation {
			if err := s.mcp.RecordInvocation(mcp.Invocation{
				WorkspaceID:       workspaceID,
				ToolKey:           toolKey,
				ServerID:          mcpServerID,
				IsMCP:             true,
				Provider:          executionProvider,
				ContentProvenance: provenance,
				PIIContent:        options.PIIContent,
				CostUSD:           0.01,
				CalledAt:          now,
			}); err != nil {
				return result, err
			}
		}
		result.ToolsCommitted++
	}
	result.Simulated = result.ToolsSimulated > 0
	result.Committed = result.ToolsCommitted > 0

	outboundPayload := []byte(fmt.Sprintf(`{"workspace_id":"%s","channel":"%s","channel_identifier":"%s","body":"ok"}`,
		msg.WorkspaceID.String(),
		inbound.Channel,
		inbound.ChannelIdentifier,
	))
	outboundReq := httptest.NewRequest(http.MethodPost, "/v1/gateway/outbound/send", bytes.NewReader(outboundPayload))
	outboundResp := httptest.NewRecorder()
	s.gatewayMux.ServeHTTP(outboundResp, outboundReq)
	result.OutboundCode = outboundResp.Code

	return result, nil
}

func (s *Service) SetToolRateCap(workspaceID, toolKey string, maxCalls int) error {
	return s.control.SetToolRateCap(workspaceID, toolKey, maxCalls)
}

func (s *Service) SetMonthlyBudgetCap(workspaceID string, maxUnits int) error {
	return s.control.SetMonthlyBudgetCap(workspaceID, maxUnits)
}

func (s *Service) RunProvisioningWorkflow(ctx context.Context, failAt string) workflows.ProvisioningResult {
	return s.workflows.ProvisioningV9(ctx, failAt)
}

func (s *Service) RunOnboardingWorkflow(ctx context.Context, answers map[string]string) workflows.OnboardingResult {
	return s.workflows.OnboardingV1(ctx, answers)
}

func (s *Service) RunDriftWorkflow(ctx context.Context, hasDrift bool) string {
	return s.workflows.DriftWatchdogV1(ctx, hasDrift)
}

func (s *Service) RunDailyCaptureWorkflow(ctx context.Context, trigger string) string {
	return s.workflows.DailyCaptureV1(ctx, trigger)
}

func (s *Service) RunRAGEvalWorkflow(ctx context.Context, faithfulness, relevance float64) string {
	return s.workflows.RagEvalV1(ctx, faithfulness, relevance)
}

func (s *Service) ExecutorAuditEventTypes() []string {
	entries := s.executor.AuditEntries()
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry.EventType)
	}
	return out
}

func (s *Service) GatewayAuditEventTypes() []string {
	return s.gateway.AuditEntries()
}

func (s *Service) ValidateExecutorSSRFBlock(targetURL string) error {
	_, err := s.executor.Simulate(executor.ExecutionRequest{
		WorkspaceID: "ws_security",
		ToolKey:     "web.fetch",
		Action:      "fetch",
		Provider:    "native",
		TargetURL:   targetURL,
	})
	return err
}

func (s *Service) EvaluateCircuitBreakerTransition(workspaceID, provider string, base time.Time) (opened bool, closedAfterCooldown bool) {
	now := base.UTC()
	s.executor.SetNowFunc(func() time.Time { return now })
	for idx := 0; idx < 5; idx++ {
		s.executor.RecordProviderFailure(workspaceID, provider)
	}
	opened = s.executor.CircuitOpen(workspaceID, provider)
	now = now.Add(301 * time.Second)
	closedAfterCooldown = !s.executor.CircuitOpen(workspaceID, provider)
	return opened, closedAfterCooldown
}

func (s *Service) VerifyPIIRotationDualKeyWindow(base time.Time) (beforeRotation bool, duringWindow bool, expiredAfterWindow bool, err error) {
	now := base.UTC()
	s.pii.SetRotationWindow(10 * time.Minute)
	s.pii.SetNowFunc(func() time.Time { return now })

	record, err := s.pii.EncryptField("email", "ceo@example.com")
	if err != nil {
		return false, false, false, err
	}
	if _, err := s.pii.DecryptField(record); err == nil {
		beforeRotation = true
	}
	if err := s.pii.RotateKey("v2", []byte("abcdef0123456789abcdef0123456789")); err != nil {
		return beforeRotation, false, false, err
	}

	now = base.UTC().Add(9 * time.Minute)
	if _, err := s.pii.DecryptField(record); err == nil {
		duringWindow = true
	}

	now = base.UTC().Add(11 * time.Minute)
	if _, err := s.pii.DecryptField(record); err != nil {
		expiredAfterWindow = true
	}
	return beforeRotation, duringWindow, expiredAfterWindow, nil
}

func (s *Service) ExecutorExecutions() []executor.ToolExecution {
	return s.executor.Executions()
}

func (s *Service) MCPToolRegistry() []mcp.ToolSpec {
	return s.mcp.ListTools()
}

func (s *Service) MCPHealthDashboard() []mcp.HealthSnapshot {
	return s.mcp.HealthDashboard()
}

func (s *Service) MCPAuthMatrixCoverage() map[mcp.AuthType]bool {
	return s.mcp.AuthMatrixCoverage()
}

func (s *Service) ConfigureMCPServerPolicy(policy mcp.ServerPolicy) error {
	return s.mcp.ConfigureServerPolicy(policy)
}

func isSensitiveFinancialTool(toolKey string) bool {
	normalized := strings.ToLower(strings.TrimSpace(toolKey))
	if normalized == "" {
		return false
	}
	financialPrefixes := []string{
		"plaid.",
		"stripe.",
		"wise.",
		"quickbooks.",
		"financial.",
	}
	for _, prefix := range financialPrefixes {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}
	return false
}

func signPayload(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
