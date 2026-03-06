BEGIN;

DROP POLICY IF EXISTS skill_email_user_or_service ON public.skill_email_outreach;
DROP INDEX IF EXISTS idx_skill_email_scheduled;
DROP INDEX IF EXISTS idx_skill_email_status;
DROP INDEX IF EXISTS idx_skill_email_sequence;
DROP INDEX IF EXISTS idx_skill_email_contact;
DROP INDEX IF EXISTS idx_skill_email_user;
DROP TABLE IF EXISTS public.skill_email_outreach;

COMMIT;
