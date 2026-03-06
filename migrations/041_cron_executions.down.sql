BEGIN;

DROP POLICY IF EXISTS cron_executions_user_or_service ON public.cron_executions;
DROP INDEX IF EXISTS idx_cron_executions_scheduled;
DROP INDEX IF EXISTS idx_cron_executions_status;
DROP INDEX IF EXISTS idx_cron_executions_user;
DROP INDEX IF EXISTS idx_cron_executions_job;
DROP TABLE IF EXISTS public.cron_executions;

COMMIT;
