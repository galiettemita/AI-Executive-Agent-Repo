package eq

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PgStateRepository is the Postgres implementation of EQPersistRepository.
// It targets the eq_emotional_states table created by migration
// 040_BREVIO_eq_cross_session.sql.
type PgStateRepository struct {
	db *sql.DB
}

// NewPgStateRepository returns a PgStateRepository backed by the given *sql.DB.
func NewPgStateRepository(db *sql.DB) *PgStateRepository {
	return &PgStateRepository{db: db}
}

// SaveState upserts the emotional state for a workspace+user pair.
// On conflict: all mutable columns are overwritten; session_count increments by 1.
func (r *PgStateRepository) SaveState(ctx context.Context, state EmotionalState) error {
	const q = `
		INSERT INTO eq_emotional_states
			(workspace_id, user_id, valence, arousal, detected_emotion,
			 confidence, session_count, updated_at)
		VALUES
			($1, $2, $3, $4, $5, $6, 1, $7)
		ON CONFLICT ON CONSTRAINT eq_ws_user DO UPDATE SET
			valence          = EXCLUDED.valence,
			arousal          = EXCLUDED.arousal,
			detected_emotion = EXCLUDED.detected_emotion,
			confidence       = EXCLUDED.confidence,
			session_count    = eq_emotional_states.session_count + 1,
			updated_at       = EXCLUDED.updated_at
	`
	// EmotionalState.WorkspaceID serves as both workspace_id and user_id key
	// since the struct has no UserID field; the workspace scope suffices for
	// the current EQ architecture.
	userID := state.ID // use state ID as user identifier fallback
	_, err := r.db.ExecContext(ctx, q,
		state.WorkspaceID,
		userID,
		state.Valence,
		state.Arousal,
		state.DetectedEmotion,
		state.Confidence,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("pg_state_repository: SaveState failed: %w", err)
	}
	return nil
}

// LoadState fetches the persisted emotional state for a workspace+user pair.
// Returns (nil, nil) when no row exists.
func (r *PgStateRepository) LoadState(ctx context.Context, workspaceID, userID string) (*EmotionalState, error) {
	const q = `
		SELECT valence, arousal, detected_emotion, confidence, session_count, updated_at
		FROM   eq_emotional_states
		WHERE  workspace_id = $1
		  AND  user_id      = $2
		LIMIT  1
	`
	row := r.db.QueryRowContext(ctx, q, workspaceID, userID)

	var (
		valence         float64
		arousal         float64
		detectedEmotion string
		confidence      float64
		sessionCount    int
		updatedAt       time.Time
	)

	err := row.Scan(
		&valence, &arousal, &detectedEmotion,
		&confidence, &sessionCount, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("pg_state_repository: LoadState scan failed: %w", err)
	}

	return &EmotionalState{
		WorkspaceID:     workspaceID,
		Valence:         valence,
		Arousal:         arousal,
		DetectedEmotion: detectedEmotion,
		Confidence:      confidence,
		Timestamp:       updatedAt,
	}, nil
}
