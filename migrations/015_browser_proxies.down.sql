BEGIN;

DROP POLICY IF EXISTS browser_proxies_user_or_service ON public.browser_proxies;
DROP INDEX IF EXISTS idx_browser_proxies_health;
DROP INDEX IF EXISTS idx_browser_proxies_user_id;
DROP TABLE IF EXISTS public.browser_proxies;

COMMIT;
