package control

import (
	"context"
	"encoding/json"
	"time"
)

// DurableReceiptService wraps the in-memory ReceiptService with persistent
// storage via ReceiptRepository. All decisions and receipts are persisted
// with enough evidence to reconstruct why a tool was allowed/denied.
type DurableReceiptService struct {
	inner *ReceiptService
	repo  ReceiptRepository
}

// NewDurableReceiptService creates a durable receipt service.
// If repo is nil, persistence is skipped (devtest mode).
func NewDurableReceiptService(inner *ReceiptService, repo ReceiptRepository) *DurableReceiptService {
	return &DurableReceiptService{
		inner: inner,
		repo:  repo,
	}
}

// EvaluateAndIssue evaluates all gates, issues a receipt, and persists both
// the receipt and the gate decision record with full evidence.
func (d *DurableReceiptService) EvaluateAndIssue(ctx context.Context, req ReceiptRequest) (*Receipt, []GateEvaluation, error) {
	receipt, evals, err := d.inner.EvaluateAndIssue(req)

	// Persist the gate decision regardless of outcome.
	if d.repo != nil {
		decision := "allow"
		reasonCode := "all_gates_passed"
		if err != nil {
			decision = "deny"
			reasonCode = err.Error()
		}

		gateEvidence := make([]map[string]any, len(evals))
		for i, eval := range evals {
			gateEvidence[i] = map[string]any{
				"gate_name": eval.GateName,
				"decision":  eval.Decision,
				"reason":    eval.Reason,
				"duration":  eval.Duration.String(),
			}
		}

		inputJSON := map[string]any{
			"workspace_id":    req.WorkspaceID,
			"workflow_run_id": req.WorkflowRunID,
			"plan_id":         req.PlanID,
			"tool_keys":       req.ToolKeys,
			"risk_level":      req.RiskLevel,
			"policy_bundle":   req.PolicyBundle,
			"gate_evaluations": gateEvidence,
		}

		_ = d.repo.StoreGateDecision(ctx, &GateDecisionRecord{
			WorkspaceID:     req.WorkspaceID,
			Decision:        decision,
			ReasonCode:      reasonCode,
			InputJSON:       inputJSON,
			GateEvaluations: evals,
			CreatedAt:       time.Now().UTC(),
		})
	}

	if err != nil {
		return nil, evals, err
	}

	// Persist the receipt.
	if d.repo != nil {
		_ = d.repo.StoreReceipt(ctx, receipt)
	}

	return receipt, evals, nil
}

// ValidateReceipt validates a receipt (delegates to in-memory service).
func (d *DurableReceiptService) ValidateReceipt(receiptID, workspaceID, toolKey string) error {
	return d.inner.ValidateReceipt(receiptID, workspaceID, toolKey)
}

// ConsumeReceipt marks a receipt as consumed in both memory and persistence.
func (d *DurableReceiptService) ConsumeReceipt(ctx context.Context, receiptID string) error {
	err := d.inner.ConsumeReceipt(receiptID)
	if err != nil {
		return err
	}
	if d.repo != nil {
		_ = d.repo.ConsumeReceipt(ctx, receiptID)
	}
	return nil
}

// RevokeReceipt revokes a receipt in both memory and persistence.
func (d *DurableReceiptService) RevokeReceipt(ctx context.Context, receiptID, reason string) error {
	d.inner.mu.Lock()
	receipt, ok := d.inner.receipts[receiptID]
	if !ok {
		d.inner.mu.Unlock()
		return ErrNoReceipt
	}
	if receipt.RevokedAt != nil {
		d.inner.mu.Unlock()
		return ErrReceiptRevoked
	}
	now := time.Now().UTC()
	receipt.RevokedAt = &now
	d.inner.mu.Unlock()

	if d.repo != nil {
		_ = d.repo.RevokeReceipt(ctx, receiptID, reason)
	}
	return nil
}

// PersistPolicyDecision stores a policy evaluation decision with full evidence
// for audit trail reconstruction.
func (d *DurableReceiptService) PersistPolicyDecision(ctx context.Context, workspaceID string, input PolicyInput, decision *PolicyDecision, evalDuration time.Duration) {
	if d.repo == nil {
		return
	}

	inputBytes, _ := json.Marshal(input)
	var inputMap map[string]any
	_ = json.Unmarshal(inputBytes, &inputMap)
	inputMap["eval_duration"] = evalDuration.String()

	if decision != nil {
		inputMap["decision_reason"] = decision.Reason
		inputMap["decision_allowed"] = decision.Allowed
		inputMap["requires_approval"] = decision.RequiresApproval
		inputMap["receipt_required"] = decision.ReceiptRequired
		if decision.Constraints != nil {
			inputMap["constraints"] = decision.Constraints
		}
	}

	decisionStr := "deny"
	reasonCode := "UNKNOWN"
	if decision != nil {
		if decision.RequiresApproval {
			decisionStr = "require_approval"
		} else if decision.Allowed {
			decisionStr = "allow"
		}
		reasonCode = decision.Reason
	}

	_ = d.repo.StoreGateDecision(ctx, &GateDecisionRecord{
		WorkspaceID: workspaceID,
		Decision:    decisionStr,
		ReasonCode:  reasonCode,
		InputJSON:   inputMap,
	})
}

// Inner returns the underlying ReceiptService (for testing/compatibility).
func (d *DurableReceiptService) Inner() *ReceiptService {
	return d.inner
}
