-- BREVIO V10 gap closure migration (BP06)
-- Implements: federation, wallet, negotiation, cost tracking, capability registry
-- Forward-only. All tables use UUIDv7 PKs, workspace_id RLS, created_at/updated_at.

-- V10 enums (completed per DECISIONS.md)
DO $$ BEGIN
  CREATE TYPE negotiation_state AS ENUM (
    'proposed','evaluating','accepted','rejected','expired',
    'executing','executed','failed','compensating'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE federation_permission_type AS ENUM (
    'calendar_query','calendar_write','routing_negotiate',
    'task_delegate','knowledge_share','status_query'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE wallet_transaction_type AS ENUM (
    'credit','debit','refund','adjustment','rollup'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE federation_status AS ENUM (
    'pending','active','suspended','revoked','expired'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE capability_status AS ENUM (
    'active','deprecated','disabled','experimental'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE cost_category AS ENUM (
    'llm_inference','tool_execution','storage','bandwidth','external_api','voice_call'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Federation peers
CREATE TABLE IF NOT EXISTS federation_peers (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  peer_workspace_id uuid NOT NULL REFERENCES workspaces(id),
  status federation_status NOT NULL DEFAULT 'pending',
  permissions federation_permission_type[] NOT NULL DEFAULT '{}',
  trust_score numeric(5,4) NOT NULL DEFAULT 0.0,
  established_at timestamptz,
  expires_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, peer_workspace_id)
);

ALTER TABLE federation_peers ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY federation_peers_workspace_isolation ON federation_peers
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Federation negotiations
CREATE TABLE IF NOT EXISTS federation_negotiations (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  peer_id uuid NOT NULL REFERENCES federation_peers(id),
  state negotiation_state NOT NULL DEFAULT 'proposed',
  proposed_permissions federation_permission_type[] NOT NULL DEFAULT '{}',
  accepted_permissions federation_permission_type[] NOT NULL DEFAULT '{}',
  proposer_workspace_id uuid NOT NULL REFERENCES workspaces(id),
  counter_count int NOT NULL DEFAULT 0,
  max_counters int NOT NULL DEFAULT 3,
  expires_at timestamptz NOT NULL,
  resolved_at timestamptz,
  resolution_reason text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE federation_negotiations ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY federation_negotiations_workspace_isolation ON federation_negotiations
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Wallet
CREATE TABLE IF NOT EXISTS wallets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) UNIQUE,
  balance_usd numeric(18,8) NOT NULL DEFAULT 0,
  credit_limit_usd numeric(18,8) NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT 'USD',
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE wallets ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY wallets_workspace_isolation ON wallets
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Wallet transactions
CREATE TABLE IF NOT EXISTS wallet_transactions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  wallet_id uuid NOT NULL REFERENCES wallets(id),
  transaction_type wallet_transaction_type NOT NULL,
  amount_usd numeric(18,8) NOT NULL,
  balance_after_usd numeric(18,8) NOT NULL,
  category cost_category NOT NULL,
  reference_type text NOT NULL,
  reference_id uuid NOT NULL,
  description text NOT NULL DEFAULT '',
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, idempotency_key)
);

ALTER TABLE wallet_transactions ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY wallet_transactions_workspace_isolation ON wallet_transactions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Cost events (raw events written by outbox, read by rollup workflows)
CREATE TABLE IF NOT EXISTS cost_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  category cost_category NOT NULL,
  amount_usd numeric(18,8) NOT NULL,
  reference_type text NOT NULL,
  reference_id uuid NOT NULL,
  provider text NOT NULL DEFAULT '',
  model text NOT NULL DEFAULT '',
  tokens_input int NOT NULL DEFAULT 0,
  tokens_output int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE cost_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY cost_events_workspace_isolation ON cost_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Cost rollups (sole writer: CostRollupWorkflow)
CREATE TABLE IF NOT EXISTS cost_rollups (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  period_start timestamptz NOT NULL,
  period_end timestamptz NOT NULL,
  category cost_category NOT NULL,
  total_usd numeric(18,8) NOT NULL DEFAULT 0,
  event_count int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, period_start, period_end, category)
);

ALTER TABLE cost_rollups ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY cost_rollups_workspace_isolation ON cost_rollups
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Capability registry
CREATE TABLE IF NOT EXISTS capability_registry (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  capability_key text NOT NULL,
  version text NOT NULL DEFAULT '1.0.0',
  status capability_status NOT NULL DEFAULT 'active',
  required_plan_tier text NOT NULL DEFAULT 'T0',
  required_autonomy_level text NOT NULL DEFAULT 'A0',
  config jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, capability_key, version)
);

ALTER TABLE capability_registry ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY capability_registry_workspace_isolation ON capability_registry
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Federation audit log
CREATE TABLE IF NOT EXISTS federation_audit_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  peer_id uuid REFERENCES federation_peers(id),
  action text NOT NULL,
  actor_id uuid NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE federation_audit_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY federation_audit_log_workspace_isolation ON federation_audit_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10 indexes
CREATE INDEX IF NOT EXISTS idx_federation_peers_workspace ON federation_peers(workspace_id);
CREATE INDEX IF NOT EXISTS idx_federation_peers_status ON federation_peers(workspace_id, status);
CREATE INDEX IF NOT EXISTS idx_federation_negotiations_workspace ON federation_negotiations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_federation_negotiations_state ON federation_negotiations(workspace_id, state);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_workspace ON wallet_transactions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_wallet_transactions_created ON wallet_transactions(workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_cost_events_workspace ON cost_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cost_events_category ON cost_events(workspace_id, category, created_at);
CREATE INDEX IF NOT EXISTS idx_cost_rollups_workspace ON cost_rollups(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cost_rollups_period ON cost_rollups(workspace_id, period_start, period_end);
CREATE INDEX IF NOT EXISTS idx_capability_registry_workspace ON capability_registry(workspace_id);
CREATE INDEX IF NOT EXISTS idx_federation_audit_log_workspace ON federation_audit_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_federation_audit_log_created ON federation_audit_log(workspace_id, created_at);
