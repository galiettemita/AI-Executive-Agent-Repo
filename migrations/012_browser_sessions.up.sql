BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_sessions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  skill_id VARCHAR(64) NOT NULL,
  session_type VARCHAR(32) NOT NULL CHECK (session_type IN ('stealth','playwright','puppeteer')),
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','active','completed','failed','timed_out')),
  proxy_endpoint VARCHAR(256),
  user_agent TEXT,
  cookies_json JSONB DEFAULT '{}',
  viewport_width INTEGER DEFAULT 1920,
  viewport_height INTEGER DEFAULT 1080,
  timeout_ms INTEGER NOT NULL DEFAULT 120000,
  error_code VARCHAR(64),
  error_message TEXT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_sessions_user_id ON public.browser_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_status ON public.browser_sessions(status);
CREATE INDEX IF NOT EXISTS idx_browser_sessions_created_at ON public.browser_sessions(created_at DESC);

ALTER TABLE public.browser_sessions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_sessions_user_or_service ON public.browser_sessions;
CREATE POLICY browser_sessions_user_or_service ON public.browser_sessions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
