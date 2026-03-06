BEGIN;

DROP POLICY IF EXISTS marketing_analytics_user_or_service ON public.marketing_analytics;
DROP INDEX IF EXISTS idx_marketing_analytics_created;
DROP INDEX IF EXISTS idx_marketing_analytics_event;
DROP INDEX IF EXISTS idx_marketing_analytics_contact;
DROP INDEX IF EXISTS idx_marketing_analytics_campaign;
DROP INDEX IF EXISTS idx_marketing_analytics_user;
DROP TABLE IF EXISTS public.marketing_analytics;

COMMIT;
