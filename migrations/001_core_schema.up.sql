BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- UUIDv7 placeholder helper for compatibility where native v7 is unavailable.
CREATE OR REPLACE FUNCTION public.uuid_v7_now()
RETURNS uuid
LANGUAGE sql
AS $$
  SELECT gen_random_uuid();
$$;

CREATE TABLE IF NOT EXISTS public.users (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  phone_number VARCHAR(20),
  display_name VARCHAR(100) NOT NULL DEFAULT '',
  email VARCHAR(255),
  channel VARCHAR(10) NOT NULL DEFAULT 'whatsapp',
  tier VARCHAR(20) NOT NULL DEFAULT 'free',
  timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
  locale VARCHAR(10) NOT NULL DEFAULT 'en-US',
  monthly_llm_budget_cents INTEGER NOT NULL DEFAULT 500,
  monthly_llm_used_cents INTEGER NOT NULL DEFAULT 0,
  profile_hash CHAR(64) NOT NULL DEFAULT repeat('0', 64),
  enabled_skills TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS phone_number VARCHAR(20),
  ADD COLUMN IF NOT EXISTS display_name VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS channel VARCHAR(10) NOT NULL DEFAULT 'whatsapp',
  ADD COLUMN IF NOT EXISTS tier VARCHAR(20) NOT NULL DEFAULT 'free',
  ADD COLUMN IF NOT EXISTS locale VARCHAR(10) NOT NULL DEFAULT 'en-US',
  ADD COLUMN IF NOT EXISTS monthly_llm_budget_cents INTEGER NOT NULL DEFAULT 500,
  ADD COLUMN IF NOT EXISTS monthly_llm_used_cents INTEGER NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS profile_hash CHAR(64) NOT NULL DEFAULT repeat('0', 64),
  ADD COLUMN IF NOT EXISTS enabled_skills TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'public'
      AND table_name = 'users'
      AND column_name = 'phone_e164'
  ) THEN
    EXECUTE 'UPDATE public.users SET phone_number = phone_e164 WHERE phone_number IS NULL AND phone_e164 IS NOT NULL';
  END IF;
END
$$;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM information_schema.check_constraints
    WHERE constraint_name = 'users_channel_check'
  ) THEN
    ALTER TABLE public.users
      ADD CONSTRAINT users_channel_check CHECK (channel IN ('whatsapp', 'imessage', 'api'));
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM information_schema.check_constraints
    WHERE constraint_name = 'users_tier_check'
  ) THEN
    ALTER TABLE public.users
      ADD CONSTRAINT users_tier_check CHECK (tier IN ('free', 'pro', 'enterprise', 'admin', 'service'));
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS public.messages (
  id UUID NOT NULL DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id),
  session_id UUID NOT NULL,
  channel VARCHAR(10) NOT NULL,
  direction VARCHAR(10) NOT NULL CHECK (direction IN ('inbound', 'outbound')),
  content_type VARCHAR(20) NOT NULL,
  content_text TEXT,
  content_media_url TEXT,
  intent VARCHAR(100),
  skill_ids TEXT[] NOT NULL DEFAULT '{}',
  status VARCHAR(20) NOT NULL DEFAULT 'received',
  latency_ms INTEGER,
  llm_tokens_used INTEGER NOT NULL DEFAULT 0,
  cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  channel_message_id VARCHAR(100) NOT NULL,
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS public.message_channel_dedup (
  channel_message_id VARCHAR(100) PRIMARY KEY,
  message_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

DO $$
DECLARE
  part_name TEXT := format('messages_%s', to_char(date_trunc('month', now()), 'YYYY_MM'));
  part_start TIMESTAMPTZ := date_trunc('month', now());
  part_end TIMESTAMPTZ := date_trunc('month', now()) + interval '1 month';
BEGIN
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS public.%I PARTITION OF public.messages FOR VALUES FROM (%L) TO (%L)',
    part_name,
    part_start,
    part_end
  );
END
$$;

CREATE OR REPLACE FUNCTION public.enforce_message_channel_id_dedup()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO public.message_channel_dedup (channel_message_id, message_id, created_at)
  VALUES (NEW.channel_message_id, NEW.id, NEW.created_at);
  RETURN NEW;
EXCEPTION
  WHEN unique_violation THEN
    RAISE EXCEPTION 'duplicate channel_message_id: %', NEW.channel_message_id;
END;
$$;

DROP TRIGGER IF EXISTS trg_messages_channel_dedup ON public.messages;
CREATE TRIGGER trg_messages_channel_dedup
BEFORE INSERT ON public.messages
FOR EACH ROW
EXECUTE FUNCTION public.enforce_message_channel_id_dedup();

CREATE TABLE IF NOT EXISTS public.sessions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id),
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  message_count INTEGER NOT NULL DEFAULT 0,
  context_snapshot JSONB,
  status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'expired', 'closed'))
);

CREATE OR REPLACE FUNCTION public.enforce_monthly_budget()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.monthly_llm_used_cents > NEW.monthly_llm_budget_cents THEN
    RAISE EXCEPTION 'monthly_llm_used_cents (%) exceeds monthly_llm_budget_cents (%)',
      NEW.monthly_llm_used_cents,
      NEW.monthly_llm_budget_cents;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_enforce_monthly_budget ON public.users;
CREATE TRIGGER trg_enforce_monthly_budget
BEFORE INSERT OR UPDATE ON public.users
FOR EACH ROW
EXECUTE FUNCTION public.enforce_monthly_budget();

COMMIT;
