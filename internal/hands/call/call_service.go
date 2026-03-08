package call

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/determinism"
)

// CallProvider is the interface that both VAPI and Retell clients implement.
type CallProvider interface {
	CreateCall(ctx context.Context, req CreateCallRequest) (*CallResponse, error)
	GetCall(ctx context.Context, callID string) (*CallStatus, error)
	CancelCall(ctx context.Context, callID string) error
	Name() string
}

// CallRequest is the user-facing request to initiate a call.
type CallRequest struct {
	PhoneNumber  string `json:"phoneNumber"`
	CallType     string `json:"callType"` // reservation, appointment, quote, custom
	SystemPrompt string `json:"systemPrompt,omitempty"`
	FirstMessage string `json:"firstMessage,omitempty"`
	MaxDuration  int    `json:"maxDuration,omitempty"`
}

// Call represents a tracked call record.
type Call struct {
	ID             uuid.UUID  `json:"id"`
	WorkspaceID    string     `json:"workspaceID"`
	ProviderCallID string     `json:"providerCallID"`
	Provider       string     `json:"provider"`
	PhoneNumber    string     `json:"phoneNumber"`
	CallType       string     `json:"callType"`
	Status         string     `json:"status"`
	Transcript     string     `json:"transcript,omitempty"`
	StartedAt      time.Time  `json:"startedAt"`
	EndedAt        *time.Time `json:"endedAt,omitempty"`
	Duration       int        `json:"duration,omitempty"`
	ApprovalID     *uuid.UUID `json:"approvalID,omitempty"`
}

// ApprovalRequest represents a pending approval for a call.
type ApprovalRequest struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID string    `json:"workspaceID"`
	CallRequest CallRequest `json:"callRequest"`
	Status      string    `json:"status"` // pending, approved, rejected
	CreatedAt   time.Time `json:"createdAt"`
}

// ProviderHealthCheck tracks error rates for provider failover.
type ProviderHealthCheck struct {
	mu          sync.Mutex
	totalCalls  int
	failedCalls int
	windowStart time.Time
}

// ErrorRate returns the current error rate as a percentage.
func (ph *ProviderHealthCheck) ErrorRate() float64 {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	// Reset window every 5 minutes.
	if time.Since(ph.windowStart) > 5*time.Minute {
		ph.totalCalls = 0
		ph.failedCalls = 0
		ph.windowStart = time.Now()
	}

	if ph.totalCalls == 0 {
		return 0
	}
	return float64(ph.failedCalls) / float64(ph.totalCalls) * 100
}

// RecordSuccess records a successful call attempt.
func (ph *ProviderHealthCheck) RecordSuccess() {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.totalCalls++
}

// RecordFailure records a failed call attempt.
func (ph *ProviderHealthCheck) RecordFailure() {
	ph.mu.Lock()
	defer ph.mu.Unlock()
	ph.totalCalls++
	ph.failedCalls++
}

// CallService orchestrates calls using primary and fallback providers.
type CallService struct {
	mu            sync.RWMutex
	primary       CallProvider
	fallback      CallProvider
	promptBuilder *PromptBuilder
	healthCheck   *ProviderHealthCheck
	calls         map[uuid.UUID]*Call
	approvals     map[uuid.UUID]*ApprovalRequest
	callsByProvider map[string]uuid.UUID // providerCallID -> our Call ID
}

// NewCallService creates a new CallService with primary and fallback providers.
func NewCallService(primary, fallback CallProvider) *CallService {
	return &CallService{
		primary:       primary,
		fallback:      fallback,
		promptBuilder: NewPromptBuilder(),
		healthCheck:   &ProviderHealthCheck{windowStart: time.Now()},
		calls:         make(map[uuid.UUID]*Call),
		approvals:     make(map[uuid.UUID]*ApprovalRequest),
		callsByProvider: make(map[string]uuid.UUID),
	}
}

// RequireApproval generates an approval request that must be approved before a call.
func (cs *CallService) RequireApproval(workspaceID string, req CallRequest) (*ApprovalRequest, error) {
	id, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate approval id: %w", err)
	}

	approval := &ApprovalRequest{
		ID:          id,
		WorkspaceID: workspaceID,
		CallRequest: req,
		Status:      "pending",
		CreatedAt:   time.Now(),
	}

	cs.mu.Lock()
	cs.approvals[id] = approval
	cs.mu.Unlock()

	return approval, nil
}

