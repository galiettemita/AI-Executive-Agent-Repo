BEGIN;

DROP POLICY IF EXISTS cron_notifications_user_or_service ON public.cron_notifications;
DROP INDEX IF EXISTS idx_cron_notifications_job;
DROP INDEX IF EXISTS idx_cron_notifications_user;
DROP TABLE IF EXISTS public.cron_notifications;

COMMIT;
