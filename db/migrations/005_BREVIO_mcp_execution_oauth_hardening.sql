ALTER TABLE tool_executions
  ADD COLUMN IF NOT EXISTS is_mcp boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS mcp_server_id text,
  ADD COLUMN IF NOT EXISTS content_provenance text NOT NULL DEFAULT 'native_result';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'tool_executions_content_provenance_check'
  ) THEN
    ALTER TABLE tool_executions
      ADD CONSTRAINT tool_executions_content_provenance_check
      CHECK (content_provenance IN ('native_result', 'mcp_result'));
  END IF;
END;
$$;

ALTER TABLE user_oauth_tokens
  ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'connector_unknown';

CREATE INDEX IF NOT EXISTS idx_tool_executions_is_mcp ON tool_executions(is_mcp);
CREATE INDEX IF NOT EXISTS idx_tool_executions_mcp_server_id ON tool_executions(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_user_oauth_tokens_provider ON user_oauth_tokens(provider);
