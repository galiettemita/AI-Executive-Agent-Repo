-- Migration: 020_BREVIO_critic_traces
-- Purpose: Durable storage for critic and reflector outputs.

CREATE TABLE IF NOT EXISTS critic_traces (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     TEXT        NOT NULL,
    request_id       TEXT        NOT NULL,
    iteration        INT         NOT NULL DEFAULT 0,
    quality_score    FLOAT8      NOT NULL DEFAULT 0,
    should_retry     BOOLEAN     NOT NULL DEFAULT FALSE,
    semantic_verdict TEXT,
    issues           JSONB,
    retry_hints      JSONB,
    step_verdicts    JSONB,
    raw_output       JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_critic_traces_workspace_id
    ON critic_traces (workspace_id);

CREATE INDEX IF NOT EXISTS idx_critic_traces_created_at
    ON critic_traces (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_critic_traces_should_retry
    ON critic_traces (should_retry)
    WHERE should_retry = TRUE;

COMMENT ON TABLE critic_traces IS
    'Stores critic outputs for quality analytics. Retention managed externally.';
