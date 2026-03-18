package learning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrAnchorProtected is returned when attempting to modify an anchored lesson.
var ErrAnchorProtected = fmt.Errorf("lesson is anchored and protected from modification")

// LessonSignal represents a new signal that would update a lesson.
type LessonSignal struct {
	WorkspaceID string
	LessonID    string
	NewTitle    string
	NewStatus   string
}

// PreferenceTransferReader abstracts cross-workspace transfer tracking.
type PreferenceTransferReader interface {
	GetWorkspaceTransferCount(ctx context.Context, lessonID uuid.UUID) (int, error)
}

// LessonAnchorManager handles EWC-style lesson anchoring for continual learning.
type LessonAnchorManager struct {
	db             *pgxpool.Pool
	transferReader PreferenceTransferReader
	logger         *slog.Logger
}

// NewLessonAnchorManager creates a lesson anchor manager.
func NewLessonAnchorManager(db *pgxpool.Pool, transferReader PreferenceTransferReader, logger *slog.Logger) *LessonAnchorManager {
	return &LessonAnchorManager{
		db:             db,
		transferReader: transferReader,
		logger:         logger,
	}
}

// PromoteToAnchor marks a lesson as anchored with weight 1.0.
func (m *LessonAnchorManager) PromoteToAnchor(ctx context.Context, lessonID uuid.UUID) error {
	if m.db == nil {
		return nil
	}

	_, err := m.db.Exec(ctx,
		`UPDATE learned_lessons SET is_anchor=true, anchored_at=NOW(), anchor_weight=1.0
		 WHERE id=$1`,
		lessonID,
	)
	if err != nil {
		return fmt.Errorf("promote to anchor: %w", err)
	}

	m.logger.Info("lesson_promoted_to_anchor", "lesson_id", lessonID)
	return nil
}

// EvaluateAnchorEligibility returns lessons eligible for anchor promotion.
// Criteria: confidence > 0.9, reuse_count > 10, not already anchored.
func (m *LessonAnchorManager) EvaluateAnchorEligibility(ctx context.Context) ([]uuid.UUID, error) {
	if m.db == nil {
		return nil, nil
	}

	rows, err := m.db.Query(ctx,
		`SELECT id FROM learned_lessons
		 WHERE confidence > 0.9 AND reuse_count > 10 AND is_anchor = FALSE`,
	)
	if err != nil {
		return nil, fmt.Errorf("evaluate eligibility: %w", err)
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}

// RunAnchorPromotion promotes all eligible lessons to anchors.
func (m *LessonAnchorManager) RunAnchorPromotion(ctx context.Context) error {
	eligible, err := m.EvaluateAnchorEligibility(ctx)
	if err != nil {
		return err
	}

	for _, id := range eligible {
		if promErr := m.PromoteToAnchor(ctx, id); promErr != nil {
			m.logger.Error("anchor_promotion_error", "lesson_id", id, "error", promErr)
		}
	}

	m.logger.Info("anchor_promotion_complete", "promoted_count", len(eligible))
	return nil
}

// ProtectAnchor checks if a lesson is an anchor. If so, rejects the modification.
// This is the EWC-equivalent guard — high-confidence anchors cannot be overwritten.
func (m *LessonAnchorManager) ProtectAnchor(ctx context.Context, lessonID uuid.UUID) (bool, error) {
	if m.db == nil {
		return true, nil // allow in test mode
	}

	var isAnchor bool
	err := m.db.QueryRow(ctx,
		`SELECT COALESCE(is_anchor, false) FROM learned_lessons WHERE id=$1`,
		lessonID,
	).Scan(&isAnchor)
	if err != nil {
		return true, nil // lesson not found = allow
	}

	if isAnchor {
		m.logger.Info("anchor_protected", "lesson_id", lessonID)
		return false, ErrAnchorProtected
	}

	return true, nil
}

// ElevateForCrossWorkspace increases anchor_weight for lessons adopted by multiple workspaces.
func (m *LessonAnchorManager) ElevateForCrossWorkspace(ctx context.Context) error {
	if m.db == nil || m.transferReader == nil {
		return nil
	}

	rows, err := m.db.Query(ctx,
		`SELECT id FROM learned_lessons WHERE is_anchor = TRUE`,
	)
	if err != nil {
		return fmt.Errorf("query anchored lessons: %w", err)
	}
	defer rows.Close()

	elevated := 0
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			continue
		}

		count, tErr := m.transferReader.GetWorkspaceTransferCount(ctx, id)
		if tErr != nil {
			continue
		}

		if count > 5 {
			_, _ = m.db.Exec(ctx,
				`UPDATE learned_lessons SET anchor_weight=2.0, workspace_adoption_count=$1 WHERE id=$2`,
				count, id)
			elevated++
		} else {
			_, _ = m.db.Exec(ctx,
				`UPDATE learned_lessons SET workspace_adoption_count=$1 WHERE id=$2`,
				count, id)
		}
	}

	m.logger.Info("cross_workspace_elevation_complete", "elevated_count", elevated)
	return nil
}

// RecordUsage records that an anchored lesson was used in a prompt injection.
func (m *LessonAnchorManager) RecordUsage(ctx context.Context, lessonID uuid.UUID, workspaceID uuid.UUID, requestID *uuid.UUID) error {
	if m.db == nil {
		return nil
	}

	_, err := m.db.Exec(ctx,
		`INSERT INTO lesson_usages (lesson_id, workspace_id, request_id) VALUES ($1, $2, $3)`,
		lessonID, workspaceID, requestID,
	)
	return err
}
