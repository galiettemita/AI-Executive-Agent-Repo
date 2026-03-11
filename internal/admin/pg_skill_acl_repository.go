package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// SkillACLRepository provides DB-backed skill ACL override operations.
type SkillACLRepository interface {
	IsSkillAllowed(ctx context.Context, workspaceID, userID, skillID string) (allowed bool, hasOverride bool, err error)
	SetOverride(ctx context.Context, workspaceID, userID, skillID string, allowed bool, reason, setBy string, expiresAt *time.Time) error
	RemoveOverride(ctx context.Context, workspaceID, userID, skillID string) error
	GetUserOverrides(ctx context.Context, workspaceID, userID string) ([]SkillACLOverride, error)
}

// PgSkillACLRepository implements SkillACLRepository backed by pgx.
type PgSkillACLRepository struct {
	q database.Querier
}

// NewPgSkillACLRepository creates a new PgSkillACLRepository.
func NewPgSkillACLRepository(q database.Querier) *PgSkillACLRepository {
	return &PgSkillACLRepository{q: q}
}

// IsSkillAllowed checks if a skill is allowed for a user. Returns (true, false, nil)
// when no override exists (default allow). Expired overrides are treated as absent.
func (r *PgSkillACLRepository) IsSkillAllowed(ctx context.Context, workspaceID, userID, skillID string) (bool, bool, error) {
	var isAllowed bool
	err := r.q.QueryRow(ctx,
		`SELECT is_allowed FROM skill_acl_overrides
		 WHERE workspace_id = $1::uuid AND user_id = $2::uuid AND skill_id = $3
		   AND (expires_at IS NULL OR expires_at > now())`,
		workspaceID, userID, skillID,
	).Scan(&isAllowed)
	if err != nil {
		// No row = no override = default allow.
		return true, false, nil
	}
	return isAllowed, true, nil
}

// SetOverride creates or updates a skill ACL override.
func (r *PgSkillACLRepository) SetOverride(ctx context.Context, workspaceID, userID, skillID string, allowed bool, reason, setBy string, expiresAt *time.Time) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO skill_acl_overrides (workspace_id, user_id, skill_id, is_allowed, reason, set_by, expires_at)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::uuid, $7)
		 ON CONFLICT (workspace_id, user_id, skill_id) DO UPDATE SET
		   is_allowed = EXCLUDED.is_allowed, reason = EXCLUDED.reason,
		   set_by = EXCLUDED.set_by, expires_at = EXCLUDED.expires_at, updated_at = now()`,
		workspaceID, userID, skillID, allowed, reason, setBy, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("set skill acl override: %w", err)
	}
	return nil
}

// RemoveOverride removes a skill ACL override.
func (r *PgSkillACLRepository) RemoveOverride(ctx context.Context, workspaceID, userID, skillID string) error {
	_, err := r.q.Exec(ctx,
		`DELETE FROM skill_acl_overrides
		 WHERE workspace_id = $1::uuid AND user_id = $2::uuid AND skill_id = $3`,
		workspaceID, userID, skillID,
	)
	return err
}

// GetUserOverrides returns all overrides for a user in a workspace.
func (r *PgSkillACLRepository) GetUserOverrides(ctx context.Context, workspaceID, userID string) ([]SkillACLOverride, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, user_id, skill_id, is_allowed, reason, created_at
		 FROM skill_acl_overrides
		 WHERE workspace_id = $1::uuid AND user_id = $2::uuid
		   AND (expires_at IS NULL OR expires_at > now())
		 ORDER BY skill_id`,
		workspaceID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get user overrides: %w", err)
	}
	defer rows.Close()

	var result []SkillACLOverride
	for rows.Next() {
		var o SkillACLOverride
		if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.UserID, &o.SkillID, &o.Allowed, &o.Reason, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan skill acl override: %w", err)
		}
		result = append(result, o)
	}
	return result, rows.Err()
}
