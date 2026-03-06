BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_lead_enrichment (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  contact_id UUID REFERENCES public.marketing_contacts(id) ON DELETE SET NULL,
  input_email VARCHAR(255),
  input_domain VARCHAR(256),
  input_linkedin_url TEXT,
  provider VARCHAR(32) NOT NULL CHECK (provider IN ('clearbit','apollo','zoominfo','hunter','builtin')),
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','processing','completed','failed','partial')),
  enriched_data JSONB DEFAULT '{}',
  confidence_score NUMERIC(3,2),
  cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_enrichment_user ON public.skill_lead_enrichment(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_enrichment_contact ON public.skill_lead_enrichment(contact_id);
CREATE INDEX IF NOT EXISTS idx_skill_enrichment_status ON public.skill_lead_enrichment(status);

ALTER TABLE public.skill_lead_enrichment ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_enrichment_user_or_service ON public.skill_lead_enrichment;
CREATE POLICY skill_enrichment_user_or_service ON public.skill_lead_enrichment
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
