BEGIN;

DROP POLICY IF EXISTS agent_definitions_user_or_service ON public.agent_definitions;
DROP INDEX IF EXISTS idx_agent_definitions_user_name;
DROP INDEX IF EXISTS idx_agent_definitions_type;
DROP INDEX IF EXISTS idx_agent_definitions_user;
DROP TABLE IF EXISTS public.agent_definitions;

COMMIT;
