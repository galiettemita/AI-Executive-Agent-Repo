-- migrations/058_orm_critic_trace.down.sql
DROP INDEX IF EXISTS idx_critic_traces_workspace_type;
ALTER TABLE critic_traces DROP COLUMN IF EXISTS score_type;
