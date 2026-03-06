BEGIN;

DROP POLICY IF EXISTS browser_sessions_user_or_service ON public.browser_sessions;
DROP INDEX IF EXISTS idx_browser_sessions_created_at;
DROP INDEX IF EXISTS idx_browser_sessions_status;
DROP INDEX IF EXISTS idx_browser_sessions_user_id;
DROP TABLE IF EXISTS public.browser_sessions;

COMMIT;
