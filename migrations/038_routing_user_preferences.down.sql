BEGIN;

DROP POLICY IF EXISTS routing_user_prefs_user_or_service ON public.routing_user_preferences;
DROP INDEX IF EXISTS idx_routing_user_prefs_user;
DROP TABLE IF EXISTS public.routing_user_preferences;

COMMIT;
