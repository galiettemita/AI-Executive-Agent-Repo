-- Reverse 012_consent_audit_oauth.up.sql in FK-safe order.

BEGIN;

DROP POLICY IF EXISTS audit_log_service_or_admin_read ON public.audit_log;
DROP POLICY IF EXISTS audit_log_service_or_admin_write ON public.audit_log;
DROP TABLE IF EXISTS public.audit_log;

DROP POLICY IF EXISTS oauth_state_nonces_user_or_service ON public.oauth_state_nonces;
DROP TABLE IF EXISTS public.oauth_state_nonces;

DROP POLICY IF EXISTS oauth_pending_messages_user_or_service ON public.oauth_pending_messages;
DROP TABLE IF EXISTS public.oauth_pending_messages;

DROP INDEX IF EXISTS idx_oauth_tokens_needs_refresh;
ALTER TABLE auth.oauth_tokens
  DROP COLUMN IF EXISTS last_refreshed_at,
  DROP COLUMN IF EXISTS key_version,
  DROP COLUMN IF EXISTS needs_reauth;

DROP TRIGGER IF EXISTS trg_user_skill_consent_active_user ON public.user_skill_consent;
DROP POLICY IF EXISTS user_skill_consent_user_or_service ON public.user_skill_consent;
DROP TABLE IF EXISTS public.user_skill_consent;

ALTER TABLE public.users DROP COLUMN IF EXISTS onboarding_card_dismissed_at;

COMMIT;
