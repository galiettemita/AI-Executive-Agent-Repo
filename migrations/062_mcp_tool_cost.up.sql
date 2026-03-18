-- migrations/062_mcp_tool_cost.up.sql
-- Add per-tool cost attribution columns to tool_executions.

ALTER TABLE tool_executions
    ADD COLUMN IF NOT EXISTS tool_cost_micro_cents BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS response_bytes        BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_tool_executions_cost
    ON tool_executions (workspace_id, tool_cost_micro_cents)
    WHERE tool_cost_micro_cents > 0;
