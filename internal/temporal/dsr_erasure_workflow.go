package temporal

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	temporalsdk "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/brevio/brevio/internal/compliance"
)

// DSRErasureInput is the input to DSRFullErasureWorkflow.
type DSRErasureInput struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	UserID      uuid.UUID `json:"user_id"`
	RequestID   uuid.UUID `json:"request_id"`
}

// DSRErasureOutput is the output of DSRFullErasureWorkflow.
type DSRErasureOutput struct {
	DeletedCounts compliance.DSRDeletedCounts `json:"deleted_counts"`
	Status        string                     `json:"status"`
}

// DSRFullErasureWorkflow orchestrates the complete GDPR Art. 17 erasure cascade.
// All 7 activities run sequentially. Any single activity failure retries up to 3 times
// before failing the workflow (allowing manual intervention).
func DSRFullErasureWorkflow(ctx workflow.Context, input DSRErasureInput) (DSRErasureOutput, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporalsdk.RetryPolicy{
			MaximumAttempts:    3,
			BackoffCoefficient: 2.0,
			InitialInterval:    5 * time.Second,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var activities *Activities
	var counts compliance.DSRDeletedCounts
	var n int

	// Step 1: Delete episodic memory
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRDeleteEpisodicMemoryActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 1 episodic: %w", err)
	}
	counts.EpisodicMemory = n

	// Step 2: Delete KG triples
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRDeleteKGTriplesActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 2 kg: %w", err)
	}
	counts.KGTriples = n

	// Step 3: Delete RAG vector chunks
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRDeleteVectorChunksActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 3 vectors: %w", err)
	}
	counts.VectorChunks = n

	// Step 4: Redact execution logs
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRRedactExecutionLogsActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 4 logs: %w", err)
	}
	counts.ExecutionLogs = n

	// Step 5: Nullify PII
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRNullifyPIIActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 5 pii: %w", err)
	}
	counts.PIIFields = n

	// Step 6: Revoke consent records
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRRevokeConsentActivity, input.UserID,
	).Get(ctx, &n); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 6 consent: %w", err)
	}
	counts.ConsentRecords = n

	// Step 7: Confirmation — update status, write evidence, emit audit event
	if err := workflow.ExecuteActivity(ctx,
		activities.DSRConfirmationActivity,
		DSRConfirmationInput{
			WorkspaceID:   input.WorkspaceID,
			UserID:        input.UserID,
			RequestID:     input.RequestID,
			DeletedCounts: counts,
		},
	).Get(ctx, nil); err != nil {
		return DSRErasureOutput{}, fmt.Errorf("step 7 confirmation: %w", err)
	}

	return DSRErasureOutput{
		DeletedCounts: counts,
		Status:        "completed",
	}, nil
}
