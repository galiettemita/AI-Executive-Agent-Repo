BEGIN;

DROP POLICY IF EXISTS browser_tasks_user_or_service ON public.browser_tasks;
DROP INDEX IF EXISTS idx_browser_tasks_status;
DROP INDEX IF EXISTS idx_browser_tasks_user_id;
DROP INDEX IF EXISTS idx_browser_tasks_session_id;
DROP TABLE IF EXISTS public.browser_tasks;

COMMIT;
