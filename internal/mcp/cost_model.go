package mcp

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CostRate defines the billing rate for a single tool.
type CostRate struct {
	FixedMicroCentsPerCall      int64 // micro-cents (1/1,000,000 of a cent)
	VariableMicroCentsPerKBResp int64 // micro-cents per KB of response bytes
}

// ToolCostConfig maps tool keys to their cost rates.
type ToolCostConfig struct {
	Rates map[string]CostRate
}

// NewToolCostConfig creates a cost config with default rates.
func NewToolCostConfig() *ToolCostConfig {
	return &ToolCostConfig{Rates: map[string]CostRate{}}
}

// ComputeCost returns the total micro-cents for a tool invocation.
func (c *ToolCostConfig) ComputeCost(toolKey string, responseBytes int64) int64 {
	rate, ok := c.Rates[toolKey]
	if !ok {
		return 0 // unknown tool — free tier
	}
	variableCost := (responseBytes / 1024) * rate.VariableMicroCentsPerKBResp
	return rate.FixedMicroCentsPerCall + variableCost
}

// ToolCostRecorder records tool costs to storage.
type ToolCostRecorder struct {
	config *ToolCostConfig
	pool   *pgxpool.Pool
}

// NewToolCostRecorder creates a cost recorder.
func NewToolCostRecorder(cfg *ToolCostConfig, pool *pgxpool.Pool) *ToolCostRecorder {
	return &ToolCostRecorder{config: cfg, pool: pool}
}

// RecordToolCost computes and persists the cost for a single tool invocation.
// Updates the tool_executions table with cost data.
// Non-fatal: logs errors but does not fail the caller.
func (r *ToolCostRecorder) RecordToolCost(
	ctx context.Context,
	executionID string,
	toolKey string,
	responseBytes int64,
) {
	if r.pool == nil {
		return
	}
	cost := r.config.ComputeCost(toolKey, responseBytes)
	_, err := r.pool.Exec(ctx, `
		UPDATE tool_executions
		SET tool_cost_micro_cents = $2,
		    response_bytes        = $3
		WHERE id = $1::uuid`,
		executionID, cost, responseBytes,
	)
	if err != nil {
		log.Printf("[mcp cost] record failed for %s: %v", toolKey, err)
	}
}

// BuildToolCostBreakdown returns per-tool cost totals for a workspace in a time range.
func BuildToolCostBreakdown(ctx context.Context, pool *pgxpool.Pool, workspaceID string, fromISO, toISO string) (map[string]int64, error) {
	if pool == nil {
		return map[string]int64{}, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT tool_key, SUM(tool_cost_micro_cents) as total_cost
		FROM tool_executions
		WHERE workspace_id = $1::uuid
		  AND created_at BETWEEN $2::timestamptz AND $3::timestamptz
		  AND tool_cost_micro_cents > 0
		GROUP BY tool_key
		ORDER BY total_cost DESC`,
		workspaceID, fromISO, toISO,
	)
	if err != nil {
		return nil, fmt.Errorf("build tool cost breakdown: %w", err)
	}
	defer rows.Close()

	result := map[string]int64{}
	for rows.Next() {
		var toolKey string
		var totalCost int64
		if err := rows.Scan(&toolKey, &totalCost); err != nil {
			return nil, err
		}
		result[toolKey] = totalCost
	}
	return result, nil
}
