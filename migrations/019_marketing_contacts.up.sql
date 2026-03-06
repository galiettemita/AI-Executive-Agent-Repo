BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_contacts (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  email VARCHAR(255),
  phone VARCHAR(20),
  first_name VARCHAR(100),
  last_name VARCHAR(100),
  company VARCHAR(256),
  title VARCHAR(128),
  source VARCHAR(32) CHECK (source IN ('manual','import','scraped','api','linkedin','crm')),
  tags TEXT[] NOT NULL DEFAULT '{}',
  custom_fields JSONB DEFAULT '{}',
  lead_score INTEGER NOT NULL DEFAULT 0,
  status VARCHAR(16) NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','unsubscribed','bounced','invalid')),
  last_contacted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_contacts_user ON public.marketing_contacts(user_id);
CREATE INDEX IF NOT EXISTS idx_marketing_contacts_email ON public.marketing_contacts(user_id, email);
CREATE INDEX IF NOT EXISTS idx_marketing_contacts_status ON public.marketing_contacts(status);
CREATE INDEX IF NOT EXISTS idx_marketing_contacts_score ON public.marketing_contacts(lead_score DESC);

ALTER TABLE public.marketing_contacts ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_contacts_user_or_service ON public.marketing_contacts;
CREATE POLICY marketing_contacts_user_or_service ON public.marketing_contacts
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
