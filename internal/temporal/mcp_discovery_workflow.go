package temporal

import (
	"time"

	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// MCPToolDiscoveryWorkflow runs MCP tool discovery on a 30-minute interval.
func MCPToolDiscoveryWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporalsdk.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities
	return workflow.ExecuteActivity(ctx, activities.MCPToolDiscoveryActivity).Get(ctx, nil)
}

// MCPToolDiscoveryCronSchedule is the cron schedule for MCP discovery (every 30 minutes).
const MCPToolDiscoveryCronSchedule = "*/30 * * * *"
