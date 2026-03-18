-- migrations/055_eq_ab_results.up.sql
CREATE TABLE IF NOT EXISTS eq_ab_results (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID        NOT NULL,
    request_id   TEXT        NOT NULL,
    orm_score    FLOAT8      NOT NULL,
    eq_enabled   BOOL        NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_eq_ab_workspace ON eq_ab_results (workspace_id, eq_enabled, recorded_at);
ALTER TABLE eq_ab_results ENABLE ROW LEVEL SECURITY;
CREATE POLICY eq_ab_isolation ON eq_ab_results
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
