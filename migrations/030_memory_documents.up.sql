BEGIN;

CREATE TABLE IF NOT EXISTS public.memory_documents (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  namespace VARCHAR(64) NOT NULL DEFAULT 'default',
  title VARCHAR(512),
  content TEXT NOT NULL,
  content_type VARCHAR(32) NOT NULL DEFAULT 'text'
    CHECK (content_type IN ('text','markdown','html','json','code')),
  source VARCHAR(32) CHECK (source IN ('user','conversation','skill','import','web')),
  source_url TEXT,
  metadata_json JSONB DEFAULT '{}',
  token_count INTEGER NOT NULL DEFAULT 0,
  chunk_count INTEGER NOT NULL DEFAULT 0,
  is_archived BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memory_documents_user ON public.memory_documents(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_documents_namespace ON public.memory_documents(user_id, namespace);
CREATE INDEX IF NOT EXISTS idx_memory_documents_source ON public.memory_documents(source);
CREATE INDEX IF NOT EXISTS idx_memory_documents_created ON public.memory_documents(created_at DESC);

ALTER TABLE public.memory_documents ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memory_documents_user_or_service ON public.memory_documents;
CREATE POLICY memory_documents_user_or_service ON public.memory_documents
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
