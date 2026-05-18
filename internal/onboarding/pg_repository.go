package onboarding

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PgRepository implements Repository backed by PostgreSQL.
type PgRepository struct {
	db  database.Querier
	now func() time.Time
}

// NewPgRepository creates a production onboarding repository.
func NewPgRepository(db database.Querier) *PgRepository {
	return &PgRepository{
		db:  db,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (r *PgRepository) StartSession(ctx context.Context, workspaceID string) (*OnboardingSession, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	// Check for existing active session (optimistic: attempt insert, rely on UNIQUE constraint).
	id := uuid.Must(uuid.NewV7()).String()
	now := r.now()
	answersJSON, _ := json.Marshal(map[string]string{})

	_, err := r.db.Exec(ctx, `
		INSERT INTO onboarding_sessions (id, workspace_id, current_stage, completed_stages, skipped_stages, stage_answers, status, started_at, created_at)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, 'in_progress', $7, $7)
		ON CONFLICT (workspace_id) DO NOTHING`,
		id, workspaceID, StageWelcome, []string{}, []string{}, answersJSON, now)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}

	// Retrieve the session (may be existing if ON CONFLICT hit).
	return r.GetStatus(ctx, workspaceID)
}

func (r *PgRepository) AdvanceStage(ctx context.Context, sessionID string, answers map[string]string) error {
	// Read current state.
	session, err := r.getByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if !session.CompletedAt.IsZero() {
		return fmt.Errorf("session is already completed")
	}

	// Record answers for current stage.
	for k, v := range answers {
		session.StageAnswers[session.CurrentStage+"."+k] = v
	}
	session.CompletedStages = append(session.CompletedStages, session.CurrentStage)

	nextStage := nextStageName(session.CurrentStage)
	var completedAt *time.Time
	if nextStage == "" {
		now := r.now()
		completedAt = &now
		session.CurrentStage = ""
	} else {
		session.CurrentStage = nextStage
	}

	answersJSON, _ := json.Marshal(session.StageAnswers)
	status := "in_progress"
	if completedAt != nil {
		status = "completed"
	}

	_, err = r.db.Exec(ctx, `
		UPDATE onboarding_sessions
		SET current_stage = $1, completed_stages = $2, stage_answers = $3, status = $4, completed_at = $5
		WHERE id = $6::uuid`,
		session.CurrentStage, session.CompletedStages, answersJSON, status, completedAt, sessionID)
	if err != nil {
		return fmt.Errorf("advance stage: %w", err)
	}
	return nil
}

func (r *PgRepository) SkipStage(ctx context.Context, sessionID string) error {
	session, err := r.getByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if !session.CompletedAt.IsZero() {
		return fmt.Errorf("session is already completed")
	}

	session.SkippedStages = append(session.SkippedStages, session.CurrentStage)
	nextStage := nextStageName(session.CurrentStage)
	var completedAt *time.Time
	if nextStage == "" {
		now := r.now()
		completedAt = &now
		session.CurrentStage = ""
	} else {
		session.CurrentStage = nextStage
	}

	status := "in_progress"
	if completedAt != nil {
		status = "completed"
	}

	_, err = r.db.Exec(ctx, `
		UPDATE onboarding_sessions
		SET current_stage = $1, skipped_stages = $2, status = $3, completed_at = $4
		WHERE id = $5::uuid`,
		session.CurrentStage, session.SkippedStages, status, completedAt, sessionID)
	if err != nil {
		return fmt.Errorf("skip stage: %w", err)
	}
	return nil
}

func (r *PgRepository) GetStatus(ctx context.Context, workspaceID string) (*OnboardingSession, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, current_stage, completed_stages, skipped_stages, stage_answers, started_at, COALESCE(completed_at, '0001-01-01'::timestamptz)
		FROM onboarding_sessions
		WHERE workspace_id = $1::uuid
		ORDER BY created_at DESC LIMIT 1`, workspaceID)

	return r.scanSession(row)
}

func (r *PgRepository) IsComplete(ctx context.Context, sessionID string) (bool, error) {
	session, err := r.getByID(ctx, sessionID)
	if err != nil {
		return false, err
	}
	return !session.CompletedAt.IsZero(), nil
}

func (r *PgRepository) getByID(ctx context.Context, sessionID string) (*OnboardingSession, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, current_stage, completed_stages, skipped_stages, stage_answers, started_at, COALESCE(completed_at, '0001-01-01'::timestamptz)
		FROM onboarding_sessions
		WHERE id = $1::uuid`, sessionID)

	return r.scanSession(row)
}

func (r *PgRepository) scanSession(row pgx.Row) (*OnboardingSession, error) {
	var s OnboardingSession
	var answersJSON []byte
	err := row.Scan(&s.ID, &s.WorkspaceID, &s.CurrentStage, &s.CompletedStages, &s.SkippedStages, &answersJSON, &s.StartedAt, &s.CompletedAt)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	s.StageAnswers = map[string]string{}
	if len(answersJSON) > 0 {
		_ = json.Unmarshal(answersJSON, &s.StageAnswers)
	}
	// Normalize zero time.
	if s.CompletedAt.Year() <= 1 {
		s.CompletedAt = time.Time{}
	}
	return &s, nil
}