// ApproveCall approves a pending call request.
func (cs *CallService) ApproveCall(approvalID string) error {
	id, err := uuid.Parse(approvalID)
	if err != nil {
		return fmt.Errorf("invalid approval id: %w", err)
	}

	cs.mu.Lock()
	approval, ok := cs.approvals[id]
	if !ok {
		cs.mu.Unlock()
		return fmt.Errorf("approval %s not found", approvalID)
	}
	if approval.Status != "pending" {
		cs.mu.Unlock()
		return fmt.Errorf("approval %s is already %s", approvalID, approval.Status)
	}
	approval.Status = "approved"
	cs.mu.Unlock()

	return nil
}

// InitiateCall checks approval, builds prompt, and initiates a call through the provider.
func (cs *CallService) InitiateCall(ctx context.Context, workspaceID string, req CallRequest) (*Call, error) {
	// Build the system prompt if not provided.
	systemPrompt := req.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = cs.promptBuilder.BuildPrompt(req.CallType, nil)
	}

	firstMessage := req.FirstMessage
	if firstMessage == "" {
		firstMessage = "Hello, I'm calling on behalf of my client."
	}

	maxDuration := req.MaxDuration
	if maxDuration <= 0 {
		maxDuration = 300 // 5 minutes default
	}

	createReq := CreateCallRequest{
		PhoneNumber:        req.PhoneNumber,
		AssistantPrompt:    systemPrompt,
		FirstMessage:       firstMessage,
		MaxDurationSeconds: maxDuration,
		Metadata: map[string]any{
			"workspace_id": workspaceID,
			"call_type":    req.CallType,
		},
	}

	// Select provider based on health check.
	provider := cs.selectProvider()

	resp, err := provider.CreateCall(ctx, createReq)
	if err != nil {
		cs.healthCheck.RecordFailure()

		// Try fallback if primary failed.
		if provider.Name() == cs.primary.Name() && cs.fallback != nil {
			provider = cs.fallback
			resp, err = provider.CreateCall(ctx, createReq)
			if err != nil {
				return nil, fmt.Errorf("both providers failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("call failed: %w", err)
		}
	}
	cs.healthCheck.RecordSuccess()

	callID, err := determinism.NewUUIDv7()
	if err != nil {
		return nil, fmt.Errorf("generate call id: %w", err)
	}

	call := &Call{
		ID:             callID,
		WorkspaceID:    workspaceID,
		ProviderCallID: resp.CallID,
		Provider:       provider.Name(),
		PhoneNumber:    req.PhoneNumber,
		CallType:       req.CallType,
		Status:         resp.Status,
		StartedAt:      time.Now(),
	}

	cs.mu.Lock()
	cs.calls[callID] = call
	cs.callsByProvider[resp.CallID] = callID
	cs.mu.Unlock()

	return call, nil
}

// selectProvider returns the primary provider unless its error rate exceeds the threshold.
func (cs *CallService) selectProvider() CallProvider {
	if cs.healthCheck.ErrorRate() > 5.0 && cs.fallback != nil {
		return cs.fallback
	}
	return cs.primary
}

// HandleCallCompleted processes a completed call event.
func (cs *CallService) HandleCallCompleted(providerCallID string, transcript string) error {
	cs.mu.Lock()
	callID, ok := cs.callsByProvider[providerCallID]
	if !ok {
		cs.mu.Unlock()
		return fmt.Errorf("call with provider id %s not found", providerCallID)
	}
	call, ok := cs.calls[callID]
	if !ok {
		cs.mu.Unlock()
		return fmt.Errorf("call %s not found", callID)
	}
	now := time.Now()
	call.Status = "completed"
	call.Transcript = transcript
	call.EndedAt = &now
	call.Duration = int(now.Sub(call.StartedAt).Seconds())
	cs.mu.Unlock()

	return nil
}

// handleCallStarted updates the call status when it starts.
func (cs *CallService) handleCallStarted(providerCallID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	callID, ok := cs.callsByProvider[providerCallID]
	if !ok {
		return
	}
	if call, ok := cs.calls[callID]; ok {
		call.Status = "in_progress"
	}
}

// GetCall retrieves a call by its internal ID.
func (cs *CallService) GetCall(callID string) (*Call, error) {
	id, err := uuid.Parse(callID)
	if err != nil {
		return nil, fmt.Errorf("invalid call id: %w", err)
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()

	call, ok := cs.calls[id]
	if !ok {
		return nil, fmt.Errorf("call %s not found", callID)
	}
	return call, nil
}

// ListCalls returns all calls for a workspace.
func (cs *CallService) ListCalls(workspaceID string) []Call {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var result []Call
	for _, c := range cs.calls {
		if c.WorkspaceID == workspaceID {
			result = append(result, *c)
		}
	}
	return result
}
