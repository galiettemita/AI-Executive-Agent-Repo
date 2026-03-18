package brain

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// PGCriticTraceRepository persists CriticOutput to PostgreSQL.
type PGCriticTraceRepository struct {
	db *sql.DB
}

func NewPGCriticTraceRepository(db *sql.DB) *PGCriticTraceRepository {
	return &PGCriticTraceRepository{db: db}
}

func (r *PGCriticTraceRepository) Save(ctx context.Context, output CriticOutput) error {
	failureModesJSON, _ := json.Marshal(output.FailureModes)
	dimensionsJSON, _ := json.Marshal(output.DimensionScores)
	rawJSON, _ := json.Marshal(output)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO critic_traces
			(workspace_id, request_id, iteration, quality_score, should_retry,
			 semantic_verdict, issues, retry_hints, step_verdicts, raw_output)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		output.WorkspaceID,
		output.RequestID,
		output.Iteration,
		output.OverallScore,
		!output.Passed,
		output.ImprovementDirective,
		failureModesJSON,
		dimensionsJSON,
		nil, // step_verdicts — not applicable for CriticOutput
		rawJSON,
	)
	if err != nil {
		return fmt.Errorf("pg_critic_trace: save: %w", err)
	}
	return nil
}

// StoreORMResult persists an ORM evaluation result for trend analysis.
func (r *PGCriticTraceRepository) StoreORMResult(
	ctx context.Context,
	workspaceID, intent string,
	score *OutcomeScore,
) error {
	sideEffectsJSON, _ := json.Marshal(score.SideEffects)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO critic_traces
			(workspace_id, request_id, iteration, quality_score, should_retry,
			 semantic_verdict, issues, retry_hints, step_verdicts, raw_output)
		VALUES ($1, $2, 0, $3, $4, $5, $6, $7, NULL, $8)`,
		workspaceID,
		"orm:"+intent,
		score.OverallQuality/5.0, // normalize to 0-1 for consistency with critic_traces schema
		!score.IntentSatisfied,
		score.ImprovementHints,
		sideEffectsJSON,
		fmt.Sprintf(`{"completeness":%f,"accuracy":%f}`, score.Completeness, score.Accuracy),
		fmt.Sprintf(`{"score_type":"orm","overall_quality":%f,"latency_ms":%d}`,
			score.OverallQuality, score.LatencyMs),
	)
	if err != nil {
		return fmt.Errorf("pg_critic_trace: store_orm_result: %w", err)
	}
	return nil
}
