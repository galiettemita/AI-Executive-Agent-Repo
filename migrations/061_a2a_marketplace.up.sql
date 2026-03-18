-- migrations/061_a2a_marketplace.up.sql
-- A2A Agent Marketplace: registry and outcome tracking tables.

CREATE TABLE IF NOT EXISTS a2a_agent_registry (
    agent_id        TEXT PRIMARY KEY,
    base_url        TEXT NOT NULL,
    capabilities    JSONB NOT NULL DEFAULT '[]',
    auth_schemes    TEXT[] NOT NULL DEFAULT '{}',
    version         TEXT NOT NULL DEFAULT '',
    last_heartbeat  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    trust_score     DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    status          TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active', 'inactive', 'suspended')),
    admin_rating    DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    registered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    workspace_id    UUID
);

CREATE INDEX IF NOT EXISTS idx_a2a_registry_status
    ON a2a_agent_registry (status, trust_score DESC);

CREATE INDEX IF NOT EXISTS idx_a2a_registry_capabilities
    ON a2a_agent_registry USING GIN (capabilities);

CREATE TABLE IF NOT EXISTS a2a_agent_outcomes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id        TEXT NOT NULL REFERENCES a2a_agent_registry(agent_id) ON DELETE CASCADE,
    success         BOOLEAN NOT NULL,
    response_time_ms BIGINT NOT NULL,
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_outcomes_agent_time
    ON a2a_agent_outcomes (agent_id, occurred_at DESC);

ALTER TABLE a2a_agent_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE a2a_agent_outcomes ENABLE ROW LEVEL SECURITY;

CREATE POLICY a2a_registry_workspace_isolation ON a2a_agent_registry
    USING (workspace_id IS NULL OR workspace_id = current_setting('app.workspace_id')::UUID);
