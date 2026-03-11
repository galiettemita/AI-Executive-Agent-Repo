package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// DecayLogRow represents a persisted decay sweep result.
type DecayLogRow struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	DecayFunction string    `json:"decay_function"`
	HalfLifeDays  float64   `json:"half_life_days"`
	ItemsDecayed  int       `json:"items_decayed"`
	ItemsPurged   int       `json:"items_purged"`
	MinWeight     float64   `json:"min_weight"`
	SweptAt       time.Time `json:"swept_at"`
}

// DecayRepository persists memory decay sweep results.
type DecayRepository interface {
	RecordDecaySweep(ctx context.Context, row DecayLogRow) error
	GetDecaySweeps(ctx context.Context, workspaceID string, limit int) ([]DecayLogRow, error)
	ApplyDecayAndPersist(ctx context.Context, workspaceID string, config DecayConfig) (DecayLogRow, error)
	PurgeDecayedAndPersist(ctx context.Context, workspaceID string, threshold float64, config DecayConfig) (DecayLogRow, error)
}

// PgDecayRepository implements DecayRepository backed by pgx.
type PgDecayRepository struct {
	q database.Querier
}

// NewPgDecayRepository creates a new PgDecayRepository.
func NewPgDecayRepository(q database.Querier) *PgDecayRepository {
	return &PgDecayRepository{q: q}
}

// RecordDecaySweep persists a decay sweep result.
func (r *PgDecayRepository) RecordDecaySweep(ctx context.Context, row DecayLogRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO memory_decay_log (workspace_id, decay_function, half_life_days, items_decayed, items_purged, min_weight, swept_at)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
		row.WorkspaceID, row.DecayFunction, row.HalfLifeDays, row.ItemsDecayed, row.ItemsPurged, row.MinWeight, row.SweptAt,
	)
	if err != nil {
		return fmt.Errorf("record decay sweep: %w", err)
	}
	return nil
}

// GetDecaySweeps returns recent decay sweeps for a workspace.
func (r *PgDecayRepository) GetDecaySweeps(ctx context.Context, workspaceID string, limit int) ([]DecayLogRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, decay_function, half_life_days, items_decayed, items_purged, min_weight, swept_at
		 FROM memory_decay_log
		 WHERE workspace_id = $1::uuid
		 ORDER BY swept_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get decay sweeps: %w", err)
	}
	defer rows.Close()

	var result []DecayLogRow
	for rows.Next() {
		var d DecayLogRow
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.DecayFunction, &d.HalfLifeDays,
			&d.ItemsDecayed, &d.ItemsPurged, &d.MinWeight, &d.SweptAt); err != nil {
			return nil, fmt.Errorf("scan decay log: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// ApplyDecayAndPersist runs in-memory decay computation, persists the result to DB.
func (r *PgDecayRepository) ApplyDecayAndPersist(ctx context.Context, workspaceID string, config DecayConfig) (DecayLogRow, error) {
	svc := NewMemoryDecayService()
	decayed, err := svc.ApplyDecay(workspaceID, config)
	if err != nil {
		return DecayLogRow{}, fmt.Errorf("apply decay: %w", err)
	}

	row := DecayLogRow{
		WorkspaceID:   workspaceID,
		DecayFunction: config.DecayFunction,
		HalfLifeDays:  config.HalfLifeDays,
		ItemsDecayed:  decayed,
		MinWeight:     config.MinWeight,
		SweptAt:       time.Now().UTC(),
	}
	if err := r.RecordDecaySweep(ctx, row); err != nil {
		return DecayLogRow{}, err
	}
	return row, nil
}

// PurgeDecayedAndPersist runs purge and persists the sweep result.
func (r *PgDecayRepository) PurgeDecayedAndPersist(ctx context.Context, workspaceID string, threshold float64, config DecayConfig) (DecayLogRow, error) {
	svc := NewMemoryDecayService()
	purged, err := svc.PurgeDecayed(workspaceID, threshold)
	if err != nil {
		return DecayLogRow{}, fmt.Errorf("purge decayed: %w", err)
	}

	row := DecayLogRow{
		WorkspaceID:   workspaceID,
		DecayFunction: config.DecayFunction,
		HalfLifeDays:  config.HalfLifeDays,
		ItemsPurged:   purged,
		MinWeight:     config.MinWeight,
		SweptAt:       time.Now().UTC(),
	}
	if err := r.RecordDecaySweep(ctx, row); err != nil {
		return DecayLogRow{}, err
	}
	return row, nil
}
