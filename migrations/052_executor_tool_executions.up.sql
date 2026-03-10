BEGIN;

-- Executor tool executions: authoritative persistence for simulate/commit 2PC.
-- Replaces in-memory Service.executions and Service.idempotency maps.
CREATE TABLE IF NOT EXISTS tool_executions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  phase VARCHAR(10) NOT NULL CHECK (phase IN ('simulate', 'commit')),
  workspace_id UUID NOT NULL,
  tool_key VARCHAR(120) NOT NULL,
  logical_action TEXT NOT NULL DEFAULT '',
  idempotency_key VARCHAR(512) NOT NULL,
  provider VARCHAR(80) NOT NULL DEFAULT '',
  is_mcp BOOLEAN NOT NULL DEFAULT false,
  mcp_server_id VARCHAR(120) NOT NULL DEFAULT '',
  content_provenance VARCHAR(20) NOT NULL DEFAULT 'native_result'
    CHECK (content_provenance IN ('native_result', 'mcp_result')),
  pii_content BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_tool_execution_idempotency UNIQUE (idempotency_key)
);

CREATE INDEX idx_tool_executions_workspace ON tool_executions (workspace_id, created_at DESC);
CREATE INDEX idx_tool_executions_tool_key ON tool_executions (workspace_id, tool_key);

-- Trust receipts issued on commit. One receipt per execution.
-- Replaces in-memory Service.receipts and Service.receiptByExec maps.
CREATE TABLE IF NOT EXISTS tool_execution_receipts (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  tool_execution_id UUID NOT NULL REFERENCES tool_executions(id),
  undo_instructions TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_tool_execution_receipt UNIQUE (tool_execution_id)
);

-- Side effect counters per workspace::tool_key.
-- Replaces in-memory Service.sideEffects map.
CREATE TABLE IF NOT EXISTS tool_side_effects (
  workspace_id UUID NOT NULL,
  tool_key VARCHAR(120) NOT NULL,
  effect_count INTEGER NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, tool_key)
);

-- Executor audit log with HMAC chain integrity.
-- Replaces in-memory Service.audit slice.
CREATE TABLE IF NOT EXISTS executor_audit_log (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  event_type VARCHAR(120) NOT NULL,
  payload TEXT NOT NULL DEFAULT '',
  hash VARCHAR(64) NOT NULL,
  prev_hash VARCHAR(64) NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_executor_audit_log_created ON executor_audit_log (created_at DESC);

COMMIT;
