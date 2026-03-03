BEGIN;

DROP TRIGGER IF EXISTS trg_users_validate_preferences ON public.users;
DROP FUNCTION IF EXISTS public.validate_user_preferences_json();
ALTER TABLE public.users DROP COLUMN IF EXISTS preferences;

COMMIT;
