BEGIN;

DROP POLICY IF EXISTS marketing_campaigns_user_or_service ON public.marketing_campaigns;
DROP INDEX IF EXISTS idx_marketing_campaigns_type;
DROP INDEX IF EXISTS idx_marketing_campaigns_status;
DROP INDEX IF EXISTS idx_marketing_campaigns_user;
DROP TABLE IF EXISTS public.marketing_campaigns;

COMMIT;
