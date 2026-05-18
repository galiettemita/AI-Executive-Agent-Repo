BEGIN;

DROP POLICY IF EXISTS memory_documents_user_or_service ON public.memory_documents;
DROP INDEX IF EXISTS idx_memory_documents_created;
DROP INDEX IF EXISTS idx_memory_documents_source;
DROP INDEX IF EXISTS idx_memory_documents_namespace;
DROP INDEX IF EXISTS idx_memory_documents_user;
DROP TABLE IF EXISTS public.memory_documents;

COMMIT;
