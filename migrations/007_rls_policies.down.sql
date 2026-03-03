BEGIN;

DROP POLICY IF EXISTS users_self_or_service ON public.users;
DROP POLICY IF EXISTS messages_user_or_service ON public.messages;
DROP POLICY IF EXISTS sessions_user_or_service ON public.sessions;
DROP POLICY IF EXISTS skills_registry_read ON skills.registry;
DROP POLICY IF EXISTS skills_registry_write ON skills.registry;
DROP POLICY IF EXISTS execution_log_user_or_service ON skills.execution_log;
DROP POLICY IF EXISTS circuit_breaker_admin_service ON skills.circuit_breaker_state;
DROP POLICY IF EXISTS oauth_tokens_user_or_admin ON auth.oauth_tokens;
DROP POLICY IF EXISTS usage_daily_user_or_admin ON billing.usage_daily;

DROP TRIGGER IF EXISTS trg_messages_active_user ON public.messages;
DROP TRIGGER IF EXISTS trg_sessions_active_user ON public.sessions;
DROP TRIGGER IF EXISTS trg_oauth_tokens_active_user ON auth.oauth_tokens;
DROP TRIGGER IF EXISTS trg_usage_daily_active_user ON billing.usage_daily;
DROP TRIGGER IF EXISTS trg_execution_log_active_user ON skills.execution_log;
DROP TRIGGER IF EXISTS trg_messages_validate_skill_ids ON public.messages;
DROP TRIGGER IF EXISTS trg_oauth_tokens_validate_service ON auth.oauth_tokens;
DROP TRIGGER IF EXISTS trg_usage_daily_reconcile ON billing.usage_daily;

DROP FUNCTION IF EXISTS billing.reconcile_usage_daily();
DROP FUNCTION IF EXISTS auth.validate_oauth_service();
DROP FUNCTION IF EXISTS public.validate_message_skill_ids();
DROP FUNCTION IF EXISTS public.assert_active_user();
DROP FUNCTION IF EXISTS public.is_service_or_admin();

COMMIT;
