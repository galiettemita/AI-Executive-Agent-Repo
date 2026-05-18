package mcp

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
)

// PgToolRepository implements ToolRepository using PostgreSQL.
type PgToolRepository struct {
	db database.Querier
}

// NewPgToolRepository creates a new PgToolRepository.
func NewPgToolRepository(db database.Querier) *PgToolRepository {
	return &PgToolRepository{db: db}
}

func (r *PgToolRepository) RegisterTool(ctx context.Context, spec *ToolSpec) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO mcp_tools (tool_key, source, server_id, connector_key, auth_type, risk_level)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (tool_key) DO UPDATE SET
			source = EXCLUDED.source,
			server_id = EXCLUDED.server_id,
			connector_key = EXCLUDED.connector_key,
			auth_type = EXCLUDED.auth_type,
			risk_level = EXCLUDED.risk_level`,
		spec.ToolKey, spec.Source, spec.ServerID, spec.ConnectorKey, spec.AuthType, spec.RiskLevel)
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}
	return nil
}

func (r *PgToolRepository) GetTool(ctx context.Context, name string) (*ToolSpec, error) {
	row := r.db.QueryRow(ctx, `
		SELECT tool_key, source, server_id, connector_key, auth_type, risk_level
		FROM mcp_tools WHERE tool_key = $1`, name)

	spec := &ToolSpec{}
	err := row.Scan(&spec.ToolKey, &spec.Source, &spec.ServerID, &spec.ConnectorKey,
		&spec.AuthType, &spec.RiskLevel)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("get tool: %w", err)
	}
	return spec, nil
}

func (r *PgToolRepository) ListTools(ctx context.Context) ([]ToolSpec, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tool_key, source, server_id, connector_key, auth_type, risk_level
		FROM mcp_tools
		ORDER BY tool_key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	defer rows.Close()

	var tools []ToolSpec
	for rows.Next() {
		var spec ToolSpec
		err := rows.Scan(&spec.ToolKey, &spec.Source, &spec.ServerID, &spec.ConnectorKey,
			&spec.AuthType, &spec.RiskLevel)
		if err != nil {
			return nil, fmt.Errorf("scan tool: %w", err)
		}
		tools = append(tools, spec)
	}
	return tools, rows.Err()
}

func (r *PgToolRepository) RemoveTool(ctx context.Context, name string) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM mcp_tools WHERE tool_key = $1`, name)
	if err != nil {
		return fmt.Errorf("remove tool: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("tool not found: %s", name)
	}
	return nil
}

// Compile-time interface compliance check.
var _ ToolRepository = (*PgToolRepository)(nil)
