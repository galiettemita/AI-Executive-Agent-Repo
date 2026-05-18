package hands

import (
	"context"
	"fmt"
)

// ExecutorAdapter adapts the hands Service to satisfy the HandsExecutor interface
// used by Temporal activities.
type ExecutorAdapter struct {
	svc *Service
}

// NewExecutorAdapter creates an adapter wrapping the hands service.
func NewExecutorAdapter(svc *Service) *ExecutorAdapter {
	return &ExecutorAdapter{svc: svc}
}

// ExecuteTool calls the hands runtime to execute a skill.
func (a *ExecutorAdapter) ExecuteTool(ctx context.Context, skillID, workspaceID, receiptID, idempotencyKey, mode string, args map[string]interface{}) (bool, any, error) {
	result := a.svc.Execute(ctx, ExecuteRequest{
		SkillID:        skillID,
		WorkspaceID:    workspaceID,
		ReceiptID:      receiptID,
		IdempotencyKey: idempotencyKey,
		Mode:           mode,
		Args:           args,
	})

	if result.Status == "FAILED" {
		errMsg := "execution failed"
		if result.Error != nil {
			errMsg = result.Error.Message
		}
		return false, result.Data, fmt.Errorf("hands: %s", errMsg)
	}

	return true, result.Data, nil
}
