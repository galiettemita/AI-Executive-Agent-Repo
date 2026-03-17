package proactive

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SnoozeStore manages per-workspace snooze preferences.
type SnoozeStore struct {
	pool *pgxpool.Pool
}

func NewSnoozeStore(pool *pgxpool.Pool) *SnoozeStore {
	return &SnoozeStore{pool: pool}
}

// IsSnoozed returns true if the workspace is globally snoozed or the specific signal type is snoozed.
func (s *SnoozeStore) IsSnoozed(ctx context.Context, workspaceID, signalType string) (bool, error) {
	if s.pool == nil {
		return false, nil
	}
	var snoozedUntil *time.Time
	var typeSnoozed []string
	err := s.pool.QueryRow(ctx, `
		SELECT snoozed_until, signal_type_snoozed FROM proactive_snooze_preferences
		WHERE workspace_id = $1
	`, workspaceID).Scan(&snoozedUntil, &typeSnoozed)
	if err != nil {
		return false, nil
	}
	if snoozedUntil != nil && time.Now().Before(*snoozedUntil) {
		return true, nil
	}
	if signalType != "" {
		for _, t := range typeSnoozed {
			if t == signalType {
				return true, nil
			}
		}
	}
	return false, nil
}

// Snooze sets a global snooze for the workspace.
func (s *SnoozeStore) Snooze(ctx context.Context, workspaceID string, until time.Time) error {
	if s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO proactive_snooze_preferences (workspace_id, snoozed_until)
		VALUES ($1, $2)
		ON CONFLICT (workspace_id) DO UPDATE SET snoozed_until = EXCLUDED.snoozed_until, updated_at = NOW()
	`, workspaceID, until)
	return err
}
