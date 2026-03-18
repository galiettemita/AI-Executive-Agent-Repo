package temporal

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
)

// EQPromotionCheckActivity checks A/B results across all workspaces and logs
// promotion recommendations for FEATURE_EQ_PROMPT_ROUTING where treatment
// shows ≥5% ORM improvement.
func (a *Activities) EQPromotionCheckActivity(ctx context.Context) error {
	if a.pool == nil || a.eqABTracker == nil {
		return nil
	}

	rows, err := a.pool.Query(ctx,
		`SELECT DISTINCT workspace_id FROM eq_ab_results`)
	if err != nil {
		return fmt.Errorf("EQPromotionCheckActivity: list workspaces: %w", err)
	}
	defer rows.Close()

	var workspaceIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}
		workspaceIDs = append(workspaceIDs, id)
	}

	for _, wsID := range workspaceIDs {
		summary, err := a.eqABTracker.GetABSummary(ctx, wsID)
		if err != nil {
			// Insufficient samples — skip silently.
			continue
		}

		if summary.ShouldPromote {
			log.Printf("[EQ A/B] workspace %s: EQ routing shows %.1f%% ORM improvement (control=%.4f treatment=%.4f) — recommend promotion",
				wsID, summary.ImprovementPct, summary.ControlAvg, summary.TreatmentAvg)

			// Best-effort: record promotion event.
			_, _ = a.pool.Exec(ctx,
				`INSERT INTO eq_ab_results (workspace_id, request_id, orm_score, eq_enabled)
				 VALUES ($1, $2, $3, true)
				 ON CONFLICT DO NOTHING`,
				wsID, fmt.Sprintf("promotion_event_%s", wsID), summary.TreatmentAvg,
			)
		}
	}
	return nil
}
