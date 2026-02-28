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
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/executor"
	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/llm"
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
		defaultRisk: "LOW",
	}
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
	req := httptest.NewRequest(http.MethodPost, "/v1/gateway/webhook/whatsapp", bytes.NewReader(blob))
	req.Header.Set("X-Signature", signPayload([]byte(s.secret), blob))
	resp := httptest.NewRecorder()
	s.gatewayMux.ServeHTTP(resp, req)
	return resp.Code, nil
}

func (s *Service) ProcessNextQueuedTurn(ctx context.Context, budgetExhausted bool) (PipelineResult, error) {
	return s.ProcessNextQueuedTurnWithOptions(ctx, ProcessOptions{BudgetExhausted: budgetExhausted})
}

func (s *Service) ProcessNextQueuedTurnWithOptions(ctx context.Context, options ProcessOptions) (PipelineResult, error) {
	msg, ok := s.gateway.PopQueueMessage()
	if !ok {
		return PipelineResult{}, fmt.Errorf("no queued turn")
	}

	var inbound WebhookPayload
	if err := json.Unmarshal(msg.Payload, &inbound); err != nil {
		return PipelineResult{}, err
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
				"integration_"+inbound.Nonce,
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
	for _, toolKey := range toolKeys {
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
			WorkspaceID: workspaceID,
			ToolKey:     toolKey,
			Action:      inbound.Message,
			Provider:    "native",
			TargetURL:   "https://api.example.com/action",
		}); err != nil {
			return result, err
		}
		result.ToolsSimulated++

		if _, _, err := s.executor.Commit(executor.ExecutionRequest{
			WorkspaceID: workspaceID,
			ToolKey:     toolKey,
			Action:      inbound.Message,
			Provider:    "native",
			TargetURL:   "https://api.example.com/action",
		}); err != nil {
			return result, err
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

func signPayload(secret, payload []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
