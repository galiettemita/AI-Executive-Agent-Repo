BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_captcha_solutions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  session_id UUID NOT NULL REFERENCES public.browser_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  captcha_type VARCHAR(32) NOT NULL CHECK (captcha_type IN ('recaptcha_v2','recaptcha_v3','hcaptcha','funcaptcha','turnstile','image','text')),
  site_key VARCHAR(256),
  page_url TEXT,
  solver_provider VARCHAR(32) NOT NULL CHECK (solver_provider IN ('2captcha','anticaptcha','capsolver','builtin')),
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','solving','solved','failed','expired')),
  solution_token TEXT,
  solve_time_ms INTEGER,
  cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  error_code VARCHAR(64),
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_captcha_session ON public.browser_captcha_solutions(session_id);
CREATE INDEX IF NOT EXISTS idx_browser_captcha_user ON public.browser_captcha_solutions(user_id);
CREATE INDEX IF NOT EXISTS idx_browser_captcha_status ON public.browser_captcha_solutions(status);

ALTER TABLE public.browser_captcha_solutions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_captcha_user_or_service ON public.browser_captcha_solutions;
CREATE POLICY browser_captcha_user_or_service ON public.browser_captcha_solutions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
