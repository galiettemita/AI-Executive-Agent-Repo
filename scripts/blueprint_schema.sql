-- Executive OS Blueprint 2026 schema (from BLUEPRINT.pdf)

-- Extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- Enums
CREATE TYPE channel_type AS ENUM ('whatsapp', 'imessage', 'web');
CREATE TYPE message_direction AS ENUM ('inbound', 'outbound');
CREATE TYPE run_state AS ENUM (
  'pending',
  'classifying',
  'planning',
  'executing',
  'awaiting_approval',
  'completed',
  'failed',
  'cancelled'
);
CREATE TYPE memory_type AS ENUM ('preference', 'fact', 'contact', 'procedure', 'episode');
CREATE TYPE sensitivity_level AS ENUM ('public', 'private', 'sensitive');
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'denied', 'expired');
CREATE TYPE tool_exec_status AS ENUM ('success', 'failed', 'timeout', 'compensated');
CREATE TYPE risk_level AS ENUM ('none', 'low', 'medium', 'high', 'critical');
CREATE TYPE account_status AS ENUM ('active', 'expired', 'revoked');
CREATE TYPE trigger_type AS ENUM ('schedule', 'event', 'pattern');

-- Core tables
CREATE TABLE users (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  phone_number TEXT UNIQUE NOT NULL,
  display_name TEXT,
  email TEXT,
  timezone TEXT DEFAULT 'America/New_York',
  locale TEXT DEFAULT 'en-US',
  work_hours JSONB DEFAULT '{"start":"09:00","end":"17:00","days":[1,2,3,4,5]}',
  quiet_hours JSONB DEFAULT '{"start":"22:00","end":"07:00"}',
  channels JSONB DEFAULT '{}',
  onboarding_stage INT DEFAULT 0,
  autonomy_profile JSONB DEFAULT '{}',
  preferences JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_users_phone ON users(phone_number);
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
CREATE POLICY users_isolation ON users USING (id = current_setting('app.user_id')::uuid);

CREATE TABLE conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  channel channel_type NOT NULL,
  state JSONB DEFAULT '{}',
  summary TEXT,
  started_at TIMESTAMPTZ DEFAULT now(),
  last_active_at TIMESTAMPTZ DEFAULT now(),
  is_active BOOLEAN DEFAULT true
);
CREATE INDEX idx_conversations_user ON conversations(user_id, last_active_at DESC);
CREATE INDEX idx_conversations_active ON conversations(user_id, is_active) WHERE is_active = true;
ALTER TABLE conversations ENABLE ROW LEVEL SECURITY;
CREATE POLICY convos_isolation ON conversations USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE messages (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id),
  direction message_direction NOT NULL,
  content JSONB NOT NULL,
  intent TEXT,
  tier INT,
  run_id UUID,
  cost_cents INT DEFAULT 0,
  latency_ms INT,
  channel_msg_id TEXT,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_messages_convo ON messages(conversation_id, created_at);
CREATE INDEX idx_messages_user ON messages(user_id, created_at DESC);
CREATE INDEX idx_messages_run ON messages(run_id) WHERE run_id IS NOT NULL;
CREATE UNIQUE INDEX idx_messages_channel_dedup ON messages(channel_msg_id) WHERE channel_msg_id IS NOT NULL;
ALTER TABLE messages ENABLE ROW LEVEL SECURITY;
CREATE POLICY messages_isolation ON messages USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  conversation_id UUID NOT NULL REFERENCES conversations(id),
  parent_run_id UUID REFERENCES runs(id),
  intent TEXT NOT NULL,
  tier INT NOT NULL DEFAULT 1,
  plan JSONB DEFAULT '[]',
  state run_state NOT NULL DEFAULT 'pending',
  envelope JSONB NOT NULL,
  error JSONB,
  total_cost_cents INT DEFAULT 0,
  total_latency_ms INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT now(),
  completed_at TIMESTAMPTZ
);
CREATE INDEX idx_runs_user_state ON runs(user_id, state);
CREATE INDEX idx_runs_convo ON runs(conversation_id);
CREATE INDEX idx_runs_parent ON runs(parent_run_id) WHERE parent_run_id IS NOT NULL;
ALTER TABLE runs ENABLE ROW LEVEL SECURITY;
CREATE POLICY runs_isolation ON runs USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE memories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type memory_type NOT NULL,
  category TEXT NOT NULL,
  key TEXT NOT NULL,
  content JSONB NOT NULL,
  embedding vector(1536),
  confidence FLOAT DEFAULT 0.8 CHECK (confidence BETWEEN 0.0 AND 1.0),
  sensitivity sensitivity_level DEFAULT 'private',
  valid_from TIMESTAMPTZ DEFAULT now(),
  valid_until TIMESTAMPTZ,
  source_run_id UUID REFERENCES runs(id),
  version INT DEFAULT 1,
  is_active BOOLEAN DEFAULT true,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, key, version)
);
CREATE INDEX idx_memories_embedding ON memories USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_memories_user_type ON memories(user_id, type, category) WHERE is_active = true;
CREATE INDEX idx_memories_user_key ON memories(user_id, key) WHERE is_active = true;
ALTER TABLE memories ENABLE ROW LEVEL SECURITY;
CREATE POLICY memories_isolation ON memories USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE contacts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  email TEXT,
  phone TEXT,
  relationship TEXT,
  organization TEXT,
  title TEXT,
  trust_score FLOAT DEFAULT 0.5 CHECK (trust_score BETWEEN 0.0 AND 1.0),
  interaction_count INT DEFAULT 0,
  metadata JSONB DEFAULT '{}',
  last_interacted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_contacts_user ON contacts(user_id);
