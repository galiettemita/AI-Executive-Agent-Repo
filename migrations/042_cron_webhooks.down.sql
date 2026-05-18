BEGIN;

DROP POLICY IF EXISTS cron_webhooks_user_or_service ON public.cron_webhooks;
DROP INDEX IF EXISTS idx_cron_webhooks_job;
DROP INDEX IF EXISTS idx_cron_webhooks_user;
DROP TABLE IF EXISTS public.cron_webhooks;

COMMIT;
