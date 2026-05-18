BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_web_scraper_configs (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  config_name VARCHAR(128) NOT NULL,
  target_url TEXT NOT NULL,
  selectors_json JSONB NOT NULL DEFAULT '{}',
  output_format VARCHAR(16) NOT NULL DEFAULT 'json'
    CHECK (output_format IN ('json','csv','text','markdown')),
  use_browser BOOLEAN NOT NULL DEFAULT false,
  fingerprint_id UUID REFERENCES public.browser_fingerprints(id) ON DELETE SET NULL,
  proxy_id UUID REFERENCES public.browser_proxies(id) ON DELETE SET NULL,
  schedule_cron VARCHAR(128),
  pagination_config JSONB DEFAULT '{}',
  rate_limit_ms INTEGER NOT NULL DEFAULT 1000,
  max_pages INTEGER NOT NULL DEFAULT 10,
  last_run_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_scraper_user ON public.skill_web_scraper_configs(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_scraper_fingerprint ON public.skill_web_scraper_configs(fingerprint_id);
CREATE INDEX IF NOT EXISTS idx_skill_scraper_proxy ON public.skill_web_scraper_configs(proxy_id);

ALTER TABLE public.skill_web_scraper_configs ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_scraper_user_or_service ON public.skill_web_scraper_configs;
CREATE POLICY skill_scraper_user_or_service ON public.skill_web_scraper_configs
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
