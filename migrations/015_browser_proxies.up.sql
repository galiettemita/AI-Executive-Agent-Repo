BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_proxies (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  proxy_name VARCHAR(128) NOT NULL,
  proxy_type VARCHAR(16) NOT NULL CHECK (proxy_type IN ('residential','datacenter','mobile','isp')),
  endpoint VARCHAR(256) NOT NULL,
  port INTEGER NOT NULL,
  username VARCHAR(128),
  password_encrypted TEXT,
  country_code CHAR(2),
  city VARCHAR(64),
  provider VARCHAR(64),
  is_rotating BOOLEAN NOT NULL DEFAULT false,
  rotation_interval_ms INTEGER,
  max_concurrent INTEGER NOT NULL DEFAULT 5,
  health_status VARCHAR(16) NOT NULL DEFAULT 'unknown'
    CHECK (health_status IN ('healthy','degraded','down','unknown')),
  last_health_check_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_proxies_user_id ON public.browser_proxies(user_id);
CREATE INDEX IF NOT EXISTS idx_browser_proxies_health ON public.browser_proxies(health_status);

ALTER TABLE public.browser_proxies ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_proxies_user_or_service ON public.browser_proxies;
CREATE POLICY browser_proxies_user_or_service ON public.browser_proxies
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
