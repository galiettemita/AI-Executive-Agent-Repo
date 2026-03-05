BEGIN;

DROP TRIGGER IF EXISTS trg_enforce_monthly_budget ON public.users;
DROP FUNCTION IF EXISTS public.enforce_monthly_budget();
DROP TRIGGER IF EXISTS trg_messages_channel_dedup ON public.messages;
DROP FUNCTION IF EXISTS public.enforce_message_channel_id_dedup();

DROP TABLE IF EXISTS public.sessions;
DROP TABLE IF EXISTS public.message_channel_dedup;
DROP TABLE IF EXISTS public.messages;

ALTER TABLE public.users
  DROP COLUMN IF EXISTS deleted_at,
  DROP COLUMN IF EXISTS enabled_skills,
  DROP COLUMN IF EXISTS profile_hash,
  DROP COLUMN IF EXISTS monthly_llm_used_cents,
  DROP COLUMN IF EXISTS monthly_llm_budget_cents,
  DROP COLUMN IF EXISTS locale,
  DROP COLUMN IF EXISTS tier,
  DROP COLUMN IF EXISTS channel,
  DROP COLUMN IF EXISTS display_name,
  DROP COLUMN IF EXISTS phone_number;

ALTER TABLE public.users
  DROP CONSTRAINT IF EXISTS users_channel_check,
  DROP CONSTRAINT IF EXISTS users_tier_check;

COMMIT;
