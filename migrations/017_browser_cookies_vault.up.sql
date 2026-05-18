BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_cookies_vault (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  domain VARCHAR(256) NOT NULL,
  cookie_name VARCHAR(256) NOT NULL,
  cookie_value_encrypted TEXT NOT NULL,
  path VARCHAR(512) NOT NULL DEFAULT '/',
  secure BOOLEAN NOT NULL DEFAULT true,
  http_only BOOLEAN NOT NULL DEFAULT true,
  same_site VARCHAR(8) NOT NULL DEFAULT 'Lax' CHECK (same_site IN ('Strict','Lax','None')),
  expires_at TIMESTAMPTZ,
  source_session_id UUID REFERENCES public.browser_sessions(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_cookies_user ON public.browser_cookies_vault(user_id);
CREATE INDEX IF NOT EXISTS idx_browser_cookies_domain ON public.browser_cookies_vault(user_id, domain);
CREATE INDEX IF NOT EXISTS idx_browser_cookies_session ON public.browser_cookies_vault(source_session_id);
CREATE INDEX IF NOT EXISTS idx_browser_cookies_expires ON public.browser_cookies_vault(expires_at) WHERE expires_at IS NOT NULL;

ALTER TABLE public.browser_cookies_vault ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_cookies_user_or_service ON public.browser_cookies_vault;
CREATE POLICY browser_cookies_user_or_service ON public.browser_cookies_vault
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
