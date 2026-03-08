-- BREVIO OpenClaw adoption migration (BP16/BP17)
-- Implements: hooks, DM pairing, A2A, queue lanes, sandbox configs, skills gate,
-- idempotency, compaction, auth profile rotation, bootstrap hook, NO_REPLY, doctor state.

-- OpenClaw adoption enums
DO $$ BEGIN
  CREATE TYPE hook_type AS ENUM (
    'pre_plan','post_plan','pre_execute','post_execute',
    'pre_commit','post_commit','on_error','on_compensation',
    'bootstrap','shutdown'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE hook_status AS ENUM ('active','disabled','failed','pending_approval');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE queue_lane AS ENUM ('critical','high','normal','low','background','batch');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE sandbox_profile AS ENUM ('strict','standard','permissive','custom');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE a2a_message_status AS ENUM (
    'pending','delivered','acknowledged','failed','expired'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE auth_profile_status AS ENUM (
    'active','rotating','expired','revoked'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Pipeline hooks
CREATE TABLE IF NOT EXISTS pipeline_hooks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  hook_type hook_type NOT NULL,
  name text NOT NULL,
  handler_url text NOT NULL DEFAULT '',
  handler_script text NOT NULL DEFAULT '',
  priority int NOT NULL DEFAULT 0,
  status hook_status NOT NULL DEFAULT 'active',
  timeout_ms int NOT NULL DEFAULT 5000,
  retry_count int NOT NULL DEFAULT 0,
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, hook_type, name)
);

ALTER TABLE pipeline_hooks ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY pipeline_hooks_workspace_isolation ON pipeline_hooks
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Hook execution log
CREATE TABLE IF NOT EXISTS hook_execution_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  hook_id uuid NOT NULL REFERENCES pipeline_hooks(id),
  workflow_run_id text NOT NULL DEFAULT '',
  status text NOT NULL,
  duration_ms int NOT NULL DEFAULT 0,
  input_hash text NOT NULL DEFAULT '',
  output_summary text NOT NULL DEFAULT '',
  error_message text,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE hook_execution_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY hook_execution_log_workspace_isolation ON hook_execution_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- A2A (Agent-to-Agent) messages
CREATE TABLE IF NOT EXISTS a2a_messages (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  sender_agent_id uuid NOT NULL,
  receiver_agent_id uuid NOT NULL,
  message_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  status a2a_message_status NOT NULL DEFAULT 'pending',
  priority queue_lane NOT NULL DEFAULT 'normal',
  idempotency_key text NOT NULL,
  expires_at timestamptz,
  delivered_at timestamptz,
  acknowledged_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, idempotency_key)
);

ALTER TABLE a2a_messages ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY a2a_messages_workspace_isolation ON a2a_messages
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Queue lane configuration
CREATE TABLE IF NOT EXISTS queue_lane_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  lane queue_lane NOT NULL,
  max_concurrent int NOT NULL DEFAULT 10,
  rate_limit_per_second int NOT NULL DEFAULT 100,
  priority_weight int NOT NULL DEFAULT 1,
  timeout_seconds int NOT NULL DEFAULT 300,
  retry_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, lane)
);

ALTER TABLE queue_lane_config ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY queue_lane_config_workspace_isolation ON queue_lane_config
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Sandbox configurations
CREATE TABLE IF NOT EXISTS sandbox_configs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  profile sandbox_profile NOT NULL DEFAULT 'standard',
  name text NOT NULL,
  allowed_domains text[] NOT NULL DEFAULT '{}',
  blocked_domains text[] NOT NULL DEFAULT '{}',
  max_memory_mb int NOT NULL DEFAULT 256,
  max_cpu_seconds int NOT NULL DEFAULT 30,
  network_access boolean NOT NULL DEFAULT false,
  filesystem_access boolean NOT NULL DEFAULT false,
  custom_rules jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, name)
);

