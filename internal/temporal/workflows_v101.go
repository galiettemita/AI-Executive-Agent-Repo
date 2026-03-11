package temporal

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SubscriptionReconciliationWorkflow ingests a Stripe event and reconciles the MRR snapshot.
// Idempotent: duplicate stripe_event_id is a no-op.
func SubscriptionReconciliationWorkflow(ctx workflow.Context, input IngestSubscriptionEventInput) (*ReconcileMRRResult, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("SubscriptionReconciliationWorkflow started", "workspaceID", input.WorkspaceID, "stripeEventID", input.StripeEventID)

	var a *Activities
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    60 * time.Second,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Ingest the subscription event idempotently.
	var ingestResult IngestSubscriptionEventResult
	err := workflow.ExecuteActivity(ctx, a.IngestSubscriptionEventActivity, input).Get(ctx, &ingestResult)
	if err != nil {
		return nil, err
	}

	// Step 2: Reconcile MRR snapshot for today.
	wfInfo := workflow.GetInfo(ctx)
	// Use workflow start time for determinism.
	date := wfInfo.WorkflowStartTime.Format("2006-01-02")

	var mrrResult ReconcileMRRResult
	err = workflow.ExecuteActivity(ctx, a.ReconcileMRRActivity, ReconcileMRRInput{
		WorkspaceID: input.WorkspaceID,
		Date:        date,
	}).Get(ctx, &mrrResult)
	if err != nil {
		return nil, err
	}

	return &mrrResult, nil
}
