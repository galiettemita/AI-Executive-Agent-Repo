-- BREVIO V10 authorization receipts and enforcement chain (TASK-E)
-- Implements: durable gate decisions, authorization receipts, execution ledger
-- Non-bypassable enforcement: no side effects without receipts.

-- Authorization receipts (issued by Control plane)
CREATE TABLE IF NOT EXISTS authorization_receipts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_run_id text NOT NULL,
  plan_id text NOT NULL,
  decision gate_decision NOT NULL,
  policy_bundle_hash text NOT NULL,
  evaluated_gates text[] NOT NULL DEFAULT '{}',
  gate_results jsonb NOT NULL DEFAULT '{}'::jsonb,
  tool_keys text[] NOT NULL DEFAULT '{}',
  risk_level risk_level NOT NULL DEFAULT 'LOW',
  issued_by text NOT NULL DEFAULT 'control',
  issued_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  consumed_at timestamptz,
  revoked_at timestamptz,
  revocation_reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE authorization_receipts ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY authorization_receipts_workspace_isolation ON authorization_receipts
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Execution ledger (written by Executor after receipt validation)
CREATE TABLE IF NOT EXISTS execution_ledger (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  receipt_id uuid NOT NULL REFERENCES authorization_receipts(id),
  tool_key text NOT NULL,
  phase tool_execution_phase NOT NULL,
  idempotency_key text NOT NULL,
  payload_hash text NOT NULL,
  result_status text NOT NULL,
  result_payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  duration_ms int NOT NULL DEFAULT 0,
  error_message text,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, idempotency_key, phase)
);

ALTER TABLE execution_ledger ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY execution_ledger_workspace_isolation ON execution_ledger
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Idempotency keys (dedup for tool executions)
CREATE TABLE IF NOT EXISTS idempotency_keys (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  scope_key text NOT NULL,
  payload_hash text NOT NULL,
  result_payload jsonb,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, scope_key)
);

ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY idempotency_keys_workspace_isolation ON idempotency_keys
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Kill switch state (per-workspace, non-bypassable)
CREATE TABLE IF NOT EXISTS kill_switch_state (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) UNIQUE,
  is_active boolean NOT NULL DEFAULT false,
  activated_by uuid,
  activated_at timestamptz,
  deactivated_at timestamptz,
  reason text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE kill_switch_state ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY kill_switch_state_workspace_isolation ON kill_switch_state
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_authorization_receipts_workspace ON authorization_receipts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_authorization_receipts_workflow ON authorization_receipts(workspace_id, workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_authorization_receipts_expires ON authorization_receipts(expires_at) WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_execution_ledger_workspace ON execution_ledger(workspace_id);
CREATE INDEX IF NOT EXISTS idx_execution_ledger_receipt ON execution_ledger(receipt_id);
CREATE INDEX IF NOT EXISTS idx_execution_ledger_tool ON execution_ledger(workspace_id, tool_key, created_at);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_workspace ON idempotency_keys(workspace_id);
CREATE INDEX IF NOT EXISTS idx_idempotency_keys_expires ON idempotency_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_kill_switch_state_workspace ON kill_switch_state(workspace_id);
