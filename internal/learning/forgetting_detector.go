package learning

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ForgettingDetector monitors anchored lesson reuse rates and alerts on degradation.
type ForgettingDetector struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewForgettingDetector creates a forgetting detector.
func NewForgettingDetector(db *pgxpool.Pool, logger *slog.Logger) *ForgettingDetector {
	return &ForgettingDetector{db: db, logger: logger}
}

// DetectForgetting checks all anchored lessons for reuse rate drops > 20%.
func (d *ForgettingDetector) DetectForgetting(ctx context.Context) error {
	if d.db == nil {
		return nil
	}

	rows, err := d.db.Query(ctx,
		`SELECT ll.id, ll.workspace_id FROM learned_lessons ll WHERE ll.is_anchor = TRUE`,
	)
	if err != nil {
		return fmt.Errorf("query anchored lessons: %w", err)
	}
	defer rows.Close()

	alertCount := 0
	for rows.Next() {
		var lessonID, wsID uuid.UUID
		if err := rows.Scan(&lessonID, &wsID); err != nil {
			continue
		}

		var currentRate int
		_ = d.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM lesson_usages WHERE lesson_id=$1 AND used_at > NOW() - INTERVAL '7 days'`,
			lessonID,
		).Scan(&currentRate)

		var baselineRate float64
		_ = d.db.QueryRow(ctx,
			`SELECT COALESCE(AVG(weekly_reuse_count), 0) FROM lesson_reuse_baselines WHERE lesson_id=$1`,
			lessonID,
		).Scan(&baselineRate)

		if baselineRate > 0 && float64(currentRate) < baselineRate*0.80 {
			d.logger.Warn("forgetting_detected",
				"lesson_id", lessonID,
				"workspace_id", wsID,
				"current_rate", currentRate,
				"baseline_rate", baselineRate,
			)
			alertCount++
		}
	}

	d.logger.Info("forgetting_detection_complete", "alerts", alertCount)
	return nil
}

// UpdateBaselines records the current week's reuse counts for all anchored lessons.
func (d *ForgettingDetector) UpdateBaselines(ctx context.Context) error {
	if d.db == nil {
		return nil
	}

	_, err := d.db.Exec(ctx,
		`INSERT INTO lesson_reuse_baselines (lesson_id, workspace_id, weekly_reuse_count, recorded_week)
		 SELECT lu.lesson_id, lu.workspace_id, COUNT(*), date_trunc('week', NOW())::date
		 FROM lesson_usages lu
		 JOIN learned_lessons ll ON ll.id = lu.lesson_id AND ll.is_anchor = TRUE
		 WHERE lu.used_at > NOW() - INTERVAL '7 days'
		 GROUP BY lu.lesson_id, lu.workspace_id
		 ON CONFLICT (lesson_id, recorded_week) DO UPDATE SET weekly_reuse_count = EXCLUDED.weekly_reuse_count`,
	)
	return err
}

// ForgettingDetectorWorkflow runs weekly: update baselines, detect forgetting, promote anchors.
func ForgettingDetectorWorkflow(ctx workflow.Context) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("ForgettingDetectorWorkflow started")

	actCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	})

	if err := workflow.ExecuteActivity(actCtx, UpdateBaselinesActivity).Get(ctx, nil); err != nil {
		logger.Error("update baselines failed", "error", err)
	}
	if err := workflow.ExecuteActivity(actCtx, DetectForgettingActivity).Get(ctx, nil); err != nil {
		logger.Error("detect forgetting failed", "error", err)
	}
	if err := workflow.ExecuteActivity(actCtx, RunAnchorPromotionActivity).Get(ctx, nil); err != nil {
		logger.Error("anchor promotion failed", "error", err)
	}

	logger.Info("ForgettingDetectorWorkflow complete")
	return nil
}

// Activity stubs for Temporal registration.
func UpdateBaselinesActivity(_ context.Context) error     { return nil }
func DetectForgettingActivity(_ context.Context) error    { return nil }
func RunAnchorPromotionActivity(_ context.Context) error  { return nil }
