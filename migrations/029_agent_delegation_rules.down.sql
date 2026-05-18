BEGIN;

DROP POLICY IF EXISTS agent_delegation_service_only ON public.agent_delegation_rules;
DROP INDEX IF EXISTS idx_agent_delegation_fallback;
DROP INDEX IF EXISTS idx_agent_delegation_worker;
DROP INDEX IF EXISTS idx_agent_delegation_supervisor;
DROP TABLE IF EXISTS public.agent_delegation_rules;

COMMIT;
