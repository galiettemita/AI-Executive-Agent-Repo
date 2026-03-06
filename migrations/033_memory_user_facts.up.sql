BEGIN;

CREATE TABLE IF NOT EXISTS public.memory_user_facts (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  fact_type VARCHAR(32) NOT NULL CHECK (fact_type IN ('preference','context','relationship','goal','constraint','habit','demographic')),
  fact_key VARCHAR(128) NOT NULL,
  fact_value TEXT NOT NULL,
  confidence NUMERIC(3,2) NOT NULL DEFAULT 0.8,
  source VARCHAR(32) CHECK (source IN ('explicit','inferred','conversation','profile')),
  source_message_id UUID,
  is_active BOOLEAN NOT NULL DEFAULT true,
  contradicts_fact_id UUID REFERENCES public.memory_user_facts(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_memory_user_facts_user ON public.memory_user_facts(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_user_facts_type ON public.memory_user_facts(user_id, fact_type);
CREATE INDEX IF NOT EXISTS idx_memory_user_facts_key ON public.memory_user_facts(user_id, fact_key);
CREATE INDEX IF NOT EXISTS idx_memory_user_facts_active ON public.memory_user_facts(user_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_memory_user_facts_contradicts ON public.memory_user_facts(contradicts_fact_id);

ALTER TABLE public.memory_user_facts ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS memory_user_facts_user_or_service ON public.memory_user_facts;
CREATE POLICY memory_user_facts_user_or_service ON public.memory_user_facts
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
