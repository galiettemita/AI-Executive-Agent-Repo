BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_integrations (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  platform VARCHAR(32) NOT NULL CHECK (platform IN ('mailchimp','sendgrid','twilio','hubspot','salesforce','linkedin','google_ads','meta_ads','twitter')),
  status VARCHAR(16) NOT NULL DEFAULT 'disconnected'
    CHECK (status IN ('connected','disconnected','error','expired')),
  credentials_encrypted TEXT,
  api_endpoint VARCHAR(512),
  webhook_url VARCHAR(512),
  config_json JSONB DEFAULT '{}',
  last_sync_at TIMESTAMPTZ,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_integrations_user ON public.marketing_integrations(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_marketing_integrations_user_platform ON public.marketing_integrations(user_id, platform);

ALTER TABLE public.marketing_integrations ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_integrations_user_or_service ON public.marketing_integrations;
CREATE POLICY marketing_integrations_user_or_service ON public.marketing_integrations
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
