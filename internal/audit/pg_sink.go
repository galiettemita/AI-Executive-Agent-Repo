package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGSink struct {
	pool *pgxpool.Pool
}

func NewPGSink(ctx context.Context, dsn string) (*PGSink, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open audit pg sink pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping audit pg sink: %w", err)
	}
	return &PGSink{pool: pool}, nil
}

func (s *PGSink) PersistMutation(ctx context.Context, entry MutationEntry) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("pg sink is not initialized")
	}

	entryID, err := uuid.Parse(strings.TrimSpace(entry.ID))
	if err != nil {
		return fmt.Errorf("parse entry id: %w", err)
	}
	workspaceID, err := uuid.Parse(strings.TrimSpace(entry.WorkspaceID))
	if err != nil {
		return fmt.Errorf("parse workspace_id: %w", err)
	}

	var actorUserID any
	if parsed, parseErr := uuid.Parse(strings.TrimSpace(entry.Actor)); parseErr == nil {
		actorUserID = parsed
	}

	eventJSON, err := json.Marshal(map[string]any{
		"action":    entry.Action,
		"resource":  entry.Resource,
		"before":    entry.Before,
		"after":     entry.After,
		"timestamp": entry.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("marshal event_json: %w", err)
	}

	createdAt := time.Now().UTC()
	if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(entry.Timestamp)); parseErr == nil {
		createdAt = parsed.UTC()
	}

	var prevHash any
	if strings.TrimSpace(entry.PrevHash) != "" {
		prevHash = entry.PrevHash
	}

	_, err = s.pool.Exec(ctx, `
INSERT INTO audit_log_entries (
  id,
  workspace_id,
  actor_user_id,
  event_type,
  event_json,
  previous_hash,
  event_hash,
  created_at
) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)
`, entryID, workspaceID, actorUserID, entry.Action, string(eventJSON), prevHash, entry.Hash, createdAt)
	if err != nil {
		return fmt.Errorf("insert audit log entry: %w", err)
	}
	return nil
}

func (s *PGSink) Close() error {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
	return nil
}
