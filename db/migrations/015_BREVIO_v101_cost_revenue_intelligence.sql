-- BREVIO V10.1 cost & revenue intelligence migration (BP02 gap closure)
-- Implements: 18 cost/revenue/operational intelligence tables from BP02.
-- Applies after: 014_BREVIO_gateway_production_hardening.sql
-- All tables: UUIDv7 PKs, workspace_id RLS where workspace-scoped, forward-only.
-- Money types: NUMERIC(18,8) per D-NNR-103.

-- ============================================================
-- ENUMS
-- ============================================================

DO $$ BEGIN
  CREATE TYPE kill_switch_scope AS ENUM ('user','workspace','global');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE risk_severity AS ENUM ('low','medium','high','critical');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE risk_signal_type AS ENUM (
    'anomalous_volume','cost_spike','tool_abuse','prompt_injection_attempt',
    'data_exfiltration','unusual_hours','rapid_skill_switching'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE adoption_event_type AS ENUM (
    'first_use','repeated_use','power_use','abandoned','reactivated'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE mttr_status AS ENUM ('quarantined','recovering','recovered','permanently_failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- TABLES
-- ============================================================

-- Table 1: llm_cost_ledger — token-level cost record per LLM call (V101-DB-01)
CREATE TABLE IF NOT EXISTS llm_cost_ledger (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  workflow_run_id text NOT NULL DEFAULT '',
  provider text NOT NULL,
  model text NOT NULL,
  tokens_input int NOT NULL DEFAULT 0,
  tokens_output int NOT NULL DEFAULT 0,
  cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  latency_ms int NOT NULL DEFAULT 0,
  cache_hit boolean NOT NULL DEFAULT false,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 2: task_cost_rollup — per-workflow cost aggregation (V101-DB-02)
CREATE TABLE IF NOT EXISTS task_cost_rollup (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_run_id text NOT NULL UNIQUE,
  user_id uuid NOT NULL,
  llm_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  connector_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  total_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  llm_calls int NOT NULL DEFAULT 0,
  connector_calls int NOT NULL DEFAULT 0,
  duration_ms int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 3: connector_cost_ledger — per-call cost for paid connectors (V101-DB-03)
CREATE TABLE IF NOT EXISTS connector_cost_ledger (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  workflow_run_id text NOT NULL DEFAULT '',
  connector_id uuid NOT NULL,
  connector_name text NOT NULL,
  operation text NOT NULL DEFAULT '',
  cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  latency_ms int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 4: user_cost_daily_rollup — pre-aggregated daily cost per user (V101-DB-04)
CREATE TABLE IF NOT EXISTS user_cost_daily_rollup (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  rollup_date date NOT NULL,
  llm_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  connector_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  total_cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  task_count int NOT NULL DEFAULT 0,
  llm_calls int NOT NULL DEFAULT 0,
  connector_calls int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, rollup_date)
);

-- Table 5: operator_margin_report — daily P&L snapshot (V101-DB-05)
CREATE TABLE IF NOT EXISTS operator_margin_report (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  report_date date NOT NULL,
  revenue_usd numeric(18,8) NOT NULL DEFAULT 0,
  cogs_usd numeric(18,8) NOT NULL DEFAULT 0,
  gross_margin_usd numeric(18,8) NOT NULL DEFAULT 0,
  gross_margin_pct numeric(7,4) NOT NULL DEFAULT 0,
  user_count int NOT NULL DEFAULT 0,
  task_count int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, report_date)
);

-- Table 6: agent_kill_switches — per-user kill switch state (V101-DB-06)
CREATE TABLE IF NOT EXISTS agent_kill_switches (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  scope kill_switch_scope NOT NULL DEFAULT 'user',
  is_active boolean NOT NULL DEFAULT true,
  reason text NOT NULL DEFAULT '',
  activated_by uuid NOT NULL,
  deactivated_by uuid,
  deactivated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id)
);

-- Table 7: agent_kill_switch_log — audit log for kill switch actions (V101-DB-07)
CREATE TABLE IF NOT EXISTS agent_kill_switch_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  action text NOT NULL,
  reason text NOT NULL DEFAULT '',
  performed_by uuid NOT NULL,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 8: skill_acl_overrides — per-user per-skill enable/disable (V101-DB-08)
CREATE TABLE IF NOT EXISTS skill_acl_overrides (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  skill_id text NOT NULL,
  is_allowed boolean NOT NULL DEFAULT true,
  reason text NOT NULL DEFAULT '',
  set_by uuid NOT NULL,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, skill_id)
);

-- Table 9: subscription_events — raw Stripe webhook events (V101-DB-09)
CREATE TABLE IF NOT EXISTS subscription_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  stripe_event_id text NOT NULL UNIQUE,
  event_type text NOT NULL,
  payload jsonb NOT NULL,
  processed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 10: mrr_snapshots — daily MRR/ARR snapshot (V101-DB-10)
CREATE TABLE IF NOT EXISTS mrr_snapshots (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  snapshot_date date NOT NULL,
  mrr_usd numeric(18,8) NOT NULL DEFAULT 0,
  arr_usd numeric(18,8) NOT NULL DEFAULT 0,
  new_mrr_usd numeric(18,8) NOT NULL DEFAULT 0,
  churned_mrr_usd numeric(18,8) NOT NULL DEFAULT 0,
  expansion_mrr_usd numeric(18,8) NOT NULL DEFAULT 0,
  active_subscriptions int NOT NULL DEFAULT 0,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, snapshot_date)
);

-- Table 11: user_cohorts — cohort assignment per user (V101-DB-11)
CREATE TABLE IF NOT EXISTS user_cohorts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  cohort_week date NOT NULL,
  signup_source text NOT NULL DEFAULT '',
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id)
);

