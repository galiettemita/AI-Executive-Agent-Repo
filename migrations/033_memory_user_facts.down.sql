BEGIN;

DROP POLICY IF EXISTS memory_user_facts_user_or_service ON public.memory_user_facts;
DROP INDEX IF EXISTS idx_memory_user_facts_contradicts;
DROP INDEX IF EXISTS idx_memory_user_facts_active;
DROP INDEX IF EXISTS idx_memory_user_facts_key;
DROP INDEX IF EXISTS idx_memory_user_facts_type;
DROP INDEX IF EXISTS idx_memory_user_facts_user;
DROP TABLE IF EXISTS public.memory_user_facts;

COMMIT;
