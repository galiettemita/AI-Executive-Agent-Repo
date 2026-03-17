package temporal

import (
	"fmt"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/a2a"
)

// A2ATaskExecutionWorkflow executes an outbound A2A delegation as a Temporal workflow.
func A2ATaskExecutionWorkflow(ctx workflow.Context, req a2a.DelegateRequest) (*a2a.DelegateResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("A2ATaskExecutionWorkflow starting",
		"workspace_id", req.WorkspaceID,
		"capability", req.Capability)

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporalSDK.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    5 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	var result a2a.DelegateResult
	if err := workflow.ExecuteActivity(ctx, "DelegateA2ATaskActivity", req).Get(ctx, &result); err != nil {
		return nil, fmt.Errorf("A2ATaskExecutionWorkflow: %w", err)
	}

	logger.Info("A2ATaskExecutionWorkflow complete",
		"task_id", result.TaskID,
		"status", string(result.Status))
	return &result, nil
}