ALTER TABLE sandbox_configs ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY sandbox_configs_workspace_isolation ON sandbox_configs
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Skills gate decisions (extends execution_gate_decisions for skill-specific gating)
CREATE TABLE IF NOT EXISTS skills_gate_decisions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  skill_key text NOT NULL,
  decision gate_decision NOT NULL,
  reason text NOT NULL DEFAULT '',
  policy_version text NOT NULL DEFAULT '',
  context jsonb NOT NULL DEFAULT '{}'::jsonb,
  receipt_id uuid REFERENCES authorization_receipts(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE skills_gate_decisions ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY skills_gate_decisions_workspace_isolation ON skills_gate_decisions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Auth profile rotation
CREATE TABLE IF NOT EXISTS auth_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  profile_name text NOT NULL,
  status auth_profile_status NOT NULL DEFAULT 'active',
  credentials_encrypted text NOT NULL,
  provider text NOT NULL,
  rotated_at timestamptz,
  rotation_schedule_cron text,
  next_rotation_at timestamptz,
  rotation_count int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, profile_name)
);

ALTER TABLE auth_profiles ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY auth_profiles_workspace_isolation ON auth_profiles
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Context compaction log
CREATE TABLE IF NOT EXISTS context_compaction_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  session_id uuid,
  original_token_count int NOT NULL,
  compacted_token_count int NOT NULL,
  compression_ratio numeric(5,4) NOT NULL,
  strategy text NOT NULL DEFAULT 'summarize',
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE context_compaction_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY context_compaction_log_workspace_isolation ON context_compaction_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- NO_REPLY configuration
CREATE TABLE IF NOT EXISTS no_reply_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  channel_type text NOT NULL,
  patterns jsonb NOT NULL DEFAULT '[]'::jsonb,
  auto_ack_message text NOT NULL DEFAULT '',
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, channel_type)
);

ALTER TABLE no_reply_config ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY no_reply_config_workspace_isolation ON no_reply_config
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Doctor CLI state snapshots
CREATE TABLE IF NOT EXISTS doctor_snapshots (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  check_results jsonb NOT NULL DEFAULT '{}'::jsonb,
  overall_status text NOT NULL DEFAULT 'unknown',
  checks_passed int NOT NULL DEFAULT 0,
  checks_failed int NOT NULL DEFAULT 0,
  checks_warned int NOT NULL DEFAULT 0,
  run_by text NOT NULL DEFAULT 'cli',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- DM pairing enhanced config
CREATE TABLE IF NOT EXISTS dm_pairing_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) UNIQUE,
  require_pairing boolean NOT NULL DEFAULT true,
  pairing_timeout_minutes int NOT NULL DEFAULT 60,
  max_delegates_per_owner int NOT NULL DEFAULT 5,
  auto_expire_days int NOT NULL DEFAULT 90,
  notification_channels text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE dm_pairing_config ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY dm_pairing_config_workspace_isolation ON dm_pairing_config
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Indexes for OpenClaw adoption
CREATE INDEX IF NOT EXISTS idx_pipeline_hooks_workspace ON pipeline_hooks(workspace_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_hooks_type ON pipeline_hooks(workspace_id, hook_type);
CREATE INDEX IF NOT EXISTS idx_hook_execution_log_workspace ON hook_execution_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_hook_execution_log_hook ON hook_execution_log(hook_id, created_at);
CREATE INDEX IF NOT EXISTS idx_a2a_messages_workspace ON a2a_messages(workspace_id);
CREATE INDEX IF NOT EXISTS idx_a2a_messages_receiver ON a2a_messages(receiver_agent_id, status);
CREATE INDEX IF NOT EXISTS idx_a2a_messages_sender ON a2a_messages(sender_agent_id, created_at);
CREATE INDEX IF NOT EXISTS idx_queue_lane_config_workspace ON queue_lane_config(workspace_id);
CREATE INDEX IF NOT EXISTS idx_sandbox_configs_workspace ON sandbox_configs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_skills_gate_decisions_workspace ON skills_gate_decisions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_skills_gate_decisions_skill ON skills_gate_decisions(workspace_id, skill_key, created_at);
CREATE INDEX IF NOT EXISTS idx_auth_profiles_workspace ON auth_profiles(workspace_id);
CREATE INDEX IF NOT EXISTS idx_auth_profiles_rotation ON auth_profiles(next_rotation_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_context_compaction_log_workspace ON context_compaction_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_no_reply_config_workspace ON no_reply_config(workspace_id);
CREATE INDEX IF NOT EXISTS idx_doctor_snapshots_workspace ON doctor_snapshots(workspace_id);
CREATE INDEX IF NOT EXISTS idx_dm_pairing_config_workspace ON dm_pairing_config(workspace_id);