-- Table 12: cohort_retention — weekly retention computation (V101-DB-12)
CREATE TABLE IF NOT EXISTS cohort_retention (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  cohort_week date NOT NULL,
  period_offset int NOT NULL,
  cohort_size int NOT NULL DEFAULT 0,
  retained_count int NOT NULL DEFAULT 0,
  retention_rate numeric(5,4) NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, cohort_week, period_offset)
);

-- Table 13: oauth_token_registry — OAuth token expiry tracking (V101-DB-13)
CREATE TABLE IF NOT EXISTS oauth_token_registry (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  provider text NOT NULL,
  scopes text[] NOT NULL DEFAULT '{}',
  token_hash text NOT NULL,
  expires_at timestamptz NOT NULL,
  refresh_expires_at timestamptz,
  last_refreshed_at timestamptz,
  is_revoked boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, provider)
);

-- Table 14: behavioral_risk_scores — risk scoring (V101-DB-14)
CREATE TABLE IF NOT EXISTS behavioral_risk_scores (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  overall_score numeric(5,4) NOT NULL DEFAULT 0,
  severity risk_severity NOT NULL DEFAULT 'low',
  signal_breakdown jsonb NOT NULL DEFAULT '{}'::jsonb,
  last_computed_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id)
);

-- Table 15: behavioral_risk_history — risk score history (V101-DB-14 supplement)
CREATE TABLE IF NOT EXISTS behavioral_risk_history (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  signal_type risk_signal_type NOT NULL,
  score_delta numeric(5,4) NOT NULL DEFAULT 0,
  evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 16: feature_adoption_events — skill usage tracking (V101-DB-15)
CREATE TABLE IF NOT EXISTS feature_adoption_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  skill_id text NOT NULL,
  event_type adoption_event_type NOT NULL,
  usage_count int NOT NULL DEFAULT 1,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 17: tool_mttr_log — tool quarantine recovery tracking (V101-DB-16)
CREATE TABLE IF NOT EXISTS tool_mttr_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_id text NOT NULL,
  status mttr_status NOT NULL DEFAULT 'quarantined',
  quarantined_at timestamptz NOT NULL,
  recovered_at timestamptz,
  mttr_seconds int,
  failure_reason text NOT NULL DEFAULT '',
  recovery_action text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 18: agent_action_replay_log — workflow replay audit (V101-DB-17)
CREATE TABLE IF NOT EXISTS agent_action_replay_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  original_workflow_run_id text NOT NULL,
  replay_workflow_run_id text NOT NULL,
  requested_by uuid NOT NULL,
  reason text NOT NULL DEFAULT '',
  outcome text NOT NULL DEFAULT 'pending',
  diff_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================

ALTER TABLE llm_cost_ledger ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY llm_cost_ledger_workspace_isolation ON llm_cost_ledger
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE task_cost_rollup ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY task_cost_rollup_workspace_isolation ON task_cost_rollup
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE connector_cost_ledger ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY connector_cost_ledger_workspace_isolation ON connector_cost_ledger
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE user_cost_daily_rollup ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY user_cost_daily_rollup_workspace_isolation ON user_cost_daily_rollup
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE operator_margin_report ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY operator_margin_report_workspace_isolation ON operator_margin_report
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE agent_kill_switches ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY agent_kill_switches_workspace_isolation ON agent_kill_switches
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE agent_kill_switch_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY agent_kill_switch_log_workspace_isolation ON agent_kill_switch_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE skill_acl_overrides ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY skill_acl_overrides_workspace_isolation ON skill_acl_overrides
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE subscription_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY subscription_events_workspace_isolation ON subscription_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE mrr_snapshots ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY mrr_snapshots_workspace_isolation ON mrr_snapshots
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE user_cohorts ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY user_cohorts_workspace_isolation ON user_cohorts
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE cohort_retention ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY cohort_retention_workspace_isolation ON cohort_retention
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE oauth_token_registry ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY oauth_token_registry_workspace_isolation ON oauth_token_registry
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE behavioral_risk_scores ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY behavioral_risk_scores_workspace_isolation ON behavioral_risk_scores
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE behavioral_risk_history ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY behavioral_risk_history_workspace_isolation ON behavioral_risk_history
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE feature_adoption_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY feature_adoption_events_workspace_isolation ON feature_adoption_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE tool_mttr_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY tool_mttr_log_workspace_isolation ON tool_mttr_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE agent_action_replay_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY agent_action_replay_log_workspace_isolation ON agent_action_replay_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- INDEXES
-- ============================================================

-- Cost ledgers
CREATE INDEX IF NOT EXISTS idx_llm_cost_ledger_workspace ON llm_cost_ledger(workspace_id);
CREATE INDEX IF NOT EXISTS idx_llm_cost_ledger_user ON llm_cost_ledger(workspace_id, user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_cost_ledger_workflow ON llm_cost_ledger(workflow_run_id) WHERE workflow_run_id != '';
CREATE INDEX IF NOT EXISTS idx_task_cost_rollup_workspace ON task_cost_rollup(workspace_id);
CREATE INDEX IF NOT EXISTS idx_task_cost_rollup_user ON task_cost_rollup(workspace_id, user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_connector_cost_ledger_workspace ON connector_cost_ledger(workspace_id);
CREATE INDEX IF NOT EXISTS idx_connector_cost_ledger_user ON connector_cost_ledger(workspace_id, user_id, created_at);

-- Daily rollups
CREATE INDEX IF NOT EXISTS idx_user_cost_daily_rollup_workspace ON user_cost_daily_rollup(workspace_id);
CREATE INDEX IF NOT EXISTS idx_user_cost_daily_rollup_date ON user_cost_daily_rollup(workspace_id, rollup_date);
CREATE INDEX IF NOT EXISTS idx_operator_margin_report_workspace ON operator_margin_report(workspace_id);
CREATE INDEX IF NOT EXISTS idx_operator_margin_report_date ON operator_margin_report(workspace_id, report_date);

-- Kill switches
CREATE INDEX IF NOT EXISTS idx_agent_kill_switches_workspace ON agent_kill_switches(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_kill_switches_active ON agent_kill_switches(workspace_id, user_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_agent_kill_switch_log_workspace ON agent_kill_switch_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_kill_switch_log_user ON agent_kill_switch_log(workspace_id, user_id, created_at);

-- Skill ACL
CREATE INDEX IF NOT EXISTS idx_skill_acl_overrides_workspace ON skill_acl_overrides(workspace_id);
CREATE INDEX IF NOT EXISTS idx_skill_acl_overrides_user ON skill_acl_overrides(workspace_id, user_id);

-- Revenue
CREATE INDEX IF NOT EXISTS idx_subscription_events_workspace ON subscription_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_subscription_events_type ON subscription_events(event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_mrr_snapshots_workspace ON mrr_snapshots(workspace_id);
CREATE INDEX IF NOT EXISTS idx_mrr_snapshots_date ON mrr_snapshots(workspace_id, snapshot_date);

-- Cohorts
CREATE INDEX IF NOT EXISTS idx_user_cohorts_workspace ON user_cohorts(workspace_id);
CREATE INDEX IF NOT EXISTS idx_user_cohorts_week ON user_cohorts(workspace_id, cohort_week);
CREATE INDEX IF NOT EXISTS idx_cohort_retention_workspace ON cohort_retention(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cohort_retention_week ON cohort_retention(workspace_id, cohort_week, period_offset);

-- OAuth
CREATE INDEX IF NOT EXISTS idx_oauth_token_registry_workspace ON oauth_token_registry(workspace_id);
CREATE INDEX IF NOT EXISTS idx_oauth_token_registry_expiry ON oauth_token_registry(expires_at) WHERE NOT is_revoked;

-- Risk
CREATE INDEX IF NOT EXISTS idx_behavioral_risk_scores_workspace ON behavioral_risk_scores(workspace_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_risk_scores_severity ON behavioral_risk_scores(severity) WHERE severity IN ('high', 'critical');
CREATE INDEX IF NOT EXISTS idx_behavioral_risk_history_workspace ON behavioral_risk_history(workspace_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_risk_history_user ON behavioral_risk_history(workspace_id, user_id, created_at);

-- Feature adoption
CREATE INDEX IF NOT EXISTS idx_feature_adoption_events_workspace ON feature_adoption_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_feature_adoption_events_skill ON feature_adoption_events(workspace_id, skill_id, created_at);

-- Tool MTTR
CREATE INDEX IF NOT EXISTS idx_tool_mttr_log_workspace ON tool_mttr_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_tool_mttr_log_tool ON tool_mttr_log(workspace_id, tool_id, quarantined_at);

-- Replay
CREATE INDEX IF NOT EXISTS idx_agent_action_replay_log_workspace ON agent_action_replay_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_agent_action_replay_log_original ON agent_action_replay_log(original_workflow_run_id);
