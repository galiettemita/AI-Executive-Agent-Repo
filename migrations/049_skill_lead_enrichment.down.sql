BEGIN;

DROP POLICY IF EXISTS skill_enrichment_user_or_service ON public.skill_lead_enrichment;
DROP INDEX IF EXISTS idx_skill_enrichment_status;
DROP INDEX IF EXISTS idx_skill_enrichment_contact;
DROP INDEX IF EXISTS idx_skill_enrichment_user;
DROP TABLE IF EXISTS public.skill_lead_enrichment;

COMMIT;
