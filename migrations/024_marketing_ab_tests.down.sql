BEGIN;

DROP POLICY IF EXISTS marketing_ab_tests_user_or_service ON public.marketing_ab_tests;
DROP INDEX IF EXISTS idx_marketing_ab_tests_campaign;
DROP INDEX IF EXISTS idx_marketing_ab_tests_user;
DROP TABLE IF EXISTS public.marketing_ab_tests;

COMMIT;
