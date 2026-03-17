package temporal

import (
	"fmt"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/proactive"
)

// ProactiveMonitorWorkflow runs every 15 minutes.
// It detects proactive signals, constructs offer messages, and dispatches them.
// It NEVER executes actions — only offers.
func ProactiveMonitorWorkflow(ctx workflow.Context, workspaceID string) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ProactiveMonitorWorkflow starting", "workspace_id", workspaceID)

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporalSDK.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	var signals []proactive.Signal
	if err := workflow.ExecuteActivity(ctx, "DetectProactiveSignalsActivity", workspaceID).Get(ctx, &signals); err != nil {
		logger.Error("DetectProactiveSignalsActivity failed", "error", err)
		return nil
	}

	if len(signals) == 0 {
		return nil
	}

	for _, s := range signals {
		var offerText string
		if err := workflow.ExecuteActivity(ctx, "BuildAndDispatchProactiveOfferActivity", s).Get(ctx, &offerText); err != nil {
			logger.Error("BuildAndDispatchProactiveOfferActivity failed",
				"signal_type", string(s.Type), "error", err)
		} else {
			preview := offerText
			if len(preview) > 60 {
				preview = preview[:60] + "..."
			}
			logger.Info("ProactiveMonitorWorkflow: offer dispatched",
				"workspace_id", workspaceID,
				"signal_type", string(s.Type),
				"offer_preview", fmt.Sprintf("%s", preview))
		}
	}

	return nil
}
