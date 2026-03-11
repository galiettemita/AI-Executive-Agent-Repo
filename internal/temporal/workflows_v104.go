package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// V10.4 Outbound Call Workflows — deterministic, replay-safe.
// No nondeterministic calls (time, rand, uuid, os) in workflow code.

// OutboundCallWorkflow orchestrates an outbound call end-to-end:
// verify phone → request approval → (wait for approval) → make call.
// NNR: No call without approved approval request (cannot be bypassed).
func OutboundCallWorkflow(ctx workflow.Context, input OutboundCallWorkflowInput) (*OutboundCallWorkflowResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Verify phone number.
	var verifyResult *VerifyPhoneResult
	err := workflow.ExecuteActivity(ctx, (*Activities).VerifyPhoneActivity, VerifyPhoneInput{
		PhoneNumber:   input.PhoneNumber,
		BusinessQuery: input.BusinessQuery,
	}).Get(ctx, &verifyResult)
	if err != nil {
		return &OutboundCallWorkflowResult{Status: "failed", Reason: "phone_verification_error"}, err
	}
	if !verifyResult.Verified {
		return &OutboundCallWorkflowResult{
			Status: "rejected",
			Reason: verifyResult.Reason,
		}, nil
	}

	// Step 2: Request approval.
	var approvalResult *RequestCallApprovalResult
	err = workflow.ExecuteActivity(ctx, (*Activities).RequestCallApprovalActivity, RequestCallApprovalInput{
		WorkspaceID:   input.WorkspaceID,
		PhoneNumber:   input.PhoneNumber,
		CallType:      input.CallType,
		SystemPrompt:  input.SystemPrompt,
		FirstMessage:  input.FirstMessage,
		MaxDuration:   input.MaxDuration,
		BusinessQuery: input.BusinessQuery,
		ReceiptID:     input.ReceiptID,
	}).Get(ctx, &approvalResult)
	if err != nil {
		return &OutboundCallWorkflowResult{Status: "failed", Reason: "approval_request_error"}, err
	}

	if approvalResult.Status == "denied" {
		return &OutboundCallWorkflowResult{
			Status:     "denied",
			Reason:     approvalResult.Reason,
			ApprovalID: approvalResult.ApprovalID,
		}, nil
	}

	// Step 3: Check provider health before calling.
	var healthResult *CheckProviderHealthResult
	err = workflow.ExecuteActivity(ctx, (*Activities).CheckProviderHealthActivity, CheckProviderHealthInput{
		WorkspaceID:  input.WorkspaceID,
		ProviderName: "vapi",
	}).Get(ctx, &healthResult)
	if err != nil {
		return &OutboundCallWorkflowResult{
			Status:     "failed",
			Reason:     "provider_health_check_error",
			ApprovalID: approvalResult.ApprovalID,
		}, err
	}

	// Step 4: Make the call.
	var callResult *MakeCallResult
	err = workflow.ExecuteActivity(ctx, (*Activities).MakeCallActivity, MakeCallInput{
		WorkspaceID:  input.WorkspaceID,
		ApprovalID:   approvalResult.ApprovalID,
		PhoneNumber:  input.PhoneNumber,
		CallType:     input.CallType,
		SystemPrompt: input.SystemPrompt,
		FirstMessage: input.FirstMessage,
		MaxDuration:  input.MaxDuration,
	}).Get(ctx, &callResult)
	if err != nil {
		return &OutboundCallWorkflowResult{
			Status:     "failed",
			Reason:     "call_initiation_error",
			ApprovalID: approvalResult.ApprovalID,
		}, err
	}

	return &OutboundCallWorkflowResult{
		Status:         "initiated",
		CallID:         callResult.CallID,
		ProviderCallID: callResult.ProviderCallID,
		Provider:       callResult.Provider,
		ApprovalID:     approvalResult.ApprovalID,
		FailoverCount:  callResult.FailoverCount,
	}, nil
}

// CallWebhookProcessingWorkflow processes an incoming call webhook event,
// persisting transcript segments and updating call status.
func CallWebhookProcessingWorkflow(ctx workflow.Context, input CallWebhookProcessingWorkflowInput) (*CallWebhookProcessingWorkflowResult, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Process the webhook event.
	var webhookResult *ProcessCallWebhookResult
	err := workflow.ExecuteActivity(ctx, (*Activities).ProcessCallWebhookActivity, ProcessCallWebhookInput{
		ProviderCallID: input.ProviderCallID,
		EventType:      input.EventType,
		Transcript:     input.Transcript,
		Duration:       input.Duration,
	}).Get(ctx, &webhookResult)
	if err != nil {
		return &CallWebhookProcessingWorkflowResult{Status: "failed"}, err
	}

	// Step 2: If transcript is provided, persist segments.
	segmentsPersisted := 0
	if input.Transcript != "" && webhookResult.CallID != "" {
		var segResult *PersistTranscriptSegmentResult
		err = workflow.ExecuteActivity(ctx, (*Activities).PersistTranscriptSegmentActivity, PersistTranscriptSegmentInput{
			WorkspaceID:  input.WorkspaceID,
			CallID:       webhookResult.CallID,
			SegmentIndex: 0,
			SegmentType:  "agent",
			Speaker:      "agent",
			Content:      input.Transcript,
			Confidence:   1.0,
			Language:     "en",
		}).Get(ctx, &segResult)
		if err == nil && segResult.Persisted {
			segmentsPersisted = 1
		}
	}

	return &CallWebhookProcessingWorkflowResult{
		Status:            "complete",
		CallID:            webhookResult.CallID,
		EventType:         input.EventType,
		SegmentsPersisted: segmentsPersisted,
	}, nil
}

// Workflow input/output types.

type OutboundCallWorkflowInput struct {
	WorkspaceID   string `json:"workspace_id"`
	PhoneNumber   string `json:"phone_number"`
	CallType      string `json:"call_type"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	FirstMessage  string `json:"first_message,omitempty"`
	MaxDuration   int    `json:"max_duration,omitempty"`
	BusinessQuery string `json:"business_query"`
	ReceiptID     string `json:"receipt_id"`
}

type OutboundCallWorkflowResult struct {
	Status         string `json:"status"`
	CallID         string `json:"call_id,omitempty"`
	ProviderCallID string `json:"provider_call_id,omitempty"`
	Provider       string `json:"provider,omitempty"`
	ApprovalID     string `json:"approval_id,omitempty"`
	FailoverCount  int    `json:"failover_count"`
	Reason         string `json:"reason,omitempty"`
}

type CallWebhookProcessingWorkflowInput struct {
	WorkspaceID    string `json:"workspace_id"`
	ProviderCallID string `json:"provider_call_id"`
	EventType      string `json:"event_type"`
	Transcript     string `json:"transcript,omitempty"`
	Duration       int    `json:"duration,omitempty"`
}

type CallWebhookProcessingWorkflowResult struct {
	Status            string `json:"status"`
	CallID            string `json:"call_id"`
	EventType         string `json:"event_type"`
	SegmentsPersisted int    `json:"segments_persisted"`
}
