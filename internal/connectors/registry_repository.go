package connectors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brevio/brevio/internal/database"
)

// ConnectorRegistryRepository persists connectors and tools in PostgreSQL.
type ConnectorRegistryRepository interface {
	UpsertConnector(ctx context.Context, c Connector) error
	UpsertTool(ctx context.Context, t ConnectorTool) error
	ListAllTools(ctx context.Context) ([]ConnectorTool, error)
	ListAllConnectors(ctx context.Context) ([]Connector, error)
}

// PgConnectorRegistryRepository implements ConnectorRegistryRepository using pgx.
type PgConnectorRegistryRepository struct {
	db database.Querier
}

var _ ConnectorRegistryRepository = (*PgConnectorRegistryRepository)(nil)

func NewPgConnectorRegistryRepository(db database.Querier) *PgConnectorRegistryRepository {
	return &PgConnectorRegistryRepository{db: db}
}

func (r *PgConnectorRegistryRepository) UpsertConnector(ctx context.Context, c Connector) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO connectors (connector_key, domain, risk_level, data_class, mcp_server_url, status)
		VALUES ($1, $2, $3::risk_level, $4::data_class, $5, 'enabled')
		ON CONFLICT (connector_key) DO UPDATE SET
			domain = EXCLUDED.domain,
			risk_level = EXCLUDED.risk_level,
			data_class = EXCLUDED.data_class,
			mcp_server_url = EXCLUDED.mcp_server_url`,
		c.Key, c.Domain, c.RiskLevel, c.DataClass, c.MCPServerURL,
	)
	if err != nil {
		return fmt.Errorf("upsert connector %s: %w", c.Key, err)
	}
	return nil
}

func (r *PgConnectorRegistryRepository) UpsertTool(ctx context.Context, t ConnectorTool) error {
	inputJSON, err := json.Marshal(t.InputSchema)
	if err != nil {
		return fmt.Errorf("marshal input_schema for %s: %w", t.ToolKey, err)
	}
	outputJSON, err := json.Marshal(t.OutputSchema)
	if err != nil {
		return fmt.Errorf("marshal output_schema for %s: %w", t.ToolKey, err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO connector_tools (connector_id, tool_key, write_capable, reversible, autonomy_floor, input_schema, output_schema)
		VALUES (
			(SELECT id FROM connectors WHERE connector_key = $1),
			$2, $3, $4, $5::autonomy_level, $6::jsonb, $7::jsonb
		)
		ON CONFLICT (tool_key) DO UPDATE SET
			write_capable = EXCLUDED.write_capable,
			reversible = EXCLUDED.reversible,
			autonomy_floor = EXCLUDED.autonomy_floor,
			input_schema = EXCLUDED.input_schema,
			output_schema = EXCLUDED.output_schema`,
		t.ConnectorKey, t.ToolKey, t.Write, t.Reversible, t.AutonomyFloor,
		string(inputJSON), string(outputJSON),
	)
	if err != nil {
		return fmt.Errorf("upsert tool %s: %w", t.ToolKey, err)
	}
	return nil
}

func (r *PgConnectorRegistryRepository) ListAllTools(ctx context.Context) ([]ConnectorTool, error) {
	rows, err := r.db.Query(ctx, `
		SELECT c.connector_key, ct.tool_key, ct.write_capable, ct.reversible,
		       ct.autonomy_floor::text, ct.input_schema, ct.output_schema
		FROM connector_tools ct
		JOIN connectors c ON c.id = ct.connector_id
		ORDER BY ct.tool_key`)
	if err != nil {
		return nil, fmt.Errorf("list all tools: %w", err)
	}
	defer rows.Close()

	var tools []ConnectorTool
	for rows.Next() {
		var t ConnectorTool
		var inputJSON, outputJSON []byte
		if err := rows.Scan(&t.ConnectorKey, &t.ToolKey, &t.Write, &t.Reversible,
			&t.AutonomyFloor, &inputJSON, &outputJSON); err != nil {
			return nil, fmt.Errorf("scan tool row: %w", err)
		}
		if len(inputJSON) > 0 {
			_ = json.Unmarshal(inputJSON, &t.InputSchema)
		}
		if t.InputSchema == nil {
			t.InputSchema = map[string]interface{}{}
		}
		if len(outputJSON) > 0 {
			_ = json.Unmarshal(outputJSON, &t.OutputSchema)
		}
		if t.OutputSchema == nil {
			t.OutputSchema = map[string]interface{}{}
		}
		tools = append(tools, t)
	}
	return tools, rows.Err()
}

func (r *PgConnectorRegistryRepository) ListAllConnectors(ctx context.Context) ([]Connector, error) {
	rows, err := r.db.Query(ctx, `
		SELECT connector_key, domain, risk_level::text, data_class::text, COALESCE(mcp_server_url, '')
		FROM connectors
		WHERE status = 'enabled'
		ORDER BY connector_key`)
	if err != nil {
		return nil, fmt.Errorf("list all connectors: %w", err)
	}
	defer rows.Close()

	var connectors []Connector
	for rows.Next() {
		var c Connector
		if err := rows.Scan(&c.Key, &c.Domain, &c.RiskLevel, &c.DataClass, &c.MCPServerURL); err != nil {
			return nil, fmt.Errorf("scan connector row: %w", err)
		}
		connectors = append(connectors, c)
	}
	return connectors, rows.Err()
}
