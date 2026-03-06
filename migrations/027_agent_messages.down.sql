BEGIN;

DROP POLICY IF EXISTS agent_messages_user_or_service ON public.agent_messages;
DROP INDEX IF EXISTS idx_agent_messages_created;
DROP INDEX IF EXISTS idx_agent_messages_user;
DROP INDEX IF EXISTS idx_agent_messages_execution;
DROP TABLE IF EXISTS public.agent_messages;

COMMIT;
