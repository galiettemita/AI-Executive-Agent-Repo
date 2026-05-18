package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// KillSwitchRepository provides DB-backed kill switch operations.
type KillSwitchRepository interface {
	IsActive(ctx context.Context, workspaceID, userID string) (bool, error)
	Activate(ctx context.Context, workspaceID, userID, scope, activatedBy, reason string) error
	Deactivate(ctx context.Context, workspaceID, userID, deactivatedBy string) error
	List(ctx context.Context, workspaceID string) ([]KillSwitch, error)
	LogAction(ctx context.Context, workspaceID, userID, action, reason, performedBy string) error
}

// PgKillSwitchRepository implements KillSwitchRepository backed by pgx.
type PgKillSwitchRepository struct {
	q database.Querier
}

// NewPgKillSwitchRepository creates a new PgKillSwitchRepository.
func NewPgKillSwitchRepository(q database.Querier) *PgKillSwitchRepository {
	return &PgKillSwitchRepository{q: q}
}

// IsActive checks if a kill switch is active. Checks workspace-level first (any scope),
// then user-level. Kill switch evaluation precedes ALL other gates.
func (r *PgKillSwitchRepository) IsActive(ctx context.Context, workspaceID, userID string) (bool, error) {
	// Check workspace-level (global or workspace scope).
	var wsActive bool
	err := r.q.QueryRow(ctx,
		`SELECT EXISTS(
		   SELECT 1 FROM agent_kill_switches
		   WHERE workspace_id = $1::uuid AND is_active = true
		     AND scope IN ('workspace', 'global')
		 )`,
		workspaceID,
	).Scan(&wsActive)
	if err != nil {
		return false, fmt.Errorf("check workspace kill switch: %w", err)
	}
	if wsActive {
		return true, nil
	}

	// Check user-level.
	if userID != "" {
		var userActive bool
		err := r.q.QueryRow(ctx,
			`SELECT EXISTS(
			   SELECT 1 FROM agent_kill_switches
			   WHERE workspace_id = $1::uuid AND user_id = $2::uuid AND is_active = true
			 )`,
			workspaceID, userID,
		).Scan(&userActive)
		if err != nil {
			return false, fmt.Errorf("check user kill switch: %w", err)
		}
		return userActive, nil
	}

	return false, nil
}

// Activate activates a kill switch with the given scope.
func (r *PgKillSwitchRepository) Activate(ctx context.Context, workspaceID, userID, scope, activatedBy, reason string) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO agent_kill_switches (workspace_id, user_id, scope, is_active, reason, activated_by)
		 VALUES ($1::uuid, $2::uuid, $3::kill_switch_scope, true, $4, $5::uuid)
		 ON CONFLICT (workspace_id, user_id) DO UPDATE SET
		   is_active = true, scope = $3::kill_switch_scope, reason = $4,
		   activated_by = $5::uuid, deactivated_by = NULL, deactivated_at = NULL, updated_at = now()`,
		workspaceID, userID, scope, reason, activatedBy,
	)
	if err != nil {
		return fmt.Errorf("activate kill switch: %w", err)
	}
	return r.LogAction(ctx, workspaceID, userID, "activate", reason, activatedBy)
}

// Deactivate deactivates a kill switch.
func (r *PgKillSwitchRepository) Deactivate(ctx context.Context, workspaceID, userID, deactivatedBy string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE agent_kill_switches SET is_active = false, deactivated_by = $3::uuid, deactivated_at = now(), updated_at = now()
		 WHERE workspace_id = $1::uuid AND user_id = $2::uuid AND is_active = true`,
		workspaceID, userID, deactivatedBy,
	)
	if err != nil {
		return fmt.Errorf("deactivate kill switch: %w", err)
	}
	return r.LogAction(ctx, workspaceID, userID, "deactivate", "", deactivatedBy)
}

// List returns all kill switch records for a workspace.
func (r *PgKillSwitchRepository) List(ctx context.Context, workspaceID string) ([]KillSwitch, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, user_id, activated_by, reason, created_at, deactivated_at
		 FROM agent_kill_switches
		 WHERE workspace_id = $1::uuid
		 ORDER BY created_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list kill switches: %w", err)
	}
	defer rows.Close()

	var result []KillSwitch
	for rows.Next() {
		var ks KillSwitch
		var deactivatedAt *time.Time
		if err := rows.Scan(&ks.ID, &ks.WorkspaceID, &ks.UserID, &ks.ActivatedBy, &ks.Reason, &ks.ActivatedAt, &deactivatedAt); err != nil {
			return nil, fmt.Errorf("scan kill switch: %w", err)
		}
		ks.DeactivatedAt = deactivatedAt
		result = append(result, ks)
	}
	return result, rows.Err()
}

// LogAction writes an audit record to agent_kill_switch_log.
func (r *PgKillSwitchRepository) LogAction(ctx context.Context, workspaceID, userID, action, reason, performedBy string) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO agent_kill_switch_log (workspace_id, user_id, action, reason, performed_by)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)`,
		workspaceID, userID, action, reason, performedBy,
	)
	return err
}
