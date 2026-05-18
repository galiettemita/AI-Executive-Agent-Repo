BEGIN;

DROP POLICY IF EXISTS browser_fingerprints_user_or_service ON public.browser_fingerprints;
DROP INDEX IF EXISTS idx_browser_fingerprints_user_name;
DROP INDEX IF EXISTS idx_browser_fingerprints_user_id;
DROP TABLE IF EXISTS public.browser_fingerprints;

COMMIT;
