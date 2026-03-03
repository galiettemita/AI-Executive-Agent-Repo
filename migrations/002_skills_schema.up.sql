BEGIN;

CREATE SCHEMA IF NOT EXISTS skills;

CREATE TABLE IF NOT EXISTS skills.registry (
  id VARCHAR(80) PRIMARY KEY,
  category TEXT NOT NULL,
  plane TEXT NOT NULL CHECK (plane IN ('gateway', 'brain', 'hands')),
  impact TEXT NOT NULL CHECK (impact IN ('HIGH', 'MEDIUM', 'LOW')),
  description TEXT NOT NULL,
  brevio_use_case TEXT NOT NULL,
  input_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  output_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  required_scopes TEXT[] NOT NULL DEFAULT '{}',
  retry_policy JSONB NOT NULL DEFAULT '{"max_retries":3,"initial_interval_ms":500,"backoff_multiplier":2.0,"max_interval_ms":8000,"non_retryable_errors":["AUTH_REVOKED","VALIDATION_FAILED"]}'::jsonb,
  circuit_breaker_config JSONB NOT NULL DEFAULT '{"failure_threshold":5,"recovery_timeout_ms":60000,"half_open_max_calls":3}'::jsonb,
  cost_per_invocation_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  enabled BOOLEAN NOT NULL DEFAULT true,
  min_tier VARCHAR(20) NOT NULL DEFAULT 'free' CHECK (min_tier IN ('free', 'pro', 'enterprise')),
  deployment_mode VARCHAR(20) NOT NULL DEFAULT 'cloud' CHECK (deployment_mode IN ('cloud', 'local_mac', 'mcp')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS skills.execution_log (
  id UUID NOT NULL DEFAULT public.uuid_v7_now(),
  message_id UUID,
  user_id UUID NOT NULL REFERENCES public.users(id),
  skill_id VARCHAR(80) NOT NULL REFERENCES skills.registry(id),
  status VARCHAR(20) NOT NULL CHECK (status IN ('SUCCESS', 'PARTIAL', 'FAILED', 'TIMEOUT')),
  input_payload JSONB,
  output_payload JSONB,
  error_code VARCHAR(50),
  error_detail TEXT,
  retries INTEGER NOT NULL DEFAULT 0,
  latency_ms INTEGER NOT NULL,
  tokens_used INTEGER NOT NULL DEFAULT 0,
  cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  circuit_breaker_state VARCHAR(10) NOT NULL CHECK (circuit_breaker_state IN ('CLOSED', 'HALF_OPEN', 'OPEN')),
  cache_hit BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

DO $$
DECLARE
  part_name TEXT := format('execution_log_%s', to_char(now(), 'YYYYMMDD'));
  part_start TIMESTAMPTZ := date_trunc('day', now());
  part_end TIMESTAMPTZ := date_trunc('day', now()) + interval '1 day';
BEGIN
  EXECUTE format(
    'CREATE TABLE IF NOT EXISTS skills.%I PARTITION OF skills.execution_log FOR VALUES FROM (%L) TO (%L)',
    part_name,
    part_start,
    part_end
  );
END
$$;

CREATE TABLE IF NOT EXISTS skills.circuit_breaker_state (
  skill_id VARCHAR(80) PRIMARY KEY REFERENCES skills.registry(id),
  state VARCHAR(10) NOT NULL DEFAULT 'CLOSED' CHECK (state IN ('CLOSED', 'HALF_OPEN', 'OPEN')),
  failure_count INTEGER NOT NULL DEFAULT 0,
  last_failure_at TIMESTAMPTZ,
  opened_at TIMESTAMPTZ,
  half_open_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
