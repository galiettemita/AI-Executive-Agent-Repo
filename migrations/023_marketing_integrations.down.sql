BEGIN;

DROP POLICY IF EXISTS marketing_integrations_user_or_service ON public.marketing_integrations;
DROP INDEX IF EXISTS idx_marketing_integrations_user_platform;
DROP INDEX IF EXISTS idx_marketing_integrations_user;
DROP TABLE IF EXISTS public.marketing_integrations;

COMMIT;
