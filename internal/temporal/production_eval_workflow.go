package temporal

import (
	"fmt"
	"time"

	temporalSDK "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ProductionEvalSamplerWorkflow runs as a Temporal cron every hour.
// It samples 5% of recently completed workflows and re-scores them.
func ProductionEvalSamplerWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ProductionEvalSamplerWorkflow starting")

	actOpts := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporalSDK.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, actOpts)

	var result ProductionEvalSampleResult
	if err := workflow.ExecuteActivity(ctx, "ProductionEvalSampleActivity").Get(ctx, &result); err != nil {
		logger.Error("ProductionEvalSampleActivity failed", "error", err)
		return err
	}

	logger.Info("ProductionEvalSamplerWorkflow complete",
		"sample_count", result.SampleCount,
		"pass_rate", fmt.Sprintf("%.4f", result.PassRate))

	if result.PassRate < 0.85 && result.SampleCount > 0 {
		logger.Error("ALERT: production quality pass rate below 85% threshold",
			"pass_rate", result.PassRate)
	}

	return nil
}

// ProductionEvalSampleResult is the output of ProductionEvalSampleActivity.
type ProductionEvalSampleResult struct {
	SampleCount int     `json:"sample_count"`
	PassRate    float64 `json:"pass_rate"`
}
