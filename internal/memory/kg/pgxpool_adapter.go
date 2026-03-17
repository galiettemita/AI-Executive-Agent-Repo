package kg

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PgxPoolAdapter wraps a *pgxpool.Pool to satisfy the RepoDB interface.
type PgxPoolAdapter struct {
	pool *pgxpool.Pool
}

func NewPgxPoolAdapter(pool *pgxpool.Pool) *PgxPoolAdapter {
	return &PgxPoolAdapter{pool: pool}
}

func (a *PgxPoolAdapter) ExecContext(ctx context.Context, query string, args ...any) error {
	_, err := a.pool.Exec(ctx, query, args...)
	return err
}

func (a *PgxPoolAdapter) QueryContext(ctx context.Context, query string, args ...any) (RepoRows, error) {
	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("pgxpool_adapter: QueryContext: %w", err)
	}
	return &pgxRowsAdapter{rows: rows}, nil
}

type pgxRowsAdapter struct {
	rows pgx.Rows
}

func (r *pgxRowsAdapter) Next() bool          { return r.rows.Next() }
func (r *pgxRowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }
func (r *pgxRowsAdapter) Close() error         { r.rows.Close(); return nil }
func (r *pgxRowsAdapter) Err() error           { return r.rows.Err() }

var _ RepoDB = (*PgxPoolAdapter)(nil)
var _ RepoRows = (*pgxRowsAdapter)(nil)