CREATE INDEX idx_contacts_name_trgm ON contacts USING gin (name gin_trgm_ops);
CREATE INDEX idx_contacts_email ON contacts(user_id, email) WHERE email IS NOT NULL;
CREATE INDEX idx_contacts_trust ON contacts(user_id, trust_score DESC);
ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
CREATE POLICY contacts_isolation ON contacts USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE approvals (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES runs(id),
  user_id UUID NOT NULL REFERENCES users(id),
  action_summary TEXT NOT NULL,
  risk_explanation TEXT NOT NULL,
  proposed_action JSONB NOT NULL,
  alternatives JSONB DEFAULT '[]',
  status approval_status DEFAULT 'pending',
  channel channel_type NOT NULL,
  channel_msg_id TEXT,
  responded_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_approvals_user_pending ON approvals(user_id, status) WHERE status = 'pending';
CREATE INDEX idx_approvals_run ON approvals(run_id);
ALTER TABLE approvals ENABLE ROW LEVEL SECURITY;
CREATE POLICY approvals_isolation ON approvals USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE tool_executions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  run_id UUID NOT NULL REFERENCES runs(id),
  user_id UUID NOT NULL REFERENCES users(id),
  tool_name TEXT NOT NULL,
  input_hash TEXT NOT NULL,
  input JSONB NOT NULL,
  output JSONB,
  status tool_exec_status NOT NULL DEFAULT 'success',
  error JSONB,
  idempotency_key TEXT UNIQUE NOT NULL,
  risk_level risk_level NOT NULL DEFAULT 'none',
  approved_by UUID REFERENCES approvals(id),
  cost_cents INT DEFAULT 0,
  latency_ms INT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_tool_exec_run ON tool_executions(run_id);
CREATE INDEX idx_tool_exec_user ON tool_executions(user_id, created_at DESC);
CREATE INDEX idx_tool_exec_idemp ON tool_executions(idempotency_key);
ALTER TABLE tool_executions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tool_exec_isolation ON tool_executions USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE connected_accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  provider TEXT NOT NULL,
  provider_account_id TEXT,
  access_token_encrypted BYTEA NOT NULL,
  refresh_token_encrypted BYTEA,
  token_expires_at TIMESTAMPTZ,
  scopes TEXT[] NOT NULL,
  sync_cursors JSONB DEFAULT '{}',
  last_synced_at TIMESTAMPTZ,
  status account_status DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  UNIQUE(user_id, provider)
);
ALTER TABLE connected_accounts ENABLE ROW LEVEL SECURITY;
CREATE POLICY accounts_isolation ON connected_accounts USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE proactive_triggers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type trigger_type NOT NULL,
  name TEXT NOT NULL,
  trigger_condition JSONB NOT NULL,
  action_template JSONB NOT NULL,
  confidence_threshold FLOAT DEFAULT 0.7,
  enabled BOOLEAN DEFAULT true,
  max_daily_fires INT DEFAULT 3,
  last_fired_at TIMESTAMPTZ,
  fire_count INT DEFAULT 0,
  success_count INT DEFAULT 0,
  user_dismissed_count INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_triggers_user ON proactive_triggers(user_id, enabled) WHERE enabled = true;
ALTER TABLE proactive_triggers ENABLE ROW LEVEL SECURITY;
CREATE POLICY triggers_isolation ON proactive_triggers USING (user_id = current_setting('app.user_id')::uuid);

CREATE TABLE semantic_cache (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id),
  query_embedding vector(1536) NOT NULL,
  query_text TEXT NOT NULL,
  response JSONB NOT NULL,
  model TEXT NOT NULL,
  tier INT NOT NULL,
  hit_count INT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_sem_cache_embedding ON semantic_cache USING hnsw (query_embedding vector_cosine_ops) WITH (m = 16, ef_construction = 64);
CREATE INDEX idx_sem_cache_expiry ON semantic_cache(expires_at);
ALTER TABLE semantic_cache ENABLE ROW LEVEL SECURITY;
CREATE POLICY sem_cache_isolation ON semantic_cache USING (user_id = current_setting('app.user_id')::uuid);
