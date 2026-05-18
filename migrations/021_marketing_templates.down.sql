BEGIN;

DROP POLICY IF EXISTS marketing_templates_user_or_service ON public.marketing_templates;
DROP INDEX IF EXISTS idx_marketing_templates_type;
DROP INDEX IF EXISTS idx_marketing_templates_user;
DROP TABLE IF EXISTS public.marketing_templates;

COMMIT;
