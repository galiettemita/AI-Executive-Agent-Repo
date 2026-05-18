package contract

import (
	"context"
	"testing"

	temporal "github.com/brevio/brevio/internal/temporal"
)

func TestMessageProcessingWorkflowInput_Fields(t *testing.T) {
	input := temporal.MessageProcessingWorkflowInput{
		MessageID:      "msg-1",
		WorkspaceID:    "ws-1",
		ChannelType:    "slack",
		RawPayload:     `{"text":"hello"}`,
		IdempotencyKey: "idem-1",
	}
	if input.MessageID != "msg-1" {
		t.Error("MessageID mismatch")
	}
	if input.WorkspaceID != "ws-1" {
		t.Error("WorkspaceID mismatch")
	}
	if input.IdempotencyKey != "idem-1" {
		t.Error("IdempotencyKey mismatch")
	}
}

func TestActivityTypes_ReceiptRequired(t *testing.T) {
	// Verify that ExecuteToolInput requires a receipt ID
	input := temporal.ExecuteToolInput{
		MessageID:      "msg-1",
		WorkspaceID:    "ws-1",
		ToolKey:        "tool-a",
		ReceiptID:      "",
		IdempotencyKey: "idem-1",
	}
	a := temporal.NewActivities()
	_, err := a.ExecuteToolActivity(context.Background(), input)
	if err == nil {
		t.Fatal("expected error when receipt is empty")
	}
}

func TestValidateEnvelopeActivity_Contract(t *testing.T) {
	a := temporal.NewActivities()
	tests := []struct {
		name  string
		input temporal.ValidateEnvelopeInput
		valid bool
	}{
		{
			"valid_input",
			temporal.ValidateEnvelopeInput{MessageID: "m1", WorkspaceID: "ws1", ChannelType: "slack", RawPayload: "hello"},
			true,
		},
		{
			"missing_message_id",
			temporal.ValidateEnvelopeInput{WorkspaceID: "ws1", RawPayload: "hello"},
			false,
		},
		{
			"empty_payload",
			temporal.ValidateEnvelopeInput{MessageID: "m1", WorkspaceID: "ws1", RawPayload: ""},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := a.ValidateEnvelopeActivity(context.Background(), tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Valid != tt.valid {
				t.Errorf("expected valid=%v, got %v (reason: %s)", tt.valid, result.Valid, result.Reason)
			}
		})
	}
}
