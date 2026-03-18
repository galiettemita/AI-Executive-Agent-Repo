package temporal

import (
	"fmt"
	"time"

	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EUAIActComplianceWorkflow is a Temporal daily cron workflow that:
// 1. Aggregates new risks from the previous 24h
// 2. Checks incident thresholds
// 3. Generates conformity evidence
// Schedule: "0 2 * * *" (daily at 02:00 UTC)
func EUAIActComplianceWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporalsdk.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities

	// 1. Aggregate risks
	var risksRecorded int
	if err := workflow.ExecuteActivity(ctx,
		activities.EUAIActAggregateRisksActivity,
	).Get(ctx, &risksRecorded); err != nil {
		workflow.GetLogger(ctx).Error("EUAIActAggregateRisksActivity failed", "error", err)
	}

	// 2. Check incident thresholds
	var incidentCount int
	if err := workflow.ExecuteActivity(ctx,
		activities.EUAIActCheckIncidentThresholdsActivity,
	).Get(ctx, &incidentCount); err != nil {
		workflow.GetLogger(ctx).Error("EUAIActCheckIncidentThresholdsActivity failed", "error", err)
	}

	// 3. Generate conformity evidence
	if err := workflow.ExecuteActivity(ctx,
		activities.EUAIActGenerateConformityEvidenceActivity,
	).Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Error("EUAIActGenerateConformityEvidenceActivity failed", "error", err)
	}

	workflow.GetLogger(ctx).Info("EUAIActComplianceWorkflow done",
		"risks_recorded", risksRecorded, "incidents_24h", incidentCount)
	return nil
}

// EUAIActComplianceWorkflowID returns the cron workflow ID.
func EUAIActComplianceWorkflowID() string {
	return "eu-ai-act-compliance-cron"
}

// EUAIActCronSchedule is the daily cron schedule (02:00 UTC).
const EUAIActCronSchedule = "0 2 * * *"

// Ensure fmt is used (for workflow ID formatting if needed).
var _ = fmt.Sprint
