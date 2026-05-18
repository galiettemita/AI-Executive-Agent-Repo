BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_sequences (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  campaign_id UUID REFERENCES public.marketing_campaigns(id) ON DELETE SET NULL,
  sequence_name VARCHAR(256) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','active','paused','completed')),
  trigger_type VARCHAR(32) NOT NULL CHECK (trigger_type IN ('manual','event','schedule','webhook')),
  trigger_config JSONB DEFAULT '{}',
  steps_json JSONB NOT NULL DEFAULT '[]',
  max_contacts INTEGER,
  enrolled_count INTEGER NOT NULL DEFAULT 0,
  completed_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_sequences_user ON public.marketing_sequences(user_id);
CREATE INDEX IF NOT EXISTS idx_marketing_sequences_campaign ON public.marketing_sequences(campaign_id);
CREATE INDEX IF NOT EXISTS idx_marketing_sequences_status ON public.marketing_sequences(status);

ALTER TABLE public.marketing_sequences ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_sequences_user_or_service ON public.marketing_sequences;
CREATE POLICY marketing_sequences_user_or_service ON public.marketing_sequences
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
