BEGIN;

DROP POLICY IF EXISTS cron_jobs_user_or_service ON public.cron_jobs;
DROP INDEX IF EXISTS idx_cron_jobs_user_name;
DROP INDEX IF EXISTS idx_cron_jobs_next_run;
DROP INDEX IF EXISTS idx_cron_jobs_status;
DROP INDEX IF EXISTS idx_cron_jobs_user;
DROP TABLE IF EXISTS public.cron_jobs;

COMMIT;
