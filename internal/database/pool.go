package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

type Pool struct {
	pool *pgxpool.Pool
}

type workspaceSessionSetter interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func NewPool(ctx context.Context, cfg Config) (*Pool, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	poolConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse pgx pool config: %w", err)
	}

	// PgBouncer session-mode compatible setting: avoid prepared statement cache.
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// Register pgvector types (vector, halfvec, sparsevec) on each new connection
	// so pgx can encode/decode pgvector.Vector natively without ::vector casts.
	poolConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open pgx pool: %w", err)
	}

	return &Pool{pool: pool}, nil
}

func (p *Pool) Close() {
	if p != nil && p.pool != nil {
		p.pool.Close()
	}
}

func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	workspaceID, err := WorkspaceIDFromContext(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	defer conn.Release()

	if err := setWorkspaceIDOnSession(ctx, conn, workspaceID); err != nil {
		return pgconn.CommandTag{}, err
	}

	return conn.Exec(ctx, sql, args...)
}

func setWorkspaceIDOnSession(ctx context.Context, setter workspaceSessionSetter, workspaceID uuid.UUID) error {
	if _, err := setter.Exec(ctx, "SET app.workspace_id = $1", workspaceID.String()); err != nil {
		return err
	}
	return nil
}
