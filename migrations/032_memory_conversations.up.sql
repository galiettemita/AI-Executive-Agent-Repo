BEGIN;

CREATE TABLE IF NOT EXISTS public.memory_conversations (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  session_id UUID REFERENCES public.sessions(id) ON DELETE SET NULL,
  summary TEXT,
  key_facts JSONB DEFAULT '[]',
  entities_json JSONB DEFAULT '[]',
  sentiment VARCHAR(16) CHECK (sentiment IN ('positive','negative','neutral','mixed')),
  topics TEXT[] NOT NULL DEFAULT '{}',
  importance_score NUMERIC(3,2) NOT NULL DEFAULT 0.5,
  ttl_days INTEGER,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memory_conversations_user ON public.memory_conversations(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_conversations_session ON public.memory_conversations(session_id);
CREATE INDEX IF NOT EXISTS idx_memory_conversations_importance ON public.memory_conversations(importance_score DESC);
CREATE INDEX IF NOT EXISTS idx_memory_conversations_expires ON public.memory_conversations(expires_at) WHERE expires_at IS NOT NULL;

ALTER TABLE public.memory_conversations ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memory_conversations_user_or_service ON public.memory_conversations;
CREATE POLICY memory_conversations_user_or_service ON public.memory_conversations
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
