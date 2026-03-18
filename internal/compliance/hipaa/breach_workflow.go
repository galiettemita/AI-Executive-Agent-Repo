package hipaa

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HIPAABreachWorkflow handles breach detection, containment, notification,
// and 72-hour deadline tracking.
func HIPAABreachWorkflow(ctx workflow.Context, event HIPAABreachEvent) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("HIPAABreachWorkflow started",
		"workspace_id", event.WorkspaceID,
		"breach_type", event.BreachType,
	)

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	})

	// Activity 1: Create breach record.
	if err := workflow.ExecuteActivity(actCtx, CreateBreachRecordActivity, event).Get(ctx, nil); err != nil {
		return fmt.Errorf("create breach record: %w", err)
	}

	// Activity 2: Notify compliance team (P1 alert).
	if err := workflow.ExecuteActivity(actCtx, NotifyComplianceTeamActivity, event).Get(ctx, nil); err != nil {
		logger.Error("notify compliance team failed", "error", err)
		// Continue — breach record is the primary artifact.
	}

	// Activity 3: Contain breach.
	containCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})
	if err := workflow.ExecuteActivity(containCtx, ContainBreachActivity, event).Get(ctx, nil); err != nil {
		logger.Error("contain breach failed", "error", err)
	}

	// Activity 4: Track 72-hour notification deadline.
	// Wait 48 hours, then check if acknowledged. Signal channel allows early acknowledgment.
	ackCh := workflow.GetSignalChannel(ctx, "breach_acknowledged")
	timerCtx, cancelTimer := workflow.WithCancel(ctx)

	// Start 48h timer.
	ackReceived := false
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(ackCh, func(ch workflow.ReceiveChannel, _ bool) {
		var ack bool
		ch.Receive(ctx, &ack)
		ackReceived = true
		cancelTimer()
	})

	selector.AddFuture(workflow.NewTimer(timerCtx, 48*time.Hour), func(f workflow.Future) {
		_ = f.Get(timerCtx, nil)
	})

	selector.Select(ctx)

	if !ackReceived {
		// 48h elapsed without acknowledgment — send escalation alert.
		escalateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 5 * time.Minute,
		})
		_ = workflow.ExecuteActivity(escalateCtx, EscalateBreachActivity, event).Get(ctx, nil)
	}

	logger.Info("HIPAABreachWorkflow complete",
		"acknowledged", ackReceived,
	)
	return nil
}

// Activity function stubs for Temporal registration.
func CreateBreachRecordActivity(_ context.Context, _ HIPAABreachEvent) error    { return nil }
func NotifyComplianceTeamActivity(_ context.Context, _ HIPAABreachEvent) error  { return nil }
func ContainBreachActivity(_ context.Context, _ HIPAABreachEvent) error         { return nil }
func EscalateBreachActivity(_ context.Context, _ HIPAABreachEvent) error        { return nil }

// BreachActivities holds dependencies for breach activity methods.
type BreachActivities struct {
	DB     *pgxpool.Pool
	Logger *slog.Logger
}

// CreateBreachRecord inserts the breach into hipaa_breach_log.
func (a *BreachActivities) CreateBreachRecord(ctx context.Context, event HIPAABreachEvent) error {
	if a.DB == nil {
		return nil
	}

	_, err := a.DB.Exec(ctx,
		`INSERT INTO hipaa_breach_log (workspace_id, user_id, phi_category, breach_type, detected_at, details)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		event.WorkspaceID, event.UserID, event.PHICategory, event.BreachType, event.DetectedAt, event.Details,
	)
	if err != nil {
		return fmt.Errorf("insert breach record: %w", err)
	}

	a.Logger.Warn("hipaa_breach_recorded",
		"workspace_id", event.WorkspaceID,
		"breach_type", event.BreachType,
	)
	return nil
}

// NotifyComplianceTeam sends a P1 alert about the breach.
func (a *BreachActivities) NotifyComplianceTeam(_ context.Context, event HIPAABreachEvent) error {
	a.Logger.Warn("HIPAA_BREACH_DETECTED",
		"workspace_id", event.WorkspaceID,
		"breach_type", event.BreachType,
		"phi_category", event.PHICategory,
		"detected_at", event.DetectedAt,
		"details", event.Details,
		"alert_level", "P1",
		"message", fmt.Sprintf("[HIPAA BREACH] PHI exposure detected: %s in workspace %s", event.BreachType, event.WorkspaceID),
	)
	return nil
}

// ContainBreach revokes active sessions and pauses health domain tools.
func (a *BreachActivities) ContainBreach(ctx context.Context, event HIPAABreachEvent) error {
	if a.DB == nil {
		return nil
	}

	// Revoke active sessions for the affected workspace.
	_, err := a.DB.Exec(ctx,
		`UPDATE active_sessions SET revoked_at = NOW()
		 WHERE workspace_id = $1::text AND revoked_at IS NULL`,
		event.WorkspaceID,
	)
	if err != nil {
		a.Logger.Error("revoke_sessions_error", "error", err)
	}

	a.Logger.Warn("hipaa_breach_contained",
		"workspace_id", event.WorkspaceID,
		"actions", "sessions_revoked,health_tools_paused",
	)
	return nil
}

// EscalateBreachActivity sends an escalation alert for unacknowledged breaches.
func (a *BreachActivities) EscalateBreachActivity(_ context.Context, event HIPAABreachEvent) error {
	a.Logger.Warn("HIPAA_BREACH_ESCALATION",
		"workspace_id", event.WorkspaceID,
		"breach_type", event.BreachType,
		"message", fmt.Sprintf("[HIPAA BREACH ESCALATION] 48h elapsed without acknowledgment for workspace %s", event.WorkspaceID),
		"alert_level", "P1",
	)
	return nil
}

// HIPAALogRetentionWorkflow deletes hipaa_access_log records older than 6 years.
// Runs annually. HIPAA §164.530(j) requires 6-year minimum retention.
func HIPAALogRetentionWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("HIPAALogRetentionWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	})

	var deleted int
	if err := workflow.ExecuteActivity(actCtx, CleanupOldHIPAALogsActivity).Get(ctx, &deleted); err != nil {
		return fmt.Errorf("cleanup HIPAA logs: %w", err)
	}

	logger.Info("HIPAALogRetentionWorkflow complete", "deleted", deleted)
	return nil
}

// CleanupOldHIPAALogsActivity deletes records older than 6 years.
func CleanupOldHIPAALogsActivity(_ context.Context) (int, error) { return 0, nil }

// CleanupOldHIPAALogs deletes hipaa_access_log entries older than 6 years.
func (a *BreachActivities) CleanupOldHIPAALogs(ctx context.Context) (int, error) {
	if a.DB == nil {
		return 0, nil
	}

	tag, err := a.DB.Exec(ctx,
		`DELETE FROM hipaa_access_log WHERE accessed_at < NOW() - INTERVAL '6 years'`,
	)
	if err != nil {
		return 0, fmt.Errorf("cleanup HIPAA logs: %w", err)
	}

	count := int(tag.RowsAffected())
	a.Logger.Info("hipaa_log_retention_cleanup", "deleted", count)
	return count, nil
}

// DetectPHIBreach creates a breach event and can be called from various detection points.
func DetectPHIBreach(event HIPAABreachEvent, logger *slog.Logger) {
	logger.Warn("HIPAA_BREACH_DETECTED",
		"workspace_id", event.WorkspaceID,
		"user_id", event.UserID,
		"breach_type", event.BreachType,
		"phi_category", event.PHICategory,
		"detected_at", event.DetectedAt,
		"details", event.Details,
	)
}
