-- Migration 012: Skill consent, OAuth token extensions, audit log, pending messages, state nonces.
-- Supports the skill-enablement product feature: tier-based defaults, inline opt-in for
-- email/money/health, OAuth resume-on-auth, audit-grade observability for sensitive ops.
--
-- Reuses the existing auth.oauth_tokens table (migration 003) for encrypted token storage —
-- adds three columns needed by the refresh job and the resume-on-auth flow.
-- Wrapped in single transaction; companion 012_consent_audit_oauth.down.sql reverses in
-- FK-safe order.

BEGIN;

-- ---------------------------------------------------------------------------
-- 1. users.onboarding_card_dismissed_at — when the 10-second transparency card
--    was acknowledged. NULL means user has not seen the card yet.
-- ---------------------------------------------------------------------------
ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS onboarding_card_dismissed_at TIMESTAMPTZ;

-- ---------------------------------------------------------------------------
-- 2. user_skill_consent — one row per (user, sensitive category).
--    Categories: 'email' | 'money' | 'health'. Source records where the
--    state change came from for forensic audit.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.user_skill_consent (
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  category TEXT NOT NULL CHECK (category IN ('email', 'money', 'health')),
  state TEXT NOT NULL CHECK (state IN ('granted', 'revoked', 'snoozed')),
  source TEXT NOT NULL CHECK (source IN ('inline_prompt', 'settings', 'oauth_callback', 'api', 'admin')),
  granted_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  snoozed_until_session UUID,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, category)
);

ALTER TABLE public.user_skill_consent ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS user_skill_consent_user_or_service ON public.user_skill_consent;
CREATE POLICY user_skill_consent_user_or_service ON public.user_skill_consent
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

DROP TRIGGER IF EXISTS trg_user_skill_consent_active_user ON public.user_skill_consent;
CREATE TRIGGER trg_user_skill_consent_active_user
BEFORE INSERT OR UPDATE ON public.user_skill_consent
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

-- ---------------------------------------------------------------------------
-- 3. auth.oauth_tokens — REUSE existing table from migration 003. Add three
--    columns needed by the refresh job and resume-on-auth flow.
--    The existing table already provides envelope encryption (DEK per row,
--    encrypted by KEK), RLS via auth.oauth_tokens_user_or_admin policy
--    (migration 007), and triggers for active-user assertion + service
--    validation.
-- ---------------------------------------------------------------------------
ALTER TABLE auth.oauth_tokens
  ADD COLUMN IF NOT EXISTS needs_reauth BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS key_version INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS last_refreshed_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_oauth_tokens_needs_refresh
  ON auth.oauth_tokens(expires_at)
  WHERE needs_reauth = false;

-- ---------------------------------------------------------------------------
-- 4. oauth_pending_messages — supports resume-on-auth. When the user starts
--    OAuth in the middle of a chat, we stash the original message; after
--    callback we redispatch automatically. TTL ~10 min, pruned by cron.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.oauth_pending_messages (
  pending_message_id UUID PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  original_text TEXT NOT NULL,
  channel TEXT,
  session_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_oauth_pending_messages_created_at ON public.oauth_pending_messages(created_at);

ALTER TABLE public.oauth_pending_messages ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS oauth_pending_messages_user_or_service ON public.oauth_pending_messages;
CREATE POLICY oauth_pending_messages_user_or_service ON public.oauth_pending_messages
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

-- ---------------------------------------------------------------------------
-- 5. oauth_state_nonces — one-time-use OAuth state nonces with HMAC binding.
--    Prevents state replay and OAuth fixation. TTL 10 min, pruned by cron.
--    code_verifier is the PKCE secret matched at token exchange.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.oauth_state_nonces (
  nonce TEXT PRIMARY KEY,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  skill_id TEXT NOT NULL,
  code_verifier TEXT NOT NULL,
  pending_message_id UUID,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_oauth_state_nonces_created_at ON public.oauth_state_nonces(created_at);

ALTER TABLE public.oauth_state_nonces ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS oauth_state_nonces_user_or_service ON public.oauth_state_nonces;
CREATE POLICY oauth_state_nonces_user_or_service ON public.oauth_state_nonces
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

-- ---------------------------------------------------------------------------
-- 6. audit_log — every consent/OAuth/sensitive event. Retained 2 years.
--    Read access restricted to service/admin via RLS; users see redacted
--    history through /api/v1/me/audit (separate query path).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS public.audit_log (
  id BIGSERIAL PRIMARY KEY,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  actor_user_id UUID REFERENCES public.users(id) ON DELETE SET NULL,
  actor_ip INET,
  actor_user_agent TEXT,
  action TEXT NOT NULL,
  target TEXT,
  result TEXT NOT NULL CHECK (result IN ('success', 'failure')),
  detail JSONB
);

CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON public.audit_log(actor_user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_action ON public.audit_log(action, occurred_at DESC);

ALTER TABLE public.audit_log ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_log_service_or_admin_read ON public.audit_log;
CREATE POLICY audit_log_service_or_admin_read ON public.audit_log
FOR SELECT
USING (public.is_service_or_admin());

DROP POLICY IF EXISTS audit_log_service_or_admin_write ON public.audit_log;
CREATE POLICY audit_log_service_or_admin_write ON public.audit_log
FOR INSERT
WITH CHECK (public.is_service_or_admin());

COMMIT;
