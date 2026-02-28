package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

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
	if result.Committed || result.Simulated {
		t.Fatalf("expected no tool execution when budget exhausted: %+v", result)
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
