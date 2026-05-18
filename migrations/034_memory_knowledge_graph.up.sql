BEGIN;

CREATE TABLE IF NOT EXISTS public.memory_knowledge_graph (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  subject VARCHAR(256) NOT NULL,
  predicate VARCHAR(128) NOT NULL,
  object VARCHAR(256) NOT NULL,
  subject_type VARCHAR(32) CHECK (subject_type IN ('person','company','product','concept','location','event','skill')),
  object_type VARCHAR(32) CHECK (object_type IN ('person','company','product','concept','location','event','skill')),
  confidence NUMERIC(3,2) NOT NULL DEFAULT 0.8,
  source_document_id UUID REFERENCES public.memory_documents(id) ON DELETE SET NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memory_kg_user ON public.memory_knowledge_graph(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_kg_subject ON public.memory_knowledge_graph(user_id, subject);
CREATE INDEX IF NOT EXISTS idx_memory_kg_object ON public.memory_knowledge_graph(user_id, object);
CREATE INDEX IF NOT EXISTS idx_memory_kg_predicate ON public.memory_knowledge_graph(predicate);
CREATE INDEX IF NOT EXISTS idx_memory_kg_source_doc ON public.memory_knowledge_graph(source_document_id);

ALTER TABLE public.memory_knowledge_graph ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memory_kg_user_or_service ON public.memory_knowledge_graph;
CREATE POLICY memory_kg_user_or_service ON public.memory_knowledge_graph
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
