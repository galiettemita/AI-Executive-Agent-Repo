BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_analytics (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  campaign_id UUID REFERENCES public.marketing_campaigns(id) ON DELETE SET NULL,
  contact_id UUID REFERENCES public.marketing_contacts(id) ON DELETE SET NULL,
  event_type VARCHAR(32) NOT NULL CHECK (event_type IN ('sent','delivered','opened','clicked','bounced','unsubscribed','converted','replied')),
  channel VARCHAR(16) NOT NULL CHECK (channel IN ('email','sms','social','ads','push','webhook')),
  metadata_json JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_analytics_user ON public.marketing_analytics(user_id);
CREATE INDEX IF NOT EXISTS idx_marketing_analytics_campaign ON public.marketing_analytics(campaign_id);
CREATE INDEX IF NOT EXISTS idx_marketing_analytics_event ON public.marketing_analytics(event_type);
CREATE INDEX IF NOT EXISTS idx_marketing_analytics_contact ON public.marketing_analytics(contact_id);
CREATE INDEX IF NOT EXISTS idx_marketing_analytics_created ON public.marketing_analytics(created_at DESC);

ALTER TABLE public.marketing_analytics ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_analytics_user_or_service ON public.marketing_analytics;
CREATE POLICY marketing_analytics_user_or_service ON public.marketing_analytics
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
