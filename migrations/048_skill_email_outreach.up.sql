BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_email_outreach (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  sequence_id UUID REFERENCES public.marketing_sequences(id) ON DELETE SET NULL,
  contact_id UUID REFERENCES public.marketing_contacts(id) ON DELETE SET NULL,
  from_email VARCHAR(255) NOT NULL,
  to_email VARCHAR(255) NOT NULL,
  subject VARCHAR(512) NOT NULL,
  body_html TEXT,
  body_text TEXT,
  status VARCHAR(16) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','queued','sent','delivered','opened','clicked','replied','bounced','failed')),
  provider VARCHAR(32) CHECK (provider IN ('sendgrid','ses','mailchimp','smtp','gmail')),
  provider_message_id VARCHAR(256),
  open_count INTEGER NOT NULL DEFAULT 0,
  click_count INTEGER NOT NULL DEFAULT 0,
  scheduled_at TIMESTAMPTZ,
  sent_at TIMESTAMPTZ,
  opened_at TIMESTAMPTZ,
  clicked_at TIMESTAMPTZ,
  replied_at TIMESTAMPTZ,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_email_user ON public.skill_email_outreach(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_email_sequence ON public.skill_email_outreach(sequence_id);
CREATE INDEX IF NOT EXISTS idx_skill_email_contact ON public.skill_email_outreach(contact_id);
CREATE INDEX IF NOT EXISTS idx_skill_email_status ON public.skill_email_outreach(status);
CREATE INDEX IF NOT EXISTS idx_skill_email_scheduled ON public.skill_email_outreach(scheduled_at) WHERE status = 'queued';

ALTER TABLE public.skill_email_outreach ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_email_user_or_service ON public.skill_email_outreach;
CREATE POLICY skill_email_user_or_service ON public.skill_email_outreach
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
