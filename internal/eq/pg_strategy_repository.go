package eq

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// EQStrategyRow represents a row from the eq_strategy_matrix table.
type EQStrategyRow struct {
	ID                  string    `json:"id"`
	WorkspaceID         string    `json:"workspace_id"`
	TriggerPattern      string    `json:"trigger_pattern"`
	DetectedEmotion     string    `json:"detected_emotion"`
	RecommendedStrategy string    `json:"recommended_strategy"`
	Confidence          float64   `json:"confidence"`
	SuccessCount        int       `json:"success_count"`
	FailureCount        int       `json:"failure_count"`
	LastAppliedAt       *time.Time `json:"last_applied_at"`
	Metadata            string    `json:"metadata"`
}

// EmotionalContextEntry represents a row in the emotional_context_log table.
type EmotionalContextEntry struct {
	ID              string  `json:"id"`
	WorkspaceID     string  `json:"workspace_id"`
	UserID          string  `json:"user_id"`
	SessionID       string  `json:"session_id"`
	DetectedValence string  `json:"detected_valence"`
	Confidence      float64 `json:"confidence"`
	Signals         string  `json:"signals"`
	StrategyApplied string  `json:"strategy_applied"`
	Outcome         string  `json:"outcome"`
}

// EQStrategyRepository provides DB-backed EQ strategy operations.
type EQStrategyRepository interface {
	GetBestStrategy(ctx context.Context, workspaceID, triggerPattern, detectedEmotion string) (*EQStrategyRow, error)
	UpsertStrategy(ctx context.Context, workspaceID, triggerPattern, detectedEmotion, strategy string, confidence float64) error
	RecordOutcome(ctx context.Context, workspaceID, triggerPattern, detectedEmotion string, success bool) error
	LogEmotionalContext(ctx context.Context, entry EmotionalContextEntry) error
	GetRecentEmotionalContext(ctx context.Context, workspaceID, userID string, limit int) ([]EmotionalContextEntry, error)
}

// PgEQStrategyRepository implements EQStrategyRepository backed by pgx.
type PgEQStrategyRepository struct {
	q database.Querier
}

// NewPgEQStrategyRepository creates a new PgEQStrategyRepository.
func NewPgEQStrategyRepository(q database.Querier) *PgEQStrategyRepository {
	return &PgEQStrategyRepository{q: q}
}

// GetBestStrategy retrieves the highest-confidence strategy for a trigger/emotion pair.
func (r *PgEQStrategyRepository) GetBestStrategy(ctx context.Context, workspaceID, triggerPattern, detectedEmotion string) (*EQStrategyRow, error) {
	var row EQStrategyRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, trigger_pattern, detected_emotion, recommended_strategy,
		        confidence, success_count, failure_count, last_applied_at
		 FROM eq_strategy_matrix
		 WHERE workspace_id = $1::uuid AND trigger_pattern = $2 AND detected_emotion = $3::emotional_valence
		 ORDER BY confidence DESC
		 LIMIT 1`,
		workspaceID, triggerPattern, detectedEmotion,
	).Scan(&row.ID, &row.WorkspaceID, &row.TriggerPattern, &row.DetectedEmotion,
		&row.RecommendedStrategy, &row.Confidence, &row.SuccessCount, &row.FailureCount, &row.LastAppliedAt)
	if err != nil {
		return nil, fmt.Errorf("get best strategy: %w", err)
	}
	return &row, nil
}

// UpsertStrategy creates or updates an EQ strategy entry.
func (r *PgEQStrategyRepository) UpsertStrategy(ctx context.Context, workspaceID, triggerPattern, detectedEmotion, strategy string, confidence float64) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO eq_strategy_matrix (workspace_id, trigger_pattern, detected_emotion, recommended_strategy, confidence)
		 VALUES ($1::uuid, $2, $3::emotional_valence, $4::eq_strategy, $5)
		 ON CONFLICT (workspace_id, trigger_pattern, detected_emotion) DO UPDATE SET
		   recommended_strategy = EXCLUDED.recommended_strategy,
		   confidence = EXCLUDED.confidence,
		   updated_at = now()`,
		workspaceID, triggerPattern, detectedEmotion, strategy, confidence,
	)
	if err != nil {
		return fmt.Errorf("upsert strategy: %w", err)
	}
	return nil
}

// RecordOutcome increments success or failure count and updates last_applied_at.
func (r *PgEQStrategyRepository) RecordOutcome(ctx context.Context, workspaceID, triggerPattern, detectedEmotion string, success bool) error {
	var col string
	if success {
		col = "success_count"
	} else {
		col = "failure_count"
	}
	_, err := r.q.Exec(ctx,
		fmt.Sprintf(`UPDATE eq_strategy_matrix SET %s = %s + 1, last_applied_at = now(), updated_at = now()
		 WHERE workspace_id = $1::uuid AND trigger_pattern = $2 AND detected_emotion = $3::emotional_valence`, col, col),
		workspaceID, triggerPattern, detectedEmotion,
	)
	if err != nil {
		return fmt.Errorf("record outcome: %w", err)
	}
	return nil
}

// LogEmotionalContext inserts an emotional context log entry.
func (r *PgEQStrategyRepository) LogEmotionalContext(ctx context.Context, entry EmotionalContextEntry) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO emotional_context_log (workspace_id, user_id, session_id, detected_valence, confidence, signals, strategy_applied, outcome)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::emotional_valence, $5, $6::jsonb, $7::eq_strategy, $8)`,
		entry.WorkspaceID, entry.UserID, entry.SessionID, entry.DetectedValence,
		entry.Confidence, entry.Signals, entry.StrategyApplied, entry.Outcome,
	)
	if err != nil {
		return fmt.Errorf("log emotional context: %w", err)
	}
	return nil
}

// GetRecentEmotionalContext returns recent emotional context entries for a user.
func (r *PgEQStrategyRepository) GetRecentEmotionalContext(ctx context.Context, workspaceID, userID string, limit int) ([]EmotionalContextEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, user_id, detected_valence, confidence, strategy_applied, outcome
		 FROM emotional_context_log
		 WHERE workspace_id = $1::uuid AND user_id = $2::uuid
		 ORDER BY created_at DESC LIMIT $3`,
		workspaceID, userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get recent emotional context: %w", err)
	}
	defer rows.Close()

	var result []EmotionalContextEntry
	for rows.Next() {
		var e EmotionalContextEntry
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.UserID, &e.DetectedValence,
			&e.Confidence, &e.StrategyApplied, &e.Outcome); err != nil {
			return nil, fmt.Errorf("scan emotional context: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
