BEGIN;

DROP POLICY IF EXISTS agent_executions_user_or_service ON public.agent_executions;
DROP INDEX IF EXISTS idx_agent_executions_created;
DROP INDEX IF EXISTS idx_agent_executions_status;
DROP INDEX IF EXISTS idx_agent_executions_parent;
DROP INDEX IF EXISTS idx_agent_executions_user;
DROP INDEX IF EXISTS idx_agent_executions_agent;
DROP TABLE IF EXISTS public.agent_executions;

COMMIT;
