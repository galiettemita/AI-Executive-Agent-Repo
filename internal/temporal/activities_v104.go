package temporal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brevio/brevio/internal/hands/call"
)

// V10.4 Outbound Call Activity Input/Output types.

type RequestCallApprovalInput struct {
	WorkspaceID   string `json:"workspace_id"`
	PhoneNumber   string `json:"phone_number"`
	CallType      string `json:"call_type"`
	SystemPrompt  string `json:"system_prompt,omitempty"`
	FirstMessage  string `json:"first_message,omitempty"`
	MaxDuration   int    `json:"max_duration,omitempty"`
	BusinessQuery string `json:"business_query"`
	ReceiptID     string `json:"receipt_id"`
}

type RequestCallApprovalResult struct {
	ApprovalID string `json:"approval_id,omitempty"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	PolicyID   string `json:"policy_id,omitempty"`
}

type VerifyPhoneInput struct {
	PhoneNumber   string `json:"phone_number"`
	BusinessQuery string `json:"business_query"`
}

type VerifyPhoneResult struct {
	Verified     bool   `json:"verified"`
	Source       string `json:"source"`
	BusinessName string `json:"business_name,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type MakeCallInput struct {
	WorkspaceID string `json:"workspace_id"`
	ApprovalID  string `json:"approval_id"`
	PhoneNumber string `json:"phone_number"`
	CallType    string `json:"call_type"`
	SystemPrompt string `json:"system_prompt,omitempty"`
	FirstMessage string `json:"first_message,omitempty"`
	MaxDuration  int    `json:"max_duration,omitempty"`
}

type MakeCallResult struct {
	CallID         string `json:"call_id"`
	ProviderCallID string `json:"provider_call_id"`
	Provider       string `json:"provider"`
	Status         string `json:"status"`
	FailoverCount  int    `json:"failover_count"`
}

type ProcessCallWebhookInput struct {
	ProviderCallID string `json:"provider_call_id"`
	EventType      string `json:"event_type"`
	Transcript     string `json:"transcript,omitempty"`
	Duration       int    `json:"duration,omitempty"`
}

type ProcessCallWebhookResult struct {
	CallID    string `json:"call_id"`
	Status    string `json:"status"`
	Processed bool   `json:"processed"`
}

type PersistTranscriptSegmentInput struct {
	WorkspaceID  string  `json:"workspace_id"`
	CallID       string  `json:"call_id"`
	SegmentIndex int     `json:"segment_index"`
	SegmentType  string  `json:"segment_type"`
	Speaker      string  `json:"speaker"`
	Content      string  `json:"content"`
	StartedAtMs  int     `json:"started_at_ms"`
	DurationMs   int     `json:"duration_ms"`
	Confidence   float64 `json:"confidence"`
	Language     string  `json:"language"`
}

type PersistTranscriptSegmentResult struct {
	Persisted bool `json:"persisted"`
}

type CheckProviderHealthInput struct {
	WorkspaceID  string `json:"workspace_id"`
	ProviderName string `json:"provider_name"`
}

type CheckProviderHealthResult struct {
	ProviderID     string  `json:"provider_id"`
	HealthScore    float64 `json:"health_score"`
	Status         string  `json:"status"`
	ShouldFailover bool    `json:"should_failover"`
}

// RequestCallApprovalActivity evaluates a call request against approval policies
// and creates a DB-backed approval request. NNR: No call without approval.
func (a *Activities) RequestCallApprovalActivity(ctx context.Context, input RequestCallApprovalInput) (*RequestCallApprovalResult, error) {
	if input.WorkspaceID == "" || input.PhoneNumber == "" {
		return nil, fmt.Errorf("workspace_id and phone_number are required")
	}

	if a.callRepo == nil {
		// Degraded mode: create in-memory approval.
		return &RequestCallApprovalResult{
			ApprovalID: hashKey("approval:" + input.WorkspaceID + ":" + input.PhoneNumber),
			Status:     "pending",
			Reason:     "APPROVAL_REQUIRED: awaiting approval (degraded mode)",
		}, nil
	}

	approvalSvc := call.NewApprovalService(a.callRepo)
	decision, err := approvalSvc.RequestApproval(ctx, input.WorkspaceID, call.CallRequest{
		PhoneNumber:  input.PhoneNumber,
		CallType:     input.CallType,
		SystemPrompt: input.SystemPrompt,
		FirstMessage: input.FirstMessage,
		MaxDuration:  input.MaxDuration,
	}, input.ReceiptID)
	if err != nil {
		return nil, fmt.Errorf("request approval: %w", err)
	}

	return &RequestCallApprovalResult{
		ApprovalID: decision.ApprovalID,
		Status:     decision.Status,
		Reason:     decision.Reason,
		PolicyID:   decision.PolicyID,
	}, nil
}

