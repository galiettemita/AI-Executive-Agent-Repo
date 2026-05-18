BEGIN;

DROP POLICY IF EXISTS skill_content_user_or_service ON public.skill_content_generations;
DROP INDEX IF EXISTS idx_skill_content_status;
DROP INDEX IF EXISTS idx_skill_content_type;
DROP INDEX IF EXISTS idx_skill_content_user;
DROP TABLE IF EXISTS public.skill_content_generations;

COMMIT;
