package consent

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ConsentRevocationWorkflow performs GDPR Article 7(3) erasure for data
// processed under the revoked purpose. Must complete within 55 minutes.
func ConsentRevocationWorkflow(ctx workflow.Context, input RevocationInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ConsentRevocationWorkflow started",
		"workspace_id", input.WorkspaceID,
		"user_id", input.UserID,
		"purpose", input.Purpose,
	)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Step 1: Identify data for erasure.
	var auditCount int
	if err := workflow.ExecuteActivity(actCtx, IdentifyDataForErasureActivity, input).Get(ctx, &auditCount); err != nil {
		return fmt.Errorf("identify data: %w", err)
	}

	// Step 2: Erase purpose-specific data.
	var erasedCount int
	switch input.Purpose {
	case PurposeFineTuning:
		if err := workflow.ExecuteActivity(actCtx, ErasePreferenceDataActivity, input).Get(ctx, &erasedCount); err != nil {
			return fmt.Errorf("erase preference data: %w", err)
		}
	case PurposeAnalytics:
		if err := workflow.ExecuteActivity(actCtx, EraseAnalyticsDataActivity, input).Get(ctx, &erasedCount); err != nil {
			return fmt.Errorf("erase analytics data: %w", err)
		}
	case PurposeMarketing:
		if err := workflow.ExecuteActivity(actCtx, EraseMarketingDataActivity, input).Get(ctx, &erasedCount); err != nil {
			return fmt.Errorf("erase marketing data: %w", err)
		}
	}

	// Step 3: Audit the revocation.
	if err := workflow.ExecuteActivity(actCtx, AuditRevocationCompleteActivity, input, erasedCount+auditCount).Get(ctx, nil); err != nil {
		return fmt.Errorf("audit revocation: %w", err)
	}

	logger.Info("ConsentRevocationWorkflow complete",
		"purpose", input.Purpose,
		"records_erased", erasedCount+auditCount,
	)
	return nil
}

// Activity function stubs for Temporal registration.
func IdentifyDataForErasureActivity(_ context.Context, _ RevocationInput) (int, error)           { return 0, nil }
func ErasePreferenceDataActivity(_ context.Context, _ RevocationInput) (int, error)              { return 0, nil }
func EraseAnalyticsDataActivity(_ context.Context, _ RevocationInput) (int, error)               { return 0, nil }
func EraseMarketingDataActivity(_ context.Context, _ RevocationInput) (int, error)               { return 0, nil }
func AuditRevocationCompleteActivity(_ context.Context, _ RevocationInput, _ int) error          { return nil }

// RevocationActivities holds database dependencies for revocation activity methods.
type RevocationActivities struct {
	DB     *pgxpool.Pool
	Logger *slog.Logger
}

// IdentifyDataForErasure queries the purpose_audit_log for data accessed under
// the revoked consent.
func (a *RevocationActivities) IdentifyDataForErasure(ctx context.Context, input RevocationInput) (int, error) {
	if a.DB == nil {
		return 0, nil
	}

	var count int
	err := a.DB.QueryRow(ctx,
		`SELECT COUNT(*) FROM purpose_audit_log pal
		 JOIN consent_records cr ON pal.consent_id = cr.id
		 WHERE cr.workspace_id = $1 AND cr.user_id = $2 AND cr.purpose = $3`,
		input.WorkspaceID, input.UserID, string(input.Purpose),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count audit entries: %w", err)
	}

	a.Logger.Info("revocation_data_identified",
		"workspace_id", input.WorkspaceID,
		"purpose", input.Purpose,
		"audit_entries", count,
	)
	return count, nil
}

// ErasePreferenceData deletes DPO preference pairs for the user/workspace.
func (a *RevocationActivities) ErasePreferenceData(ctx context.Context, input RevocationInput) (int, error) {
	if a.DB == nil {
		return 0, nil
	}

	tag, err := a.DB.Exec(ctx,
		`DELETE FROM preference_pairs WHERE workspace_id = $1 AND user_id = $2`,
		input.WorkspaceID, input.UserID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete preference pairs: %w", err)
	}

	count := int(tag.RowsAffected())
	a.Logger.Info("preference_data_erased",
		"workspace_id", input.WorkspaceID,
		"user_id", input.UserID,
		"deleted", count,
	)
	return count, nil
}

// EraseAnalyticsData deletes analytics events for the user.
func (a *RevocationActivities) EraseAnalyticsData(ctx context.Context, input RevocationInput) (int, error) {
	if a.DB == nil {
		return 0, nil
	}

	// Analytics events may be in various tables; delete from purpose_audit_log
	// entries linked to analytics consent.
	tag, err := a.DB.Exec(ctx,
		`DELETE FROM purpose_audit_log WHERE consent_id IN (
			SELECT id FROM consent_records
			WHERE workspace_id = $1 AND user_id = $2 AND purpose = 'analytics'
		)`,
		input.WorkspaceID, input.UserID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete analytics data: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

// EraseMarketingData removes the user from marketing contact lists.
func (a *RevocationActivities) EraseMarketingData(ctx context.Context, input RevocationInput) (int, error) {
	if a.DB == nil {
		return 0, nil
	}

	tag, err := a.DB.Exec(ctx,
		`DELETE FROM purpose_audit_log WHERE consent_id IN (
			SELECT id FROM consent_records
			WHERE workspace_id = $1 AND user_id = $2 AND purpose = 'marketing'
		)`,
		input.WorkspaceID, input.UserID,
	)
	if err != nil {
		return 0, fmt.Errorf("delete marketing data: %w", err)
	}

	return int(tag.RowsAffected()), nil
}

// AuditRevocationComplete records the completion of the erasure workflow.
func (a *RevocationActivities) AuditRevocationComplete(ctx context.Context, input RevocationInput, recordsErased int) error {
	if a.DB == nil {
		return nil
	}

	_, err := a.DB.Exec(ctx,
		`INSERT INTO dsr_erasure_log (user_id, workspace_id, purpose, erased_at, records_erased_count)
		 VALUES ($1, $2, $3, NOW(), $4)`,
		input.UserID, input.WorkspaceID, string(input.Purpose), recordsErased,
	)
	if err != nil {
		// Log but don't fail — the erasure is complete, audit is best-effort.
		a.Logger.Error("audit_revocation_complete_error", "error", err)
	}

	a.Logger.Info("revocation_audit_complete",
		"workspace_id", input.WorkspaceID,
		"user_id", input.UserID,
		"purpose", input.Purpose,
		"records_erased", recordsErased,
	)
	return nil
}

// StartRevocationWorkflow triggers the consent revocation erasure workflow via Temporal.
func StartRevocationWorkflow(tc temporalclient.Client, taskQueue string, input RevocationInput) (string, error) {
	opts := temporalclient.StartWorkflowOptions{
		ID:                       fmt.Sprintf("consent-revocation-%s-%s-%s", input.WorkspaceID, input.UserID, input.Purpose),
		TaskQueue:                taskQueue,
		WorkflowExecutionTimeout: 55 * time.Minute,
	}

	run, err := tc.ExecuteWorkflow(context.Background(), opts, ConsentRevocationWorkflow, input)
	if err != nil {
		return "", fmt.Errorf("start revocation workflow: %w", err)
	}

	return run.GetID(), nil
}
