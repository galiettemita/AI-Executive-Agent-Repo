BEGIN;

DROP POLICY IF EXISTS skill_social_user_or_service ON public.skill_social_posts;
DROP INDEX IF EXISTS idx_skill_social_campaign;
DROP INDEX IF EXISTS idx_skill_social_scheduled;
DROP INDEX IF EXISTS idx_skill_social_status;
DROP INDEX IF EXISTS idx_skill_social_platform;
DROP INDEX IF EXISTS idx_skill_social_user;
DROP TABLE IF EXISTS public.skill_social_posts;

COMMIT;
