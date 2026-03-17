package dpo

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists preference pairs and DPO rounds to Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// HashPrompt returns a SHA-256 hex string for dedup keying.
func HashPrompt(promptText string) string {
	h := sha256.Sum256([]byte(promptText))
	return fmt.Sprintf("%x", h)
}

// InsertPreferencePair stores a new (prompt, chosen, rejected) triple.
func (r *Repository) InsertPreferencePair(ctx context.Context, p PreferencePair) (PreferencePair, error) {
	if r.pool == nil {
		p.ID = uuid.New()
		p.CreatedAt = time.Now().UTC()
		return p, nil
	}

	var exists bool
	if err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM preference_pairs WHERE workspace_id=$1 AND prompt_hash=$2 AND workflow_run_id=$3)`,
		p.WorkspaceID, p.PromptHash, p.WorkflowRunID,
	).Scan(&exists); err != nil {
		return PreferencePair{}, fmt.Errorf("dpo.Repository.InsertPreferencePair dedup: %w", err)
	}
	if exists {
		return PreferencePair{}, ErrDuplicatePair
	}

	ctxJSON, _ := json.Marshal(p.CorrectionContext)
	p.ID = uuid.New()
	p.CreatedAt = time.Now().UTC()

	_, err := r.pool.Exec(ctx, `
		INSERT INTO preference_pairs
			(id, workspace_id, user_id, workflow_run_id, prompt_hash, prompt_text,
			 chosen_response, rejected_response, signal_type, correction_context,
			 quality_score_before, quality_score_after, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		p.ID, p.WorkspaceID, p.UserID, p.WorkflowRunID, p.PromptHash, p.PromptText,
		p.ChosenResponse, p.RejectedResponse, p.SignalType, ctxJSON,
		p.QualityScoreBefore, p.QualityScoreAfter, p.CreatedAt,
	)
	if err != nil {
		return PreferencePair{}, fmt.Errorf("dpo.Repository.InsertPreferencePair: %w", err)
	}
	return p, nil
}

// CountUnusedPairs returns unused pairs count for a workspace (nil = global).
func (r *Repository) CountUnusedPairs(ctx context.Context, workspaceID *uuid.UUID) (int, error) {
	if r.pool == nil {
		return 0, nil
	}
	q := `SELECT COUNT(*) FROM preference_pairs WHERE used_in_round IS NULL`
	args := []any{}
	if workspaceID != nil {
		q += ` AND workspace_id = $1`
		args = append(args, *workspaceID)
	}
	var count int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("dpo.Repository.CountUnusedPairs: %w", err)
	}
	return count, nil
}

// FetchUnusedPairs returns up to maxPairs unused pairs.
func (r *Repository) FetchUnusedPairs(ctx context.Context, workspaceID *uuid.UUID, maxPairs int) ([]PreferencePair, error) {
	if r.pool == nil {
		return nil, nil
	}
	q := `
		SELECT id, workspace_id, user_id, workflow_run_id, prompt_hash, prompt_text,
		       chosen_response, rejected_response, signal_type, correction_context,
		       quality_score_before, quality_score_after, used_in_round, created_at
		FROM preference_pairs WHERE used_in_round IS NULL`
	args := []any{}
	if workspaceID != nil {
		q += ` AND workspace_id = $1 ORDER BY created_at ASC LIMIT $2`
		args = append(args, *workspaceID, maxPairs)
	} else {
		q += ` ORDER BY created_at ASC LIMIT $1`
		args = append(args, maxPairs)
	}

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("dpo.Repository.FetchUnusedPairs: %w", err)
	}
	defer rows.Close()

	var pairs []PreferencePair
	for rows.Next() {
		var p PreferencePair
		var ctxJSON []byte
		if err := rows.Scan(
			&p.ID, &p.WorkspaceID, &p.UserID, &p.WorkflowRunID, &p.PromptHash, &p.PromptText,
			&p.ChosenResponse, &p.RejectedResponse, &p.SignalType, &ctxJSON,
			&p.QualityScoreBefore, &p.QualityScoreAfter, &p.UsedInRound, &p.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(ctxJSON, &p.CorrectionContext)
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

// MarkPairsUsed marks pairs as consumed in a DPO round.
func (r *Repository) MarkPairsUsed(ctx context.Context, pairIDs []uuid.UUID, roundNumber int) error {
	if r.pool == nil || len(pairIDs) == 0 {
		return nil
	}
	ids := make([]string, len(pairIDs))
	for i, id := range pairIDs {
		ids[i] = id.String()
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE preference_pairs SET used_in_round=$1 WHERE id = ANY($2::uuid[])`,
		roundNumber, ids,
	)
	return err
}

// InsertDPORound creates a new DPO round record.
func (r *Repository) InsertDPORound(ctx context.Context, round DPORound) (DPORound, error) {
	round.ID = uuid.New()
	round.CreatedAt = time.Now().UTC()
	round.UpdatedAt = round.CreatedAt
	if round.Status == "" {
		round.Status = "pending"
	}
	if r.pool == nil {
		return round, nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dpo_rounds
			(id, workspace_id, round_number, pair_count, base_model, fine_tune_job_id,
			 status, quality_score_baseline, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		round.ID, round.WorkspaceID, round.RoundNumber, round.PairCount,
		round.BaseModel, round.FineTuneJobID,
		round.Status, round.QualityScoreBaseline, round.CreatedAt, round.UpdatedAt,
	)
	if err != nil {
		return DPORound{}, fmt.Errorf("dpo.Repository.InsertDPORound: %w", err)
	}
	return round, nil
}

// UpdateDPORound updates status/checkpoint/quality on a round.
func (r *Repository) UpdateDPORound(ctx context.Context, id uuid.UUID, status string, checkpointID *string, qualityAfter *float64, errMsg *string) error {
	if r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE dpo_rounds
		SET status=$2, checkpoint_id=$3, quality_score_after=$4, error_message=$5, updated_at=NOW()
		WHERE id=$1`,
		id, status, checkpointID, qualityAfter, errMsg,
	)
	return err
}

// NextRoundNumber returns the next sequential round number.
func (r *Repository) NextRoundNumber(ctx context.Context, workspaceID *uuid.UUID) (int, error) {
	if r.pool == nil {
		return 1, nil
	}
	q := `SELECT COALESCE(MAX(round_number),0)+1 FROM dpo_rounds`
	args := []any{}
	if workspaceID != nil {
		q += ` WHERE workspace_id=$1`
		args = append(args, *workspaceID)
	}
	var n int
	return n, r.pool.QueryRow(ctx, q, args...).Scan(&n)
}
