BEGIN;

DROP POLICY IF EXISTS browser_captcha_user_or_service ON public.browser_captcha_solutions;
DROP INDEX IF EXISTS idx_browser_captcha_status;
DROP INDEX IF EXISTS idx_browser_captcha_user;
DROP INDEX IF EXISTS idx_browser_captcha_session;
DROP TABLE IF EXISTS public.browser_captcha_solutions;

COMMIT;
