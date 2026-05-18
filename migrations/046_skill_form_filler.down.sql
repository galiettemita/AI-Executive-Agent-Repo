BEGIN;

DROP POLICY IF EXISTS skill_form_filler_user_or_service ON public.skill_form_filler_templates;
DROP INDEX IF EXISTS idx_skill_form_filler_user;
DROP TABLE IF EXISTS public.skill_form_filler_templates;

COMMIT;
