-- migrations/058_orm_critic_trace.up.sql
-- Extend critic_traces for ORM score storage.
-- The critic_traces table was created in db/migrations/020_BREVIO_critic_traces.sql.
-- This migration ensures all ORM-required columns exist.

ALTER TABLE critic_traces
    ADD COLUMN IF NOT EXISTS score_type TEXT DEFAULT 'critic';

CREATE INDEX IF NOT EXISTS idx_critic_traces_workspace_type
    ON critic_traces (workspace_id, score_type, created_at DESC);
