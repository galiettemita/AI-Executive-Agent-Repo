-- BREVIO V10.4 voice/outbound calls migration (BP05)
-- Implements: 9 tables + 5 enums for voice/call capabilities
-- Call approval gate enforced; transcript-only persistence; no raw audio stored.

-- V10.4 enums
DO $$ BEGIN
  CREATE TYPE call_status AS ENUM (
    'pending_approval','approved','dialing','ringing','in_progress',
    'completed','failed','cancelled','no_answer','busy','rejected'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE call_direction AS ENUM ('outbound','inbound');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE call_approval_status AS ENUM (
    'pending','approved','denied','expired','auto_approved'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE call_provider_status AS ENUM (
    'active','degraded','offline','failover'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE transcript_segment_type AS ENUM (
    'agent','user','system','dtmf','silence'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 1: call_providers (provider adapter registry)
CREATE TABLE IF NOT EXISTS call_providers (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provider_name text NOT NULL,
  provider_type text NOT NULL,
  status call_provider_status NOT NULL DEFAULT 'active',
  priority int NOT NULL DEFAULT 0,
  config_encrypted text NOT NULL,
  health_score numeric(5,4) NOT NULL DEFAULT 1.0,
  last_health_check_at timestamptz,
  max_concurrent_calls int NOT NULL DEFAULT 10,
  current_concurrent_calls int NOT NULL DEFAULT 0,
  supported_regions text[] NOT NULL DEFAULT '{}',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, provider_name)
);

ALTER TABLE call_providers ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_providers_workspace_isolation ON call_providers
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 2: call_approval_policies
CREATE TABLE IF NOT EXISTS call_approval_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  name text NOT NULL,
  auto_approve_conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
  require_approval_conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
  deny_conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
  max_daily_calls int NOT NULL DEFAULT 50,
  max_call_duration_seconds int NOT NULL DEFAULT 1800,
  allowed_hours_start time,
  allowed_hours_end time,
  allowed_regions text[] NOT NULL DEFAULT '{}',
  blocked_numbers text[] NOT NULL DEFAULT '{}',
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, name)
);

ALTER TABLE call_approval_policies ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_approval_policies_workspace_isolation ON call_approval_policies
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 3: call_approval_requests
CREATE TABLE IF NOT EXISTS call_approval_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  receipt_id uuid REFERENCES authorization_receipts(id),
  policy_id uuid NOT NULL REFERENCES call_approval_policies(id),
  caller_context jsonb NOT NULL DEFAULT '{}'::jsonb,
  target_number_hash text NOT NULL,
  target_region text NOT NULL DEFAULT '',
  purpose text NOT NULL,
  status call_approval_status NOT NULL DEFAULT 'pending',
  decided_by uuid,
  decided_at timestamptz,
  decision_reason text,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE call_approval_requests ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_approval_requests_workspace_isolation ON call_approval_requests
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 4: calls (main call record — NO raw audio, transcript only)
CREATE TABLE IF NOT EXISTS calls (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  approval_request_id uuid NOT NULL REFERENCES call_approval_requests(id),
  provider_id uuid NOT NULL REFERENCES call_providers(id),
  direction call_direction NOT NULL DEFAULT 'outbound',
  status call_status NOT NULL DEFAULT 'pending_approval',
  target_number_hash text NOT NULL,
  started_at timestamptz,
  answered_at timestamptz,
  ended_at timestamptz,
  duration_seconds int NOT NULL DEFAULT 0,
  provider_call_id text,
  failover_count int NOT NULL DEFAULT 0,
  failover_from uuid REFERENCES call_providers(id),
  cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE calls ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY calls_workspace_isolation ON calls
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 5: call_transcripts (transcript-only persistence)
CREATE TABLE IF NOT EXISTS call_transcripts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  call_id uuid NOT NULL REFERENCES calls(id),
  segment_index int NOT NULL,
  segment_type transcript_segment_type NOT NULL,
  speaker text NOT NULL DEFAULT '',
  content text NOT NULL,
  started_at_ms int NOT NULL DEFAULT 0,
  duration_ms int NOT NULL DEFAULT 0,
  confidence numeric(5,4) NOT NULL DEFAULT 1.0,
  language text NOT NULL DEFAULT 'en',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE call_transcripts ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_transcripts_workspace_isolation ON call_transcripts
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 6: call_events (lifecycle events for observability)
CREATE TABLE IF NOT EXISTS call_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  call_id uuid NOT NULL REFERENCES calls(id),
  event_type text NOT NULL,
  event_data jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE call_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_events_workspace_isolation ON call_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 7: call_provider_health_log
CREATE TABLE IF NOT EXISTS call_provider_health_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provider_id uuid NOT NULL REFERENCES call_providers(id),
  health_score numeric(5,4) NOT NULL,
  latency_ms int NOT NULL DEFAULT 0,
  error_count int NOT NULL DEFAULT 0,
  success_count int NOT NULL DEFAULT 0,
  check_type text NOT NULL DEFAULT 'periodic',
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE call_provider_health_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_provider_health_log_workspace_isolation ON call_provider_health_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 8: call_rate_limits
CREATE TABLE IF NOT EXISTS call_rate_limits (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  window_start timestamptz NOT NULL,
  window_end timestamptz NOT NULL,
  call_count int NOT NULL DEFAULT 0,
  max_calls int NOT NULL DEFAULT 50,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, window_start)
);

ALTER TABLE call_rate_limits ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_rate_limits_workspace_isolation ON call_rate_limits
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 9: call_number_blocklist
CREATE TABLE IF NOT EXISTS call_number_blocklist (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  number_hash text NOT NULL,
  reason text NOT NULL DEFAULT '',
  blocked_by uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, number_hash)
);

ALTER TABLE call_number_blocklist ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY call_number_blocklist_workspace_isolation ON call_number_blocklist
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Indexes for V10.4
CREATE INDEX IF NOT EXISTS idx_call_providers_workspace ON call_providers(workspace_id);
CREATE INDEX IF NOT EXISTS idx_call_providers_status ON call_providers(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_call_approval_requests_workspace ON call_approval_requests(workspace_id);
CREATE INDEX IF NOT EXISTS idx_call_approval_requests_status ON call_approval_requests(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_calls_workspace ON calls(workspace_id);
CREATE INDEX IF NOT EXISTS idx_calls_status ON calls(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_calls_approval ON calls(approval_request_id);
CREATE INDEX IF NOT EXISTS idx_call_transcripts_call ON call_transcripts(call_id, segment_index);
CREATE INDEX IF NOT EXISTS idx_call_events_call ON call_events(call_id, created_at);
CREATE INDEX IF NOT EXISTS idx_call_provider_health_provider ON call_provider_health_log(provider_id, created_at);
CREATE INDEX IF NOT EXISTS idx_call_rate_limits_workspace ON call_rate_limits(workspace_id, window_start);
CREATE INDEX IF NOT EXISTS idx_call_number_blocklist_workspace ON call_number_blocklist(workspace_id);
