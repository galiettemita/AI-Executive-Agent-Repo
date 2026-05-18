BEGIN;

DROP POLICY IF EXISTS agent_tool_registry_write ON public.agent_tool_registry;
DROP POLICY IF EXISTS agent_tool_registry_read ON public.agent_tool_registry;
DROP INDEX IF EXISTS idx_agent_tool_registry_name;
DROP TABLE IF EXISTS public.agent_tool_registry;

COMMIT;
