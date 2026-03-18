package temporal

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// WorldModelExpirySweepActivity deletes expired world model facts.
func (a *Activities) WorldModelExpirySweepActivity(ctx context.Context, workspaceIDs []string) (int, error) {
	if a.worldModelRepo == nil {
		return 0, nil
	}

	var totalDeleted int
	for _, wsIDStr := range workspaceIDs {
		wsID, err := uuid.Parse(wsIDStr)
		if err != nil {
			continue
		}
		count, err := a.worldModelRepo.ExpireFacts(ctx, wsID)
		if err != nil {
			continue
		}
		totalDeleted += count
	}
	return totalDeleted, nil
}

// WorldModelExpirySweepWorkspaceListActivity fetches workspace IDs with expired facts.
func (a *Activities) WorldModelExpirySweepWorkspaceListActivity(ctx context.Context) ([]string, error) {
	if a.pool == nil {
		return nil, nil
	}
	rows, err := a.pool.Query(ctx,
		"SELECT DISTINCT workspace_id::text FROM world_model_facts WHERE expires_at <= now()")
	if err != nil {
		return nil, fmt.Errorf("WorldModelExpirySweep: query workspaces: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// WorldModelExpiryCronWorkflow runs the expiry sweep hourly.
func WorldModelExpiryCronWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities

	// Step 1: Get workspace IDs with expired facts.
	var workspaceIDs []string
	if err := workflow.ExecuteActivity(ctx, activities.WorldModelExpirySweepWorkspaceListActivity).Get(ctx, &workspaceIDs); err != nil {
		return fmt.Errorf("list expired workspaces: %w", err)
	}

	if len(workspaceIDs) == 0 {
		return nil
	}

	// Step 2: Delete expired facts.
	var deleted int
	if err := workflow.ExecuteActivity(ctx, activities.WorldModelExpirySweepActivity, workspaceIDs).Get(ctx, &deleted); err != nil {
		return fmt.Errorf("expire facts: %w", err)
	}

	return nil
}