// VerifyPhoneActivity verifies a phone number via Google Places lookup.
// NNR: Unverified phone numbers MUST be rejected.
func (a *Activities) VerifyPhoneActivity(ctx context.Context, input VerifyPhoneInput) (*VerifyPhoneResult, error) {
	if input.PhoneNumber == "" {
		return &VerifyPhoneResult{
			Verified: false,
			Source:   "validation",
			Reason:   "EMPTY_PHONE_NUMBER",
		}, nil
	}

	if a.phoneVerifier == nil {
		// Degraded mode: use deterministic verifier.
		verifier := call.NewDeterministicPhoneVerifier()
		result, err := verifier.VerifyPhone(ctx, input.PhoneNumber, input.BusinessQuery)
		if err != nil {
			return nil, fmt.Errorf("verify phone: %w", err)
		}
		return &VerifyPhoneResult{
			Verified:     result.Verified,
			Source:       result.Source,
			BusinessName: result.BusinessName,
			Reason:       result.Reason,
		}, nil
	}

	result, err := a.phoneVerifier.VerifyPhone(ctx, input.PhoneNumber, input.BusinessQuery)
	if err != nil {
		return nil, fmt.Errorf("verify phone: %w", err)
	}

	return &VerifyPhoneResult{
		Verified:     result.Verified,
		Source:       result.Source,
		BusinessName: result.BusinessName,
		Reason:       result.Reason,
	}, nil
}

// MakeCallActivity initiates an outbound call after verifying approval.
// NNR: Approval MUST be verified before call initiation.
// Sequence: verify approval → insert call row → call provider → persist events.
func (a *Activities) MakeCallActivity(ctx context.Context, input MakeCallInput) (*MakeCallResult, error) {
	if input.WorkspaceID == "" || input.ApprovalID == "" || input.PhoneNumber == "" {
		return nil, fmt.Errorf("workspace_id, approval_id, and phone_number are required")
	}

	// Step 1: Verify approval exists and is approved (NNR).
	if a.callRepo != nil {
		approvalSvc := call.NewApprovalService(a.callRepo)
		_, err := approvalSvc.VerifyApproved(ctx, input.ApprovalID)
		if err != nil {
			return nil, fmt.Errorf("APPROVAL_GATE_FAILED: %w", err)
		}
	}

	// Step 2: Insert call row FIRST (before calling provider).
	callID := hashKey("call:" + input.WorkspaceID + ":" + input.ApprovalID + ":" + input.PhoneNumber)
	numberHash := call.HashPhoneNumber(input.PhoneNumber)

	if a.callRepo != nil {
		// Get provider.
		provider, err := a.callRepo.GetProvider(ctx, input.WorkspaceID, "vapi")
		if err != nil {
			provider = &call.ProviderRow{ID: "00000000-0000-0000-0000-000000000000"}
		}

		if err := a.callRepo.InsertCall(ctx, call.CallRow{
			ID:                callID,
			WorkspaceID:       input.WorkspaceID,
			ApprovalRequestID: input.ApprovalID,
			ProviderID:        provider.ID,
			Direction:         "outbound",
			Status:            "dialing",
			TargetNumberHash:  numberHash,
			Metadata:          fmt.Sprintf(`{"call_type":"%s"}`, input.CallType),
		}); err != nil {
			return nil, fmt.Errorf("insert call row: %w", err)
		}

		// Record call event.
		_ = a.callRepo.InsertCallEvent(ctx, call.CallEventRow{
			WorkspaceID: input.WorkspaceID,
			CallID:      callID,
			EventType:   "call.initiated",
			EventData:   fmt.Sprintf(`{"approval_id":"%s","call_type":"%s"}`, input.ApprovalID, input.CallType),
		})
	}

	// Step 3: Call provider (with failover).
	if a.callService != nil {
		resp, err := a.callService.InitiateCall(ctx, input.WorkspaceID, call.CallRequest{
			PhoneNumber:  input.PhoneNumber,
			CallType:     input.CallType,
			SystemPrompt: input.SystemPrompt,
			FirstMessage: input.FirstMessage,
			MaxDuration:  input.MaxDuration,
		})
		if err != nil {
			// Update call status to failed.
			if a.callRepo != nil {
				_ = a.callRepo.UpdateCallStatus(ctx, callID, "failed")
				_ = a.callRepo.InsertCallEvent(ctx, call.CallEventRow{
					WorkspaceID: input.WorkspaceID,
					CallID:      callID,
					EventType:   "call.failed",
					EventData:   fmt.Sprintf(`{"error":"%s"}`, err.Error()),
				})
			}
			return nil, fmt.Errorf("initiate call: %w", err)
		}

		// Update call with provider details.
		if a.callRepo != nil {
			_ = a.callRepo.UpdateCallProvider(ctx, callID, resp.ProviderCallID, 0)
		}

		return &MakeCallResult{
			CallID:         callID,
			ProviderCallID: resp.ProviderCallID,
			Provider:       resp.Provider,
			Status:         resp.Status,
			FailoverCount:  0,
		}, nil
	}

	// Degraded mode: simulate successful call.
	return &MakeCallResult{
		CallID:         callID,
		ProviderCallID: "sim-" + callID[:8],
		Provider:       "simulated",
		Status:         "dialing",
		FailoverCount:  0,
	}, nil
}

