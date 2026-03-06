BEGIN;

DROP POLICY IF EXISTS memory_conversations_user_or_service ON public.memory_conversations;
DROP INDEX IF EXISTS idx_memory_conversations_expires;
DROP INDEX IF EXISTS idx_memory_conversations_importance;
DROP INDEX IF EXISTS idx_memory_conversations_session;
DROP INDEX IF EXISTS idx_memory_conversations_user;
DROP TABLE IF EXISTS public.memory_conversations;

COMMIT;
