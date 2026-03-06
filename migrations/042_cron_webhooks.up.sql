BEGIN;

CREATE TABLE IF NOT EXISTS public.cron_webhooks (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  job_id UUID NOT NULL REFERENCES public.cron_jobs(id) ON DELETE CASCADE,
  webhook_url VARCHAR(512) NOT NULL,
  http_method VARCHAR(8) NOT NULL DEFAULT 'POST' CHECK (http_method IN ('GET','POST','PUT','PATCH','DELETE')),
  headers_json JSONB DEFAULT '{}',
  body_template TEXT,
  auth_type VARCHAR(16) CHECK (auth_type IN ('none','bearer','basic','api_key','hmac')),
  auth_credentials_encrypted TEXT,
  timeout_ms INTEGER NOT NULL DEFAULT 30000,
  retry_on_failure BOOLEAN NOT NULL DEFAULT true,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_webhooks_user ON public.cron_webhooks(user_id);
CREATE INDEX IF NOT EXISTS idx_cron_webhooks_job ON public.cron_webhooks(job_id);

ALTER TABLE public.cron_webhooks ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cron_webhooks_user_or_service ON public.cron_webhooks;
CREATE POLICY cron_webhooks_user_or_service ON public.cron_webhooks
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
