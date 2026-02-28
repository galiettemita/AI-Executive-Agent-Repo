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
	GateDecision  string
	WorkflowState string
	Simulated     bool
	Committed     bool
	OutboundCode  int
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
	msg, ok := s.gateway.PopQueueMessage()
	if !ok {
		return PipelineResult{}, fmt.Errorf("no queued turn")
	}

	var inbound WebhookPayload
	if err := json.Unmarshal(msg.Payload, &inbound); err != nil {
		return PipelineResult{}, err
	}

	firewall := s.control.FirewallCheck(inbound.Message)
	decision := s.control.EvaluateGate(control.DecisionInput{
		AutonomyLevel:          "A3",
		ToolRiskLevel:          s.defaultRisk,
		IsWrite:                true,
		RateLimited:            false,
		BudgetExhausted:        budgetExhausted,
		FirewallAllowed:        firewall.Allowed,
		SemanticVerifierPassed: true,
		BlockedTool:            false,
	})
	result := PipelineResult{GateDecision: decision.Decision}
	if decision.Decision != "allow" {
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

	if _, err := s.executor.Simulate(executor.ExecutionRequest{
		WorkspaceID: msg.WorkspaceID.String(),
		ToolKey:     "calendar.create_event",
		Action:      inbound.Message,
		Provider:    "native",
		TargetURL:   "https://api.example.com/action",
	}); err != nil {
		return result, err
	}
	result.Simulated = true

	if _, _, err := s.executor.Commit(executor.ExecutionRequest{
		WorkspaceID: msg.WorkspaceID.String(),
		ToolKey:     "calendar.create_event",
		Action:      inbound.Message,
		Provider:    "native",
		TargetURL:   "https://api.example.com/action",
	}); err != nil {
		return result, err
	}
	result.Committed = true

	outboundReq := httptest.NewRequest(http.MethodPost, "/v1/gateway/outbound/send", bytes.NewReader([]byte(`{"status":"ok"}`)))
	outboundResp := httptest.NewRecorder()
	s.gatewayMux.ServeHTTP(outboundResp, outboundReq)
	result.OutboundCode = outboundResp.Code

	return result, nil
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
