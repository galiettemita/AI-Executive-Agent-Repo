BEGIN;

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS public.memory_embeddings (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  document_id UUID NOT NULL REFERENCES public.memory_documents(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  chunk_index INTEGER NOT NULL DEFAULT 0,
  chunk_text TEXT NOT NULL,
  embedding vector(1536),
  model_id VARCHAR(64) NOT NULL DEFAULT 'text-embedding-3-small',
  token_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memory_embeddings_document ON public.memory_embeddings(document_id);
CREATE INDEX IF NOT EXISTS idx_memory_embeddings_user ON public.memory_embeddings(user_id);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_memory_embeddings_vector'
  ) THEN
    CREATE INDEX idx_memory_embeddings_vector ON public.memory_embeddings
      USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
  END IF;
END
$$;

ALTER TABLE public.memory_embeddings ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memory_embeddings_user_or_service ON public.memory_embeddings;
CREATE POLICY memory_embeddings_user_or_service ON public.memory_embeddings
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
