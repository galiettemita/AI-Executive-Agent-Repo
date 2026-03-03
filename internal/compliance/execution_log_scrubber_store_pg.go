package compliance

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGExecutionLogPIIScrubStore struct {
	pool *pgxpool.Pool
}

func NewPGExecutionLogPIIScrubStore(ctx context.Context, dsn string) (*PGExecutionLogPIIScrubStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("database dsn is required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pgx pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping execution log scrub store: %w", err)
	}
	return &PGExecutionLogPIIScrubStore{pool: pool}, nil
}

func (s *PGExecutionLogPIIScrubStore) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *PGExecutionLogPIIScrubStore) ListExecutionLogsOlderThan(ctx context.Context, before time.Time, limit int) ([]ExecutionLogRecord, error) {
	if s == nil || s.pool == nil {
		return nil, fmt.Errorf("store is not initialized")
	}
	rows, err := s.pool.Query(ctx, `
SELECT
  id::text,
  created_at,
  COALESCE(input_payload::text, ''),
  COALESCE(output_payload::text, '')
FROM skills.execution_log
WHERE created_at < $1
ORDER BY created_at ASC
LIMIT $2
`, before.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list execution logs older than: %w", err)
	}
	defer rows.Close()

	out := make([]ExecutionLogRecord, 0, limit)
	for rows.Next() {
		var row ExecutionLogRecord
		if err := rows.Scan(&row.ID, &row.CreatedAt, &row.InputPayload, &row.OutputPayload); err != nil {
			return nil, fmt.Errorf("scan execution log row: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate execution log rows: %w", err)
	}
	return out, nil
}

func (s *PGExecutionLogPIIScrubStore) NullifyExecutionLogPayloads(ctx context.Context, ids []string, _ string, _ time.Time) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store is not initialized")
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
UPDATE skills.execution_log
SET input_payload = NULL,
    output_payload = NULL
WHERE id::text = ANY($1::text[])
`, ids)
	if err != nil {
		return fmt.Errorf("nullify execution log payloads: %w", err)
	}
	return nil
}
