package trust

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// AutonomyLevelRow represents the current autonomy level for a workspace/domain.
type AutonomyLevelRow struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Domain          string     `json:"domain"`
	CurrentLevel    int        `json:"current_level"`
	TrustScore      float64    `json:"trust_score"`
	LastEvaluatedAt *time.Time `json:"last_evaluated_at"`
}

// DemotionEventRow represents a persisted demotion event.
type DemotionEventRow struct {
	ID                    string    `json:"id"`
	WorkspaceID           string    `json:"workspace_id"`
	Domain                string    `json:"domain"`
	PreviousLevel         int       `json:"previous_level"`
	NewLevel              int       `json:"new_level"`
	Trigger               string    `json:"trigger"`
	Reason                string    `json:"reason"`
	TrustScoreAtDemotion  *float64  `json:"trust_score_at_demotion"`
	FailureCountAtDemotion *int     `json:"failure_count_at_demotion"`
	DemotedAt             time.Time `json:"demoted_at"`
}

// DemotionRepository provides DB-backed autonomy demotion operations.
type DemotionRepository interface {
	GetAutonomyLevel(ctx context.Context, workspaceID, domain string) (*AutonomyLevelRow, error)
	UpsertAutonomyLevel(ctx context.Context, workspaceID, domain string, level int, trustScore float64) error
	RecordDemotion(ctx context.Context, event DemotionEventRow) error
	GetDemotionHistory(ctx context.Context, workspaceID string, limit int) ([]DemotionEventRow, error)
	CountDemotions90d(ctx context.Context, workspaceID string) (int, error)
}

// PgDemotionRepository implements DemotionRepository backed by pgx.
type PgDemotionRepository struct {
	q database.Querier
}

// NewPgDemotionRepository creates a new PgDemotionRepository.
func NewPgDemotionRepository(q database.Querier) *PgDemotionRepository {
	return &PgDemotionRepository{q: q}
}

// GetAutonomyLevel retrieves the current autonomy level for a workspace/domain.
func (r *PgDemotionRepository) GetAutonomyLevel(ctx context.Context, workspaceID, domain string) (*AutonomyLevelRow, error) {
	var row AutonomyLevelRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, domain, current_level, trust_score, last_evaluated_at
		 FROM autonomy_levels
		 WHERE workspace_id = $1::uuid AND domain = $2`,
		workspaceID, domain,
	).Scan(&row.ID, &row.WorkspaceID, &row.Domain, &row.CurrentLevel, &row.TrustScore, &row.LastEvaluatedAt)
	if err != nil {
		return nil, fmt.Errorf("get autonomy level: %w", err)
	}
	return &row, nil
}

// UpsertAutonomyLevel creates or updates the autonomy level for a workspace/domain.
func (r *PgDemotionRepository) UpsertAutonomyLevel(ctx context.Context, workspaceID, domain string, level int, trustScore float64) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO autonomy_levels (workspace_id, domain, current_level, trust_score, last_evaluated_at)
		 VALUES ($1::uuid, $2, $3, $4, now())
		 ON CONFLICT (workspace_id, domain) DO UPDATE SET
		   current_level = EXCLUDED.current_level,
		   trust_score = EXCLUDED.trust_score,
		   last_evaluated_at = now(),
		   updated_at = now()`,
		workspaceID, domain, level, trustScore,
	)
	if err != nil {
		return fmt.Errorf("upsert autonomy level: %w", err)
	}
	return nil
}

// RecordDemotion persists a demotion event.
func (r *PgDemotionRepository) RecordDemotion(ctx context.Context, event DemotionEventRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO autonomy_demotion_events
		   (workspace_id, domain, previous_level, new_level, trigger, reason, trust_score_at_demotion, failure_count_at_demotion)
		 VALUES ($1::uuid, $2, $3, $4, $5::demotion_trigger, $6, $7, $8)`,
		event.WorkspaceID, event.Domain, event.PreviousLevel, event.NewLevel,
		event.Trigger, event.Reason, event.TrustScoreAtDemotion, event.FailureCountAtDemotion,
	)
	if err != nil {
		return fmt.Errorf("record demotion: %w", err)
	}
	return nil
}

// GetDemotionHistory returns recent demotion events for a workspace.
func (r *PgDemotionRepository) GetDemotionHistory(ctx context.Context, workspaceID string, limit int) ([]DemotionEventRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, domain, previous_level, new_level, trigger, reason,
		        trust_score_at_demotion, failure_count_at_demotion, demoted_at
		 FROM autonomy_demotion_events
		 WHERE workspace_id = $1::uuid
		 ORDER BY demoted_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get demotion history: %w", err)
	}
	defer rows.Close()

	var result []DemotionEventRow
	for rows.Next() {
		var e DemotionEventRow
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.Domain, &e.PreviousLevel, &e.NewLevel,
			&e.Trigger, &e.Reason, &e.TrustScoreAtDemotion, &e.FailureCountAtDemotion, &e.DemotedAt); err != nil {
			return nil, fmt.Errorf("scan demotion event: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// CountDemotions90d returns the number of demotions in the last 90 days.
func (r *PgDemotionRepository) CountDemotions90d(ctx context.Context, workspaceID string) (int, error) {
	var count int
	err := r.q.QueryRow(ctx,
		`SELECT COUNT(*) FROM autonomy_demotion_events
		 WHERE workspace_id = $1::uuid AND demoted_at > now() - interval '90 days'`,
		workspaceID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count demotions 90d: %w", err)
	}
	return count, nil
}
