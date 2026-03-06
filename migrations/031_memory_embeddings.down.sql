BEGIN;

DROP POLICY IF EXISTS memory_embeddings_user_or_service ON public.memory_embeddings;
DROP INDEX IF EXISTS idx_memory_embeddings_vector;
DROP INDEX IF EXISTS idx_memory_embeddings_user;
DROP INDEX IF EXISTS idx_memory_embeddings_document;
DROP TABLE IF EXISTS public.memory_embeddings;

COMMIT;