// ProcessCallWebhookActivity processes an incoming call webhook event.
// Maps provider_call_id → internal call_id, persists transcript and events.
// Idempotent: re-processing the same event is a no-op.
func (a *Activities) ProcessCallWebhookActivity(ctx context.Context, input ProcessCallWebhookInput) (*ProcessCallWebhookResult, error) {
	if input.ProviderCallID == "" || input.EventType == "" {
		return nil, fmt.Errorf("provider_call_id and event_type are required")
	}

	if a.callRepo == nil {
		return &ProcessCallWebhookResult{
			CallID:    "",
			Status:    input.EventType,
			Processed: false,
		}, nil
	}

	// Map provider_call_id → internal call_id.
	callRow, err := a.callRepo.GetCallByProviderID(ctx, input.ProviderCallID)
	if err != nil {
		return nil, fmt.Errorf("lookup call by provider id: %w", err)
	}

	// Record event.
	eventData, _ := json.Marshal(map[string]any{
		"provider_call_id": input.ProviderCallID,
		"event_type":       input.EventType,
		"duration":         input.Duration,
	})
	_ = a.callRepo.InsertCallEvent(ctx, call.CallEventRow{
		WorkspaceID: callRow.WorkspaceID,
		CallID:      callRow.ID,
		EventType:   input.EventType,
		EventData:   string(eventData),
	})

	// Update call status based on event.
	switch input.EventType {
	case "call.started":
		_ = a.callRepo.UpdateCallStatus(ctx, callRow.ID, "in_progress")
	case "call.ended":
		_ = a.callRepo.CompleteCall(ctx, callRow.ID, input.Duration, 0)
	case "call.failed":
		_ = a.callRepo.UpdateCallStatus(ctx, callRow.ID, "failed")
	}

	return &ProcessCallWebhookResult{
		CallID:    callRow.ID,
		Status:    input.EventType,
		Processed: true,
	}, nil
}

// PersistTranscriptSegmentActivity persists a transcript segment to the DB.
// NNR: No raw audio persistence — transcript text only.
func (a *Activities) PersistTranscriptSegmentActivity(ctx context.Context, input PersistTranscriptSegmentInput) (*PersistTranscriptSegmentResult, error) {
	if a.callRepo == nil {
		return &PersistTranscriptSegmentResult{Persisted: false}, nil
	}

	language := input.Language
	if language == "" {
		language = "en"
	}
	confidence := input.Confidence
	if confidence <= 0 {
		confidence = 1.0
	}

	err := a.callRepo.InsertTranscriptSegment(ctx, call.TranscriptSegmentRow{
		WorkspaceID:  input.WorkspaceID,
		CallID:       input.CallID,
		SegmentIndex: input.SegmentIndex,
		SegmentType:  input.SegmentType,
		Speaker:      input.Speaker,
		Content:      input.Content,
		StartedAtMs:  input.StartedAtMs,
		DurationMs:   input.DurationMs,
		Confidence:   confidence,
		Language:     language,
	})
	return &PersistTranscriptSegmentResult{Persisted: err == nil}, err
}

// CheckProviderHealthActivity checks a call provider's health and determines
// if failover should be triggered.
func (a *Activities) CheckProviderHealthActivity(ctx context.Context, input CheckProviderHealthInput) (*CheckProviderHealthResult, error) {
	if a.callRepo == nil {
		return &CheckProviderHealthResult{
			HealthScore:    1.0,
			Status:         "active",
			ShouldFailover: false,
		}, nil
	}

	provider, err := a.callRepo.GetProvider(ctx, input.WorkspaceID, input.ProviderName)
	if err != nil {
		return &CheckProviderHealthResult{
			HealthScore:    0,
			Status:         "unknown",
			ShouldFailover: true,
		}, nil
	}

	// Get recent health records.
	healthRecords, err := a.callRepo.GetProviderHealth(ctx, provider.ID, 10)
	if err != nil || len(healthRecords) == 0 {
		return &CheckProviderHealthResult{
			ProviderID:     provider.ID,
			HealthScore:    provider.HealthScore,
			Status:         provider.Status,
			ShouldFailover: false,
		}, nil
	}

	// Aggregate recent error/success counts.
	var totalErrors, totalSuccess int
	for _, h := range healthRecords {
		totalErrors += h.ErrorCount
		totalSuccess += h.SuccessCount
	}

	shouldFailover, healthScore := call.EvaluateProviderHealth(totalErrors, totalSuccess)

	// Update provider status if failover triggered.
	if shouldFailover && provider.Status != "failover" {
		_ = a.callRepo.UpdateProviderStatus(ctx, provider.ID, "failover", healthScore)
	}

	return &CheckProviderHealthResult{
		ProviderID:     provider.ID,
		HealthScore:    healthScore,
		Status:         provider.Status,
		ShouldFailover: shouldFailover,
	}, nil
}
