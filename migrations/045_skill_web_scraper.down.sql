BEGIN;

DROP POLICY IF EXISTS skill_scraper_user_or_service ON public.skill_web_scraper_configs;
DROP INDEX IF EXISTS idx_skill_scraper_proxy;
DROP INDEX IF EXISTS idx_skill_scraper_fingerprint;
DROP INDEX IF EXISTS idx_skill_scraper_user;
DROP TABLE IF EXISTS public.skill_web_scraper_configs;

COMMIT;
