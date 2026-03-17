-- Migration: 040_BREVIO_eq_cross_session
-- Resolves: P1-EQ-CROSS-SESSION-STATE-LOST
-- Implements: EQ cross-session emotional state persistence
-- Storage: one row per (workspace_id, user_id), UPSERT on conflict

BEGIN;

CREATE TABLE eq_emotional_states (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     UUID        NOT NULL,
    user_id          TEXT        NOT NULL,
    valence          FLOAT       NOT NULL DEFAULT 0.0,
    arousal          FLOAT       NOT NULL DEFAULT 0.5,
    detected_emotion TEXT        NOT NULL DEFAULT 'neutral',
    confidence       FLOAT       NOT NULL DEFAULT 0.5,
    session_count    INTEGER     NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT eq_ws_user UNIQUE (workspace_id, user_id)
);

ALTER TABLE eq_emotional_states ENABLE ROW LEVEL SECURITY;

CREATE POLICY eq_emotional_states_workspace_isolation
    ON  eq_emotional_states
    AS  PERMISSIVE
    FOR ALL
    USING (
        workspace_id = current_setting('app.current_workspace_id', true)::UUID
    );

CREATE INDEX IF NOT EXISTS idx_eq_emotional_states_lookup
    ON eq_emotional_states (workspace_id, user_id);

COMMIT;
