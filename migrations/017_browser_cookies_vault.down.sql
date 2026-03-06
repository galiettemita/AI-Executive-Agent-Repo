BEGIN;

DROP POLICY IF EXISTS browser_cookies_user_or_service ON public.browser_cookies_vault;
DROP INDEX IF EXISTS idx_browser_cookies_session;
DROP INDEX IF EXISTS idx_browser_cookies_expires;
DROP INDEX IF EXISTS idx_browser_cookies_domain;
DROP INDEX IF EXISTS idx_browser_cookies_user;
DROP TABLE IF EXISTS public.browser_cookies_vault;

COMMIT;
