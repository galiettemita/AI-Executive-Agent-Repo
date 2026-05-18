BEGIN;

DROP POLICY IF EXISTS marketing_contacts_user_or_service ON public.marketing_contacts;
DROP INDEX IF EXISTS idx_marketing_contacts_score;
DROP INDEX IF EXISTS idx_marketing_contacts_status;
DROP INDEX IF EXISTS idx_marketing_contacts_email;
DROP INDEX IF EXISTS idx_marketing_contacts_user;
DROP TABLE IF EXISTS public.marketing_contacts;

COMMIT;
