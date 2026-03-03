BEGIN;

CREATE OR REPLACE FUNCTION public.is_service_or_admin()
RETURNS boolean
LANGUAGE sql
AS $$
  SELECT current_setting('app.role', true) IN ('service', 'admin');
$$;

CREATE OR REPLACE FUNCTION public.assert_active_user()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.user_id IS NULL THEN
    RETURN NEW;
  END IF;

  IF NOT EXISTS (
    SELECT 1
    FROM public.users u
    WHERE u.id = NEW.user_id
      AND u.deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'user % not found or deleted', NEW.user_id;
  END IF;

  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION public.validate_message_skill_ids()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  skill_id TEXT;
BEGIN
  IF NEW.skill_ids IS NULL THEN
    RETURN NEW;
  END IF;

  FOREACH skill_id IN ARRAY NEW.skill_ids LOOP
    IF NOT EXISTS (SELECT 1 FROM skills.registry r WHERE r.id = skill_id) THEN
      RAISE EXCEPTION 'skill_id % does not exist in skills.registry', skill_id;
    END IF;
  END LOOP;

  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION auth.validate_oauth_service()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM skills.registry r
    WHERE r.enabled = true
      AND (
        r.id = NEW.service
        OR coalesce(r.config->>'service', '') = NEW.service
      )
  ) THEN
    RAISE EXCEPTION 'oauth service % is not referenced by any enabled skill', NEW.service;
  END IF;

  RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION billing.reconcile_usage_daily()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
  expected_exec INTEGER;
  expected_tokens INTEGER;
  expected_cost NUMERIC(10,4);
BEGIN
  SELECT
    COALESCE(COUNT(*), 0),
    COALESCE(SUM(tokens_used), 0),
    COALESCE(SUM(cost_cents), 0)
  INTO expected_exec, expected_tokens, expected_cost
  FROM skills.execution_log e
  WHERE e.user_id = NEW.user_id
    AND e.created_at::date = NEW.date;

  IF NEW.total_skill_executions <> expected_exec THEN
    RAISE EXCEPTION 'usage_daily total_skill_executions (%) does not reconcile with execution_log (%)',
      NEW.total_skill_executions,
      expected_exec;
  END IF;

  IF NEW.total_llm_tokens <> expected_tokens THEN
    RAISE EXCEPTION 'usage_daily total_llm_tokens (%) does not reconcile with execution_log (%)',
      NEW.total_llm_tokens,
      expected_tokens;
  END IF;

  IF abs(NEW.total_cost_cents - expected_cost) > 0.0001 THEN
    RAISE EXCEPTION 'usage_daily total_cost_cents (%) does not reconcile with execution_log (%)',
      NEW.total_cost_cents,
      expected_cost;
  END IF;

  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_messages_active_user ON public.messages;
CREATE TRIGGER trg_messages_active_user
BEFORE INSERT OR UPDATE ON public.messages
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

DROP TRIGGER IF EXISTS trg_sessions_active_user ON public.sessions;
CREATE TRIGGER trg_sessions_active_user
BEFORE INSERT OR UPDATE ON public.sessions
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

DROP TRIGGER IF EXISTS trg_oauth_tokens_active_user ON auth.oauth_tokens;
CREATE TRIGGER trg_oauth_tokens_active_user
BEFORE INSERT OR UPDATE ON auth.oauth_tokens
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

DROP TRIGGER IF EXISTS trg_usage_daily_active_user ON billing.usage_daily;
CREATE TRIGGER trg_usage_daily_active_user
BEFORE INSERT OR UPDATE ON billing.usage_daily
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

DROP TRIGGER IF EXISTS trg_execution_log_active_user ON skills.execution_log;
CREATE TRIGGER trg_execution_log_active_user
BEFORE INSERT OR UPDATE ON skills.execution_log
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

DROP TRIGGER IF EXISTS trg_messages_validate_skill_ids ON public.messages;
CREATE TRIGGER trg_messages_validate_skill_ids
BEFORE INSERT OR UPDATE ON public.messages
FOR EACH ROW
EXECUTE FUNCTION public.validate_message_skill_ids();

DROP TRIGGER IF EXISTS trg_oauth_tokens_validate_service ON auth.oauth_tokens;
CREATE TRIGGER trg_oauth_tokens_validate_service
BEFORE INSERT OR UPDATE ON auth.oauth_tokens
FOR EACH ROW
EXECUTE FUNCTION auth.validate_oauth_service();

DROP TRIGGER IF EXISTS trg_usage_daily_reconcile ON billing.usage_daily;
CREATE TRIGGER trg_usage_daily_reconcile
BEFORE INSERT OR UPDATE ON billing.usage_daily
FOR EACH ROW
EXECUTE FUNCTION billing.reconcile_usage_daily();

ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE skills.registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE skills.execution_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE skills.circuit_breaker_state ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth.oauth_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE billing.usage_daily ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS users_self_or_service ON public.users;
CREATE POLICY users_self_or_service ON public.users
USING (
  public.is_service_or_admin()
  OR id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR id::text = current_setting('app.user_id', true)
);

DROP POLICY IF EXISTS messages_user_or_service ON public.messages;
CREATE POLICY messages_user_or_service ON public.messages
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

DROP POLICY IF EXISTS sessions_user_or_service ON public.sessions;
CREATE POLICY sessions_user_or_service ON public.sessions
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

DROP POLICY IF EXISTS skills_registry_read ON skills.registry;
CREATE POLICY skills_registry_read ON skills.registry
FOR SELECT
USING (true);

DROP POLICY IF EXISTS skills_registry_write ON skills.registry;
CREATE POLICY skills_registry_write ON skills.registry
FOR ALL
USING (public.is_service_or_admin())
WITH CHECK (public.is_service_or_admin());

DROP POLICY IF EXISTS execution_log_user_or_service ON skills.execution_log;
CREATE POLICY execution_log_user_or_service ON skills.execution_log
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (public.is_service_or_admin());

DROP POLICY IF EXISTS circuit_breaker_admin_service ON skills.circuit_breaker_state;
CREATE POLICY circuit_breaker_admin_service ON skills.circuit_breaker_state
USING (public.is_service_or_admin())
WITH CHECK (public.is_service_or_admin());

DROP POLICY IF EXISTS oauth_tokens_user_or_admin ON auth.oauth_tokens;
CREATE POLICY oauth_tokens_user_or_admin ON auth.oauth_tokens
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

DROP POLICY IF EXISTS usage_daily_user_or_admin ON billing.usage_daily;
CREATE POLICY usage_daily_user_or_admin ON billing.usage_daily
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (public.is_service_or_admin());

COMMIT;
