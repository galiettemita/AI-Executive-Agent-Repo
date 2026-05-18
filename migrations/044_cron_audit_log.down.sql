BEGIN;

DROP POLICY IF EXISTS cron_audit_user_or_service ON public.cron_audit_log;
DROP INDEX IF EXISTS idx_cron_audit_created;
DROP INDEX IF EXISTS idx_cron_audit_action;
DROP INDEX IF EXISTS idx_cron_audit_job;
DROP INDEX IF EXISTS idx_cron_audit_user;
DROP TABLE IF EXISTS public.cron_audit_log;

COMMIT;
