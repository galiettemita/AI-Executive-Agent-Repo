-- migrations/054_world_model.up.sql
CREATE TABLE IF NOT EXISTS world_model_facts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID       NOT NULL,
    subject     TEXT        NOT NULL,
    predicate   TEXT        NOT NULL,
    value       TEXT        NOT NULL,
    source      TEXT        NOT NULL DEFAULT '',
    confidence  FLOAT8      NOT NULL DEFAULT 1.0,
    learned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_wmf_upsert    ON world_model_facts (workspace_id, subject, predicate);
CREATE INDEX IF NOT EXISTS idx_wmf_workspace ON world_model_facts (workspace_id);
CREATE INDEX IF NOT EXISTS idx_wmf_subject   ON world_model_facts (workspace_id, subject);
CREATE INDEX IF NOT EXISTS idx_wmf_expiry    ON world_model_facts (expires_at);

ALTER TABLE world_model_facts ENABLE ROW LEVEL SECURITY;

CREATE POLICY wmf_isolation ON world_model_facts
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
