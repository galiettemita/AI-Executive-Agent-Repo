-- migrations/062_mcp_tool_cost.down.sql
ALTER TABLE tool_executions
    DROP COLUMN IF EXISTS tool_cost_micro_cents,
    DROP COLUMN IF EXISTS response_bytes;
