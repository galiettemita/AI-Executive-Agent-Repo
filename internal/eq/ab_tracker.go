package eq

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EQABTracker records per-request ORM quality scores for A/B comparison
// between EQ-enabled (treatment) and EQ-disabled (control) request paths.
type EQABTracker struct {
	pool *pgxpool.Pool
}

// NewEQABTracker creates a tracker. Returns error if pool is nil.
func NewEQABTracker(pool *pgxpool.Pool) (*EQABTracker, error) {
	if pool == nil {
		return nil, fmt.Errorf("eq.NewEQABTracker: pool must not be nil")
	}
	return &EQABTracker{pool: pool}, nil
}

// RecordResponseQuality inserts one A/B data point.
func (t *EQABTracker) RecordResponseQuality(
	ctx context.Context,
	workspaceID uuid.UUID,
	requestID string,
	ormScore float64,
	eqEnabled bool,
) error {
	_, err := t.pool.Exec(ctx,
		`INSERT INTO eq_ab_results (workspace_id, request_id, orm_score, eq_enabled)
		 VALUES ($1, $2, $3, $4)`,
		workspaceID, requestID, ormScore, eqEnabled,
	)
	if err != nil {
		return fmt.Errorf("eq.EQABTracker.RecordResponseQuality: %w", err)
	}
	return nil
}

// EQABSummary holds the A/B comparison result.
type EQABSummary struct {
	WorkspaceID    uuid.UUID
	ControlAvg     float64 // mean ORM score when eq_enabled=false
	TreatmentAvg   float64 // mean ORM score when eq_enabled=true
	ControlCount   int
	TreatmentCount int
	ImprovementPct float64 // (TreatmentAvg - ControlAvg) / ControlAvg * 100
	ShouldPromote  bool    // true if ImprovementPct >= 5.0
}

// GetABSummary returns the A/B summary for a workspace.
// Requires at least 10 samples in each arm to be valid.
func (t *EQABTracker) GetABSummary(ctx context.Context, workspaceID uuid.UUID) (*EQABSummary, error) {
	rows, err := t.pool.Query(ctx,
		`SELECT eq_enabled, avg(orm_score)::float8, count(*) FROM eq_ab_results
		  WHERE workspace_id = $1
		  GROUP BY eq_enabled`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("eq.GetABSummary: query: %w", err)
	}
	defer rows.Close()

	summary := &EQABSummary{WorkspaceID: workspaceID}
	for rows.Next() {
		var enabled bool
		var avg float64
		var count int
		if err := rows.Scan(&enabled, &avg, &count); err != nil {
			return nil, fmt.Errorf("eq.GetABSummary: scan: %w", err)
		}
		if enabled {
			summary.TreatmentAvg = avg
			summary.TreatmentCount = count
		} else {
			summary.ControlAvg = avg
			summary.ControlCount = count
		}
	}

	const minSamples = 10
	if summary.ControlCount < minSamples || summary.TreatmentCount < minSamples {
		return nil, fmt.Errorf("eq.GetABSummary: insufficient samples (control=%d treatment=%d, need %d each)",
			summary.ControlCount, summary.TreatmentCount, minSamples)
	}

	if summary.ControlAvg > 0 {
		summary.ImprovementPct = (summary.TreatmentAvg - summary.ControlAvg) / summary.ControlAvg * 100.0
	}
	summary.ShouldPromote = summary.ImprovementPct >= 5.0
	return summary, nil
}
