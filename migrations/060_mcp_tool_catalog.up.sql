-- migrations/060_mcp_tool_catalog.up.sql
-- MCP tool discovery catalog for dynamic tool registration.

CREATE TABLE IF NOT EXISTS mcp_tool_catalog (
    tool_key          TEXT PRIMARY KEY,
    server_id         TEXT NOT NULL,
    schema_hash       TEXT NOT NULL,
    schema_json       JSONB NOT NULL,
    discovered_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_verified_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_active         BOOLEAN NOT NULL DEFAULT true,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_catalog_server
    ON mcp_tool_catalog (server_id, is_active);
CREATE INDEX IF NOT EXISTS idx_mcp_tool_catalog_active
    ON mcp_tool_catalog (is_active, updated_at DESC);
