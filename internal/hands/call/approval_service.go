package call

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// FailoverThreshold is the error rate percentage above which provider failover triggers.
// Hard-coded per NNR rules — cannot be overridden at runtime.
const FailoverThreshold = 5.0

// ApprovalExpiryDuration is the default time after which a pending approval expires.
const ApprovalExpiryDuration = 24 * time.Hour

// ApprovalService manages the call approval lifecycle with DB persistence.
// NNR: No call without approved approval request (cannot be bypassed).
type ApprovalService struct {
	repo CallRepository
}

// NewApprovalService creates a new ApprovalService backed by a CallRepository.
func NewApprovalService(repo CallRepository) *ApprovalService {
	return &ApprovalService{repo: repo}
}

// ApprovalDecision represents the result of evaluating a call request against policies.
type ApprovalDecision struct {
	Status         string `json:"status"` // auto_approved, pending, denied
	ApprovalID     string `json:"approval_id,omitempty"`
	Reason         string `json:"reason"`
	PolicyID       string `json:"policy_id"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
}

// RequestApproval evaluates a call request against the workspace's active policy
// and creates an approval request in the DB. Returns the decision.
func (s *ApprovalService) RequestApproval(ctx context.Context, workspaceID string, req CallRequest, receiptID string) (*ApprovalDecision, error) {
	// 1. Verify phone number is not blocked.
	numberHash := hashNumber(req.PhoneNumber)
	blocked, err := s.repo.IsNumberBlocked(ctx, workspaceID, numberHash)
	if err != nil {
		return nil, fmt.Errorf("check blocklist: %w", err)
	}
	if blocked {
		return &ApprovalDecision{
			Status: "denied",
			Reason: "BLOCKED_NUMBER: target number is on the blocklist",
		}, nil
	}

	// 2. Get active approval policy.
	policy, err := s.repo.GetActivePolicy(ctx, workspaceID)
	if err != nil {
		// No policy = deny by default (NNR: no call without approval).
		return &ApprovalDecision{
			Status: "denied",
			Reason: "NO_ACTIVE_POLICY: no call approval policy configured",
		}, nil
	}

	// 3. Check if number is in policy-level blocklist.
	for _, bn := range policy.BlockedNumbers {
		if hashNumber(bn) == numberHash {
			return &ApprovalDecision{
				Status:   "denied",
				Reason:   "POLICY_BLOCKED_NUMBER: number blocked by policy",
				PolicyID: policy.ID,
			}, nil
		}
	}

	// 4. Check rate limits.
	now := time.Now().UTC()
	windowStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(24 * time.Hour)
	count, err := s.repo.IncrementRateLimit(ctx, workspaceID, windowStart, windowEnd, policy.MaxDailyCalls)
	if err != nil {
		return nil, fmt.Errorf("check rate limit: %w", err)
	}
	if count > policy.MaxDailyCalls {
		return &ApprovalDecision{
			Status:   "denied",
			Reason:   fmt.Sprintf("RATE_LIMIT_EXCEEDED: %d/%d daily calls used", count, policy.MaxDailyCalls),
			PolicyID: policy.ID,
		}, nil
	}

	// 5. Create the approval request row.
	expiresAt := now.Add(ApprovalExpiryDuration)
	callerContext := fmt.Sprintf(`{"call_type":"%s","max_duration":%d}`, req.CallType, req.MaxDuration)

	approvalRow := ApprovalRequestRow{
		WorkspaceID:      workspaceID,
		ReceiptID:        receiptID,
		PolicyID:         policy.ID,
		CallerContext:    callerContext,
		TargetNumberHash: numberHash,
		Purpose:          req.CallType,
		Status:           "pending",
		ExpiresAt:        expiresAt,
	}

	if err := s.repo.CreateApprovalRequest(ctx, approvalRow); err != nil {
		return nil, fmt.Errorf("create approval request: %w", err)
	}

	// 6. Get the created request (has DB-assigned ID).
	pending, err := s.repo.GetPendingApprovals(ctx, workspaceID, 1)
	if err != nil || len(pending) == 0 {
		return nil, fmt.Errorf("retrieve created approval: %w", err)
	}
	created := pending[len(pending)-1]

	return &ApprovalDecision{
		Status:     "pending",
		ApprovalID: created.ID,
		Reason:     "APPROVAL_REQUIRED: awaiting operator approval",
		PolicyID:   policy.ID,
		ExpiresAt:  expiresAt,
	}, nil
}

// Approve approves a pending call approval request. Idempotent — re-approving an
// already-approved request is a no-op.
func (s *ApprovalService) Approve(ctx context.Context, approvalID, decidedBy, reason string) error {
	return s.repo.ApproveRequest(ctx, approvalID, decidedBy, reason)
}

// Deny denies a pending call approval request.
func (s *ApprovalService) Deny(ctx context.Context, approvalID, decidedBy, reason string) error {
	return s.repo.DenyRequest(ctx, approvalID, decidedBy, reason)
}

// VerifyApproved checks that an approval request exists and is in "approved" state.
// NNR: This MUST be called before initiating any call.
func (s *ApprovalService) VerifyApproved(ctx context.Context, approvalID string) (*ApprovalRequestRow, error) {
	req, err := s.repo.GetApprovalRequest(ctx, approvalID)
	if err != nil {
		return nil, fmt.Errorf("approval not found: %w", err)
	}
	if req.Status != "approved" && req.Status != "auto_approved" {
		return nil, fmt.Errorf("APPROVAL_NOT_GRANTED: approval %s is %s", approvalID, req.Status)
	}
	if time.Now().UTC().After(req.ExpiresAt) {
		return nil, fmt.Errorf("APPROVAL_EXPIRED: approval %s expired at %s", approvalID, req.ExpiresAt.Format(time.RFC3339))
	}
	return req, nil
}

// ExpirePending expires all pending approvals that have passed their expiry time.
func (s *ApprovalService) ExpirePending(ctx context.Context, workspaceID string) (int, error) {
	return s.repo.ExpirePendingRequests(ctx, workspaceID)
}

// EvaluateProviderHealth checks a provider's recent health and determines if failover
// should be triggered. The threshold is hard-coded at 5% error rate.
func EvaluateProviderHealth(errorCount, successCount int) (shouldFailover bool, healthScore float64) {
	total := errorCount + successCount
	if total == 0 {
		return false, 1.0
	}
	errorRate := float64(errorCount) / float64(total) * 100
	healthScore = 1.0 - (float64(errorCount) / float64(total))
	if healthScore < 0 {
		healthScore = 0
	}
	return errorRate > FailoverThreshold, healthScore
}

func hashNumber(phone string) string {
	h := sha256.Sum256([]byte(phone))
	return hex.EncodeToString(h[:])
}
