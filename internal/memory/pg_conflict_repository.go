package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// LessonConflictRow represents a persisted lesson conflict.
type LessonConflictRow struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	ExistingLessonID  string     `json:"existing_lesson_id"`
	IncomingLessonID  string     `json:"incoming_lesson_id"`
	ConflictType      string     `json:"conflict_type"`
	Resolution        string     `json:"resolution"`
	ResolvedBy        string     `json:"resolved_by"`
	ResolutionDetail  string     `json:"resolution_detail"`
	ResolvedAt        *time.Time `json:"resolved_at"`
	CreatedAt         time.Time  `json:"created_at"`
}

// ConflictRepository persists lesson conflict detection and resolution.
type ConflictRepository interface {
	RecordConflict(ctx context.Context, row LessonConflictRow) error
	ResolveConflict(ctx context.Context, conflictID, resolution, resolvedBy, detail string) error
	GetUnresolvedConflicts(ctx context.Context, workspaceID string, limit int) ([]LessonConflictRow, error)
	GetConflictHistory(ctx context.Context, workspaceID string, limit int) ([]LessonConflictRow, error)
}

// PgConflictRepository implements ConflictRepository backed by pgx.
type PgConflictRepository struct {
	q database.Querier
}

// NewPgConflictRepository creates a new PgConflictRepository.
func NewPgConflictRepository(q database.Querier) *PgConflictRepository {
	return &PgConflictRepository{q: q}
}

// RecordConflict persists a lesson conflict.
func (r *PgConflictRepository) RecordConflict(ctx context.Context, row LessonConflictRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO lesson_conflicts (workspace_id, existing_lesson_id, incoming_lesson_id, conflict_type, resolution)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5::lesson_conflict_resolution)`,
		row.WorkspaceID, row.ExistingLessonID, row.IncomingLessonID, row.ConflictType, row.Resolution,
	)
	if err != nil {
		return fmt.Errorf("record conflict: %w", err)
	}
	return nil
}

// ResolveConflict updates a conflict with resolution details.
func (r *PgConflictRepository) ResolveConflict(ctx context.Context, conflictID, resolution, resolvedBy, detail string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE lesson_conflicts SET resolution = $1::lesson_conflict_resolution, resolved_by = $2, resolution_detail = $3, resolved_at = now()
		 WHERE id = $4::uuid`,
		resolution, resolvedBy, detail, conflictID,
	)
	if err != nil {
		return fmt.Errorf("resolve conflict: %w", err)
	}
	return nil
}

// GetUnresolvedConflicts returns unresolved conflicts for a workspace.
func (r *PgConflictRepository) GetUnresolvedConflicts(ctx context.Context, workspaceID string, limit int) ([]LessonConflictRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, existing_lesson_id, COALESCE(incoming_lesson_id::text, ''), conflict_type, resolution,
		        COALESCE(resolved_by, ''), COALESCE(resolution_detail, ''), resolved_at, created_at
		 FROM lesson_conflicts
		 WHERE workspace_id = $1::uuid AND resolved_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get unresolved conflicts: %w", err)
	}
	defer rows.Close()

	var result []LessonConflictRow
	for rows.Next() {
		var c LessonConflictRow
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.ExistingLessonID, &c.IncomingLessonID,
			&c.ConflictType, &c.Resolution, &c.ResolvedBy, &c.ResolutionDetail, &c.ResolvedAt, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// GetConflictHistory returns all conflicts for a workspace.
func (r *PgConflictRepository) GetConflictHistory(ctx context.Context, workspaceID string, limit int) ([]LessonConflictRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, existing_lesson_id, COALESCE(incoming_lesson_id::text, ''), conflict_type, resolution,
		        COALESCE(resolved_by, ''), COALESCE(resolution_detail, ''), resolved_at, created_at
		 FROM lesson_conflicts
		 WHERE workspace_id = $1::uuid
		 ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get conflict history: %w", err)
	}
	defer rows.Close()

	var result []LessonConflictRow
	for rows.Next() {
		var c LessonConflictRow
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.ExistingLessonID, &c.IncomingLessonID,
			&c.ConflictType, &c.Resolution, &c.ResolvedBy, &c.ResolutionDetail, &c.ResolvedAt, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}
