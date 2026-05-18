BEGIN;

DROP POLICY IF EXISTS memory_kg_user_or_service ON public.memory_knowledge_graph;
DROP INDEX IF EXISTS idx_memory_kg_source_doc;
DROP INDEX IF EXISTS idx_memory_kg_predicate;
DROP INDEX IF EXISTS idx_memory_kg_object;
DROP INDEX IF EXISTS idx_memory_kg_subject;
DROP INDEX IF EXISTS idx_memory_kg_user;
DROP TABLE IF EXISTS public.memory_knowledge_graph;

COMMIT;
