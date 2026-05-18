-- BREVIO V10.2 intelligence gap closure migration
-- Implements: autonomy demotion, interruption rules, critic/reflector outputs,
-- multi-intent outputs, uncertainty assessments.
-- Applies after: 016_BREVIO_v103_cognitive_architecture.sql
-- All tables: UUIDv7 PKs, workspace_id RLS, forward-only.

-- ============================================================
-- ENUMS
-- ============================================================

DO $$ BEGIN
  CREATE TYPE demotion_trigger AS ENUM ('trust_score','failure_count','drift','manual');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE interruption_trigger_type AS ENUM ('deadline','anomaly','reminder','insight');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- TABLES
-- ============================================================

-- Autonomy levels per workspace/domain
CREATE TABLE IF NOT EXISTS autonomy_levels (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  domain text NOT NULL,
  current_level int NOT NULL DEFAULT 4 CHECK (current_level >= 0 AND current_level <= 4),
  trust_score numeric(5,4) NOT NULL DEFAULT 1.0,
  last_evaluated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, domain)
);

ALTER TABLE autonomy_levels ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY autonomy_levels_workspace_isolation ON autonomy_levels
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Autonomy demotion event log
CREATE TABLE IF NOT EXISTS autonomy_demotion_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  domain text NOT NULL,
  previous_level int NOT NULL,
  new_level int NOT NULL,
  trigger demotion_trigger NOT NULL,
  reason text NOT NULL DEFAULT '',
  trust_score_at_demotion numeric(5,4),
  failure_count_at_demotion int,
  demoted_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE autonomy_demotion_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY autonomy_demotion_events_workspace_isolation ON autonomy_demotion_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Interruption rules
CREATE TABLE IF NOT EXISTS interruption_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  trigger_type interruption_trigger_type NOT NULL,
  priority int NOT NULL DEFAULT 5 CHECK (priority >= 1 AND priority <= 10),
  condition_text text NOT NULL,
  cooldown_minutes int NOT NULL DEFAULT 60,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE interruption_rules ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY interruption_rules_workspace_isolation ON interruption_rules
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Interruption evaluation log
CREATE TABLE IF NOT EXISTS interruption_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rule_id uuid NOT NULL REFERENCES interruption_rules(id),
  urgency numeric(4,3) NOT NULL,
  message text NOT NULL,
  was_surfaced boolean NOT NULL DEFAULT false,
  evaluated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE interruption_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY interruption_log_workspace_isolation ON interruption_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Critic/reflector outputs
CREATE TABLE IF NOT EXISTS critic_reflector_outputs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_run_id text NOT NULL,
  overall_score numeric(4,3) NOT NULL,
  dimension_scores jsonb NOT NULL DEFAULT '{}'::jsonb,
  passed boolean NOT NULL DEFAULT false,
  failure_modes text[] NOT NULL DEFAULT '{}',
  improvement_directive text NOT NULL DEFAULT '',
  lesson_candidates jsonb NOT NULL DEFAULT '[]'::jsonb,
  pattern_detected boolean NOT NULL DEFAULT false,
  escalate_to_feedback boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE critic_reflector_outputs ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY critic_reflector_outputs_workspace_isolation ON critic_reflector_outputs
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Multi-intent classification outputs
CREATE TABLE IF NOT EXISTS multi_intent_outputs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL,
  raw_input text NOT NULL,
  intents jsonb NOT NULL DEFAULT '[]'::jsonb,
  compound_request boolean NOT NULL DEFAULT false,
  overall_confidence numeric(4,3) NOT NULL DEFAULT 0.0,
  requires_decomposition boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE multi_intent_outputs ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY multi_intent_outputs_workspace_isolation ON multi_intent_outputs
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Uncertainty assessments
CREATE TABLE IF NOT EXISTS uncertainty_assessments (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL,
  raw_confidence numeric(4,3) NOT NULL,
  calibrated_confidence numeric(4,3),
  uncertainty_label text NOT NULL,
  should_qualify boolean NOT NULL DEFAULT false,
  qualifier_phrase text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE uncertainty_assessments ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY uncertainty_assessments_workspace_isolation ON uncertainty_assessments
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- INDEXES
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_autonomy_levels_workspace ON autonomy_levels(workspace_id);
CREATE INDEX IF NOT EXISTS idx_autonomy_demotion_events_workspace ON autonomy_demotion_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_autonomy_demotion_events_domain ON autonomy_demotion_events(workspace_id, domain, demoted_at);
CREATE INDEX IF NOT EXISTS idx_interruption_rules_workspace ON interruption_rules(workspace_id);
CREATE INDEX IF NOT EXISTS idx_interruption_rules_active ON interruption_rules(workspace_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_interruption_log_workspace ON interruption_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_interruption_log_rule ON interruption_log(rule_id, evaluated_at);
CREATE INDEX IF NOT EXISTS idx_critic_reflector_outputs_workspace ON critic_reflector_outputs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_critic_reflector_outputs_workflow ON critic_reflector_outputs(workspace_id, workflow_run_id);
CREATE INDEX IF NOT EXISTS idx_multi_intent_outputs_workspace ON multi_intent_outputs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_multi_intent_outputs_turn ON multi_intent_outputs(ingress_turn_id);
CREATE INDEX IF NOT EXISTS idx_uncertainty_assessments_workspace ON uncertainty_assessments(workspace_id);
CREATE INDEX IF NOT EXISTS idx_uncertainty_assessments_turn ON uncertainty_assessments(ingress_turn_id);
