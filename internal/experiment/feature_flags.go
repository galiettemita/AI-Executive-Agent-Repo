package experiment

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FeatureFlag represents a boolean feature gate.
type FeatureFlag struct {
	Key         string
	Enabled     bool
	WorkspaceID string // empty = global
}

// FeatureFlagStore manages feature flags in Postgres.
type FeatureFlagStore struct {
	pool *pgxpool.Pool
}

func NewFeatureFlagStore(pool *pgxpool.Pool) *FeatureFlagStore {
	return &FeatureFlagStore{pool: pool}
}

// IsEnabled returns whether a flag is enabled for the given workspace.
func (s *FeatureFlagStore) IsEnabled(ctx context.Context, key, workspaceID string) (bool, error) {
	var enabled bool
	err := s.pool.QueryRow(ctx, `
		SELECT enabled FROM feature_flags
		WHERE key = $1 AND (workspace_id = $2::uuid OR workspace_id IS NULL)
		ORDER BY workspace_id DESC NULLS LAST
		LIMIT 1
	`, key, workspaceID).Scan(&enabled)
	if err != nil {
		return false, nil
	}
	return enabled, nil
}

// Set creates or updates a feature flag.
func (s *FeatureFlagStore) Set(ctx context.Context, flag FeatureFlag) error {
	var wsID interface{}
	if flag.WorkspaceID != "" {
		wsID = flag.WorkspaceID
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO feature_flags (key, enabled, workspace_id)
		VALUES ($1, $2, $3::uuid)
		ON CONFLICT (key, workspace_id) DO UPDATE SET enabled = EXCLUDED.enabled, updated_at = NOW()
	`, flag.Key, flag.Enabled, wsID)
	return err
}
