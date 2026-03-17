BEGIN;

CREATE TABLE IF NOT EXISTS external_agents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    description     TEXT,
    base_url        TEXT NOT NULL,
    m2m_token       TEXT NOT NULL,
    capabilities    TEXT[] NOT NULL DEFAULT '{}',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_external_agents_active
    ON external_agents (is_active, name);

COMMIT;
