package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/workflows"
)

func TestFailureDomainChannelIngressRetriesDeadLetterAfterThreeFailures(t *testing.T) {
	t.Parallel()

	result, err := ProcessChannelIngressWithRetry(3, func(_ int) error {
		return errors.New("provider timeout")
	})
	if err != nil {
		t.Fatalf("process ingress retry: %v", err)
	}
	if !result.DeadLettered || result.Attempts != 3 {
		t.Fatalf("unexpected ingress retry result: %+v", result)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(result.BackoffDelays) != len(want) {
		t.Fatalf("unexpected backoff delays: %v", result.BackoffDelays)
	}
	for i := range want {
		if result.BackoffDelays[i] != want[i] {
			t.Fatalf("unexpected backoff delay at %d: got=%s want=%s", i, result.BackoffDelays[i], want[i])
		}
	}
}

func TestFailureDomainGatewayProcessingInvalidFormatRejected(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	status, err := svc.IngestWebhookRaw("whatsapp", []byte(`{"channel":"whatsapp"`), "")
	if err != nil {
		t.Fatalf("ingest raw invalid payload: %v", err)
	}
	if status != 400 {
		t.Fatalf("expected 400 for invalid webhook payload, got %d", status)
	}
}

func TestFailureDomainGatewayProcessingTranscriptionFallbackToText(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d6f")
	svc.BindWorkspace("whatsapp", "+15550009998", workspaceID)

	status, err := svc.IngestWebhookRaw("whatsapp", []byte(`{
		"channel":"whatsapp",
		"channel_identifier":"+15550009998",
		"user_channel_id":"u-domain",
		"nonce":"domain_nonce_1",
		"message":"fallback to text",
		"audio_url":"http://insecure.example.com/audio.ogg"
	}`), "")
	if err != nil {
		t.Fatalf("ingest raw webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("expected accepted webhook, got %d", status)
	}

	msg, ok := svc.gateway.PopQueueMessage()
	if !ok {
		t.Fatal("expected queue message")
	}
	envelope, err := gateway.DecodeMessageEnvelope(msg.Payload)
	if err != nil {
		t.Fatalf("decode message envelope: %v", err)
	}
	if envelope.Content.Type != "TEXT" {
		t.Fatalf("expected TEXT fallback content type, got %s", envelope.Content.Type)
	}
	if envelope.Content.Text != "fallback to text" {
		t.Fatalf("unexpected fallback text content: %s", envelope.Content.Text)
	}
}

func TestFailureDomainBrainClassificationFallback(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	result := svc.MessageProcessingWorkflowV1(workflows.MessageProcessingInput{
		MessageID:                "msg-domain-1",
		WorkflowRunID:            "run-domain-1",
		EnvelopeValid:            true,
		ClassifyError:            true,
		KeywordFallbackAvailable: true,
		DAGValid:                 true,
	})
	if result.TerminalState != workflows.MessageStateCompleted {
		t.Fatalf("expected completed terminal state, got %s", result.TerminalState)
	}
	if !containsStringDomain(result.Fallbacks, "keyword_classifier") {
		t.Fatalf("expected keyword classifier fallback, got %v", result.Fallbacks)
	}
}

func TestFailureDomainBrainDecompositionFallback(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	result := svc.MessageProcessingWorkflowV1(workflows.MessageProcessingInput{
		MessageID:                "msg-domain-2",
		WorkflowRunID:            "run-domain-2",
		EnvelopeValid:            true,
		ClassifyConfidence:       0.9,
		KeywordFallbackAvailable: true,
		DAGValid:                 false,
	})
	if result.TerminalState != workflows.MessageStateCompleted {
		t.Fatalf("expected completed terminal state, got %s", result.TerminalState)
	}
	if !containsStringDomain(result.Fallbacks, "single_skill_fallback") {
		t.Fatalf("expected decomposition fallback, got %v", result.Fallbacks)
	}
}

func TestFailureDomainHandsExecutionCompensation(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	result := svc.MessageProcessingWorkflowV1(workflows.MessageProcessingInput{
		MessageID:                "msg-domain-3",
		WorkflowRunID:            "run-domain-3",
		EnvelopeValid:            true,
		ClassifyConfidence:       0.9,
		KeywordFallbackAvailable: true,
		DAGValid:                 true,
		ExecuteError:             true,
		RequiresCompensation:     true,
	})
	if result.TerminalState != workflows.MessageStateFailed {
		t.Fatalf("expected failed terminal state, got %s", result.TerminalState)
	}
	if !result.CompensationNeeded {
		t.Fatalf("expected compensation needed, got %+v", result)
	}
}

func TestFailureDomainResultAggregationReturnsPartialResults(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	result := svc.MessageProcessingWorkflowV1(workflows.MessageProcessingInput{
		MessageID:                "msg-domain-4",
		WorkflowRunID:            "run-domain-4",
		EnvelopeValid:            true,
		ClassifyConfidence:       0.9,
		KeywordFallbackAvailable: true,
		DAGValid:                 true,
		AggregateError:           true,
		AllowPartialAggregation:  true,
	})
	if result.TerminalState != workflows.MessageStateCompleted {
		t.Fatalf("expected completed terminal state with partial aggregation, got %s", result.TerminalState)
	}
	if !containsStringDomain(result.Fallbacks, "partial_results") {
		t.Fatalf("expected partial results fallback, got %v", result.Fallbacks)
	}
}

func TestFailureDomainChannelEgressDeliveryFailureToDLQ(t *testing.T) {
	t.Parallel()

	svc := workflows.NewService()
	result := svc.MessageProcessingWorkflowV1(workflows.MessageProcessingInput{
		MessageID:                    "msg-domain-5",
		WorkflowRunID:                "run-domain-5",
		EnvelopeValid:                true,
		ClassifyConfidence:           0.9,
		KeywordFallbackAvailable:     true,
		DAGValid:                     true,
		DeliveryFailures:             3,
		DeliveryFallbackQueueAllowed: true,
	})
	if result.TerminalState != workflows.MessageStateDeadLetter {
		t.Fatalf("expected dead-letter terminal state, got %s", result.TerminalState)
	}
	if !containsStringDomain(result.Fallbacks, "delivery_dlq") {
		t.Fatalf("expected delivery_dlq fallback, got %v", result.Fallbacks)
	}
}

func containsStringDomain(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func TestFailureDomainIngressRetryRecoversBeforeDLQ(t *testing.T) {
	t.Parallel()

	calls := 0
	result, err := ProcessChannelIngressWithRetry(3, func(_ int) error {
		calls++
		if calls == 2 {
			return nil
		}
		return errors.New("transient")
	})
	if err != nil {
		t.Fatalf("process ingress retry: %v", err)
	}
	if result.DeadLettered || result.Attempts != 2 {
		t.Fatalf("expected recovery on second attempt, got %+v", result)
	}
}

func TestFailureDomainMatrixCanRunThroughIntegrationPipeline(t *testing.T) {
	t.Parallel()

	svc := NewService("integration-secret")
	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d7f")
	svc.BindWorkspace("whatsapp", "+15550009997", workspaceID)

	status, err := svc.IngestWebhook(WebhookPayload{
		Channel:           "whatsapp",
		ChannelIdentifier: "+15550009997",
		UserChannelID:     "u-domain-matrix",
		Nonce:             "domain_matrix_nonce",
		Message:           "test domain matrix",
	})
	if err != nil {
		t.Fatalf("ingest webhook: %v", err)
	}
	if status != 202 {
		t.Fatalf("unexpected ingest status: %d", status)
	}

	result, err := svc.ProcessNextQueuedTurn(context.Background(), false)
	if err != nil {
		t.Fatalf("process queued turn: %v", err)
	}
	if result.GateDecision != "allow" || !result.Committed {
		t.Fatalf("expected successful baseline integration processing, got %+v", result)
	}
}
