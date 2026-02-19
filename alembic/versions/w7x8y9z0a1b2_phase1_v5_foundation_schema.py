"""phase 1 v5 foundation schema

Revision ID: w7x8y9z0a1b2
Revises: v6f7g8h9i0j1
Create Date: 2026-02-16
"""

from alembic import op


# revision identifiers, used by Alembic.
revision = "w7x8y9z0a1b2"
down_revision = "v6f7g8h9i0j1"
branch_labels = None
depends_on = None


def _postgres_only_sql() -> str:
    return """
-- Extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Enums
DO $$ BEGIN
    CREATE TYPE channel_type AS ENUM ('whatsapp', 'imessage', 'slack', 'web', 'voice');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE message_direction AS ENUM ('inbound', 'outbound');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE input_modality AS ENUM ('text', 'voice', 'image', 'document', 'location');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE run_state AS ENUM (
      'pending', 'classifying', 'planning', 'executing',
      'awaiting_approval', 'completed', 'failed', 'cancelled',
      'delegated', 'researching'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE memory_type AS ENUM ('preference', 'fact', 'contact', 'procedure', 'episode');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE sensitivity_level AS ENUM ('public', 'private', 'sensitive');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'denied', 'expired');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE tool_exec_status AS ENUM ('success', 'failed', 'timeout', 'compensated');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE risk_level AS ENUM ('none', 'low', 'medium', 'high', 'critical');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE account_status AS ENUM ('active', 'expired', 'revoked');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE trigger_type AS ENUM ('schedule', 'event', 'pattern', 'goal', 'research', 'workflow', 'delegation');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE emotion_state AS ENUM ('neutral', 'positive', 'frustrated', 'rushed', 'stressed', 'excited');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE llm_provider AS ENUM ('openai', 'anthropic', 'google', 'local', 'mcp');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE bi_layer AS ENUM ('brain', 'dna', 'bones', 'muscles');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE profiling_status AS ENUM ('pending', 'active', 'completed', 'skipped');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE feedback_type AS ENUM ('correction', 'override', 'edit', 'praise', 'complaint', 'restyle');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE rule_source AS ENUM ('explicit', 'inferred', 'correction', 'default');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE goal_status AS ENUM ('active', 'completed', 'paused', 'abandoned');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE goal_timeframe AS ENUM ('this_week', 'this_month', 'this_quarter', 'this_year', '3_year');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE delegation_status AS ENUM ('pending', 'sent', 'acknowledged', 'in_progress', 'completed', 'overdue', 'cancelled');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE research_status AS ENUM ('active', 'paused', 'completed', 'archived');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE workflow_status AS ENUM ('active', 'paused', 'draft', 'archived');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE workflow_trigger_type AS ENUM ('schedule', 'event', 'condition', 'manual');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE content_provenance AS ENUM ('user_direct', 'email_body', 'web_search', 'calendar_desc', 'document', 'mcp_result');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'content_provenance' AND e.enumlabel = 'mcp_result'
    ) THEN
        ALTER TYPE content_provenance ADD VALUE 'mcp_result';
    END IF;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Core v5 tables
-- Canonical identity (accounts) + compatibility identity (users) must exist in baseline migration.
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ DEFAULT now(),
  deletion_requested_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  clerk_user_id TEXT UNIQUE,
  display_name TEXT,
  timezone TEXT NOT NULL DEFAULT 'America/New_York',
  status account_status NOT NULL DEFAULT 'active',
  preferred_channel channel_type DEFAULT 'whatsapp',
  quiet_hours_start TIME,
  quiet_hours_end TIME,
  max_daily_proactive INT DEFAULT 5,
  onboarding_completed_at TIMESTAMPTZ,
  emotion_sensitivity FLOAT DEFAULT 0.5,
  team_enabled BOOLEAN DEFAULT false,
  voice_enabled BOOLEAN DEFAULT false,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS channel_connections (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  channel channel_type NOT NULL,
  channel_identifier TEXT NOT NULL,
  is_primary BOOLEAN DEFAULT false,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(channel, channel_identifier)
);

-- Operational Month-1 schema (billing baseline) lives in this same migration.
CREATE TABLE IF NOT EXISTS subscriptions (
  user_id TEXT PRIMARY KEY REFERENCES users(id),
  plan TEXT NOT NULL DEFAULT 'free',
  status TEXT NOT NULL DEFAULT 'active',
  provider TEXT,
  provider_customer_id TEXT,
  provider_subscription_id TEXT,
  current_period_end TIMESTAMPTZ,
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_plan_status ON subscriptions(plan, status);

CREATE TABLE IF NOT EXISTS invoices (
  id BIGSERIAL PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  provider TEXT NOT NULL DEFAULT 'stripe',
  provider_invoice_id TEXT NOT NULL UNIQUE,
  provider_customer_id TEXT,
  provider_subscription_id TEXT,
  status TEXT NOT NULL DEFAULT 'open',
  amount_due INT,
  amount_paid INT,
  currency TEXT,
  hosted_invoice_url TEXT,
  invoice_pdf_url TEXT,
  paid_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_invoices_user_status_created ON invoices(user_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  trigger_message_id UUID,
  intent TEXT,
  tier INT DEFAULT 1,
  state run_state DEFAULT 'pending',
  llm_provider llm_provider DEFAULT 'openai',
  context_tokens_used INT DEFAULT 0,
  knowledge_files_injected TEXT[] DEFAULT '{}',
  plan JSONB,
  result JSONB,
  error JSONB,
  parent_run_id UUID,
  delegation_id UUID,
  research_job_id UUID,
  cost_cents FLOAT DEFAULT 0,
  latency_ms INT,
  created_at TIMESTAMPTZ DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  run_id UUID REFERENCES runs(id),
  channel channel_type,
  direction message_direction,
  content TEXT,
  input_modality input_modality DEFAULT 'text',
  original_media_url TEXT,
  transcription_confidence FLOAT,
  extracted_entities JSONB DEFAULT '{}',
  content_provenance content_provenance DEFAULT 'user_direct',
  emotion_detected emotion_state DEFAULT 'neutral',
  wa_message_id TEXT,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_messages_user_created ON messages(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_run ON messages(run_id);
CREATE INDEX IF NOT EXISTS idx_runs_user_state ON runs(user_id, state);

CREATE TABLE IF NOT EXISTS memories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  run_id UUID REFERENCES runs(id),
  memory_type memory_type,
  content TEXT,
  embedding vector(1536),
  sensitivity sensitivity_level DEFAULT 'private',
  confidence FLOAT DEFAULT 0.8,
  source TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS tool_executions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID REFERENCES runs(id),
  user_id UUID REFERENCES accounts(id),
  tool_name TEXT,
  is_mcp BOOLEAN DEFAULT false,
  mcp_server_id TEXT,
  arguments JSONB DEFAULT '{}',
  input_provenance content_provenance DEFAULT 'user_direct',
  result JSONB,
  status tool_exec_status DEFAULT 'success',
  risk_level risk_level DEFAULT 'none',
  approval_status approval_status,
  idempotency_key TEXT UNIQUE,
  compensating_action JSONB,
  cost_cents FLOAT DEFAULT 0,
  latency_ms INT,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS oauth_tokens (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  provider TEXT,
  encrypted_access_token TEXT,
  encrypted_refresh_token TEXT,
  token_expiry TIMESTAMPTZ,
  scopes TEXT[] DEFAULT '{}',
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, provider)
);

CREATE TABLE IF NOT EXISTS proactive_triggers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  trigger_type trigger_type,
  source TEXT,
  payload JSONB DEFAULT '{}',
  confidence_threshold FLOAT DEFAULT 0.7,
  fire_at TIMESTAMPTZ,
  fired_at TIMESTAMPTZ,
  dismissed BOOLEAN DEFAULT false,
  fire_count INT DEFAULT 0,
  user_dismissed_count INT DEFAULT 0,
  workflow_id UUID,
  delegation_id UUID,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sync_cursors (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  provider TEXT,
  resource_type TEXT,
  cursor_value TEXT,
  last_sync_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, provider, resource_type)
);

CREATE TABLE IF NOT EXISTS side_effects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID REFERENCES runs(id),
  user_id UUID REFERENCES accounts(id),
  effect_type TEXT,
  description TEXT,
  reversible BOOLEAN DEFAULT false,
  reversed_at TIMESTAMPTZ,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge_files (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  file_path TEXT,
  layer bi_layer,
  content TEXT,
  content_hash TEXT,
  token_count INT DEFAULT 0,
  version INT DEFAULT 1,
  metadata JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, file_path, version)
);
CREATE INDEX IF NOT EXISTS idx_kf_user_path ON knowledge_files(user_id, file_path, version DESC);

CREATE TABLE IF NOT EXISTS profiling_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  dimension TEXT,
  layer bi_layer DEFAULT 'brain',
  status profiling_status DEFAULT 'pending',
  questions_asked INT DEFAULT 0,
  facts_extracted INT DEFAULT 0,
  progress_pct FLOAT DEFAULT 0,
  scheduled_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS behavioral_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  file_path TEXT,
  category TEXT,
  rule_key TEXT,
  rule_value TEXT,
  source rule_source DEFAULT 'default',
  confidence FLOAT DEFAULT 0.5,
  override_count INT DEFAULT 0,
  last_triggered_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, file_path, category, rule_key)
);

CREATE TABLE IF NOT EXISTS feedback_signals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  run_id UUID REFERENCES runs(id),
  signal_type feedback_type,
  original_output TEXT,
  corrected_output TEXT,
  context JSONB DEFAULT '{}',
  lesson_extracted JSONB,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_feedback_user ON feedback_signals(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS goals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  title TEXT,
  description TEXT,
  timeframe goal_timeframe,
  status goal_status DEFAULT 'active',
  progress_pct FLOAT DEFAULT 0,
  milestones JSONB DEFAULT '[]',
  blockers JSONB DEFAULT '[]',
  target_date DATE,
  last_activity_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS delegations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  delegate_name TEXT,
  delegate_contact TEXT,
  delegate_channel channel_type,
  task_description TEXT,
  context JSONB DEFAULT '{}',
  status delegation_status DEFAULT 'pending',
  priority risk_level DEFAULT 'medium',
  deadline TIMESTAMPTZ,
  reminder_schedule JSONB DEFAULT '[]',
  last_reminder_at TIMESTAMPTZ,
  completion_criteria TEXT,
  result TEXT,
  linked_run_id UUID REFERENCES runs(id),
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_delegations_user_status ON delegations(user_id, status);

CREATE TABLE IF NOT EXISTS research_jobs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  title TEXT,
  query TEXT,
  sources TEXT[] DEFAULT '{}',
  schedule TEXT,
  status research_status DEFAULT 'active',
  last_run_at TIMESTAMPTZ,
  next_run_at TIMESTAMPTZ,
  findings JSONB DEFAULT '[]',
  delivery_channel channel_type,
  delivery_format TEXT DEFAULT 'summary',
  max_cost_per_run FLOAT DEFAULT 0.50,
  total_cost FLOAT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflows (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  name TEXT,
  description TEXT,
  trigger_type workflow_trigger_type,
  trigger_config JSONB DEFAULT '{}',
  actions JSONB DEFAULT '[]',
  status workflow_status DEFAULT 'active',
  last_run_at TIMESTAMPTZ,
  next_run_at TIMESTAMPTZ,
  run_count INT DEFAULT 0,
  error_count INT DEFAULT 0,
  last_error JSONB,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS knowledge_graph_edges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID REFERENCES accounts(id),
  source_type TEXT,
  source_id TEXT,
  source_label TEXT,
  relationship TEXT,
  target_type TEXT,
  target_id TEXT,
  target_label TEXT,
  weight FLOAT DEFAULT 1.0,
  metadata JSONB DEFAULT '{}',
  confidence FLOAT DEFAULT 0.8,
  source_file TEXT,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_kg_user_source ON knowledge_graph_edges(user_id, source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_kg_user_target ON knowledge_graph_edges(user_id, target_type, target_id);
CREATE INDEX IF NOT EXISTS idx_kg_relationship ON knowledge_graph_edges(user_id, relationship);

-- Enhancement columns on pre-existing legacy tables (if they exist)
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS input_modality input_modality DEFAULT 'text';
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS original_media_url TEXT;
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS transcription_confidence FLOAT;
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS extracted_entities JSONB DEFAULT '{}';
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS content_provenance content_provenance DEFAULT 'user_direct';
ALTER TABLE IF EXISTS messages ADD COLUMN IF NOT EXISTS emotion_detected emotion_state DEFAULT 'neutral';

ALTER TABLE IF EXISTS runs ADD COLUMN IF NOT EXISTS llm_provider llm_provider DEFAULT 'openai';
ALTER TABLE IF EXISTS runs ADD COLUMN IF NOT EXISTS knowledge_files_injected TEXT[] DEFAULT '{}';
ALTER TABLE IF EXISTS runs ADD COLUMN IF NOT EXISTS delegation_id UUID;
ALTER TABLE IF EXISTS runs ADD COLUMN IF NOT EXISTS research_job_id UUID;

ALTER TABLE IF EXISTS tool_executions ADD COLUMN IF NOT EXISTS is_mcp BOOLEAN DEFAULT false;
ALTER TABLE IF EXISTS tool_executions ADD COLUMN IF NOT EXISTS mcp_server_id TEXT;
ALTER TABLE IF EXISTS tool_executions ADD COLUMN IF NOT EXISTS input_provenance content_provenance DEFAULT 'user_direct';
ALTER TABLE IF EXISTS tool_executions ADD COLUMN IF NOT EXISTS compensating_action JSONB;

ALTER TABLE IF EXISTS proactive_triggers ADD COLUMN IF NOT EXISTS workflow_id UUID;
ALTER TABLE IF EXISTS proactive_triggers ADD COLUMN IF NOT EXISTS delegation_id UUID;

ALTER TABLE IF EXISTS accounts ADD COLUMN IF NOT EXISTS emotion_sensitivity FLOAT DEFAULT 0.5;
ALTER TABLE IF EXISTS accounts ADD COLUMN IF NOT EXISTS team_enabled BOOLEAN DEFAULT false;
ALTER TABLE IF EXISTS accounts ADD COLUMN IF NOT EXISTS voice_enabled BOOLEAN DEFAULT false;

-- RLS baseline (Section 3 / Section 32)
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS accounts_self_access ON accounts;
CREATE POLICY accounts_self_access ON accounts
  USING (id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE channel_connections ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS channel_connections_user_access ON channel_connections;
CREATE POLICY channel_connections_user_access ON channel_connections
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE runs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS runs_user_access ON runs;
CREATE POLICY runs_user_access ON runs
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS messages_user_access ON messages;
CREATE POLICY messages_user_access ON messages
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS memories_user_access ON memories;
CREATE POLICY memories_user_access ON memories
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE tool_executions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tool_executions_user_access ON tool_executions;
CREATE POLICY tool_executions_user_access ON tool_executions
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE oauth_tokens ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS oauth_tokens_user_access ON oauth_tokens;
CREATE POLICY oauth_tokens_user_access ON oauth_tokens
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE proactive_triggers ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS proactive_triggers_user_access ON proactive_triggers;
CREATE POLICY proactive_triggers_user_access ON proactive_triggers
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE sync_cursors ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS sync_cursors_user_access ON sync_cursors;
CREATE POLICY sync_cursors_user_access ON sync_cursors
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE side_effects ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS side_effects_user_access ON side_effects;
CREATE POLICY side_effects_user_access ON side_effects
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE knowledge_files ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS knowledge_files_user_access ON knowledge_files;
CREATE POLICY knowledge_files_user_access ON knowledge_files
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE profiling_sessions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS profiling_sessions_user_access ON profiling_sessions;
CREATE POLICY profiling_sessions_user_access ON profiling_sessions
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE behavioral_rules ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS behavioral_rules_user_access ON behavioral_rules;
CREATE POLICY behavioral_rules_user_access ON behavioral_rules
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE feedback_signals ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS feedback_signals_user_access ON feedback_signals;
CREATE POLICY feedback_signals_user_access ON feedback_signals
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE goals ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS goals_user_access ON goals;
CREATE POLICY goals_user_access ON goals
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE delegations ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS delegations_user_access ON delegations;
CREATE POLICY delegations_user_access ON delegations
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE research_jobs ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS research_jobs_user_access ON research_jobs;
CREATE POLICY research_jobs_user_access ON research_jobs
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS workflows_user_access ON workflows;
CREATE POLICY workflows_user_access ON workflows
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);

ALTER TABLE knowledge_graph_edges ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS knowledge_graph_edges_user_access ON knowledge_graph_edges;
CREATE POLICY knowledge_graph_edges_user_access ON knowledge_graph_edges
  USING (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid)
  WITH CHECK (user_id = NULLIF(current_setting('app.user_id', true), '')::uuid);
    """


def upgrade():
    bind = op.get_bind()
    if bind.dialect.name != "postgresql":
        # This blueprint migration is PostgreSQL-specific (enum types, vector, RLS-oriented schema).
        return
    op.execute(_postgres_only_sql())


def downgrade():
    # Intentional no-op: this migration introduces foundational blueprint schema assets.
    # Rolling back would risk deleting production user data.
    pass
