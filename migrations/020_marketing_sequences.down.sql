BEGIN;

DROP POLICY IF EXISTS marketing_sequences_user_or_service ON public.marketing_sequences;
DROP INDEX IF EXISTS idx_marketing_sequences_status;
DROP INDEX IF EXISTS idx_marketing_sequences_campaign;
DROP INDEX IF EXISTS idx_marketing_sequences_user;
DROP TABLE IF EXISTS public.marketing_sequences;

COMMIT;
