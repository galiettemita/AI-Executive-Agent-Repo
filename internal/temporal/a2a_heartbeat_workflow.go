package temporal

import (
	"time"

	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// AgentHeartbeatWorkflow pings all registered agents and marks stale ones inactive.
// Runs every 5 minutes via cron.
func AgentHeartbeatWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 4 * time.Minute,
		RetryPolicy: &temporalsdk.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities
	return workflow.ExecuteActivity(ctx, activities.AgentHeartbeatActivity).Get(ctx, nil)
}

// AgentHeartbeatCronSchedule runs every 5 minutes.
const AgentHeartbeatCronSchedule = "*/5 * * * *"
