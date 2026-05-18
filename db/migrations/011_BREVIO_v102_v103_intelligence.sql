-- BREVIO V10.2 + V10.3 intelligence migration (BP03/BP04)
-- Implements: EQ strategy matrix, confidence calibration, prospective memory,
-- metacognitive monitoring, emotional context, cognitive intelligence subsystems.

-- V10.2/V10.3 enums
DO $$ BEGIN
  CREATE TYPE eq_strategy AS ENUM (
    'empathetic_acknowledgment','direct_action','clarifying_probe',
    'proactive_suggestion','gentle_redirect','supportive_framing',
    'task_decomposition','confidence_boost','boundary_respect',
    'humor_appropriate'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE emotional_valence AS ENUM (
    'very_negative','negative','slightly_negative','neutral',
    'slightly_positive','positive','very_positive'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE calibration_status AS ENUM (
    'uncalibrated','calibrating','calibrated','drifted','recalibrating'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE metacognitive_state AS ENUM (
    'monitoring','reflecting','adjusting','stable','alert'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE prospective_memory_status AS ENUM (
    'pending','triggered','executed','expired','cancelled'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE cognitive_load_level AS ENUM (
    'minimal','low','moderate','high','overloaded'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.2: EQ strategy matrix
CREATE TABLE IF NOT EXISTS eq_strategy_matrix (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  trigger_pattern text NOT NULL,
  detected_emotion emotional_valence NOT NULL,
  recommended_strategy eq_strategy NOT NULL,
  confidence numeric(5,4) NOT NULL DEFAULT 0.5,
  success_count int NOT NULL DEFAULT 0,
  failure_count int NOT NULL DEFAULT 0,
  last_applied_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, trigger_pattern, detected_emotion)
);

ALTER TABLE eq_strategy_matrix ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY eq_strategy_matrix_workspace_isolation ON eq_strategy_matrix
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.2: Emotional context log
CREATE TABLE IF NOT EXISTS emotional_context_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  session_id uuid,
  detected_valence emotional_valence NOT NULL,
  confidence numeric(5,4) NOT NULL DEFAULT 0.5,
  signals jsonb NOT NULL DEFAULT '[]'::jsonb,
  strategy_applied eq_strategy,
  outcome text,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE emotional_context_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY emotional_context_log_workspace_isolation ON emotional_context_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.2: Confidence calibration
CREATE TABLE IF NOT EXISTS confidence_calibration (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  domain text NOT NULL,
  status calibration_status NOT NULL DEFAULT 'uncalibrated',
  predicted_confidence numeric(5,4) NOT NULL DEFAULT 0.5,
  actual_accuracy numeric(5,4),
  calibration_error numeric(5,4),
  sample_count int NOT NULL DEFAULT 0,
  last_calibrated_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, domain)
);

ALTER TABLE confidence_calibration ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY confidence_calibration_workspace_isolation ON confidence_calibration
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.2: Confidence calibration samples
CREATE TABLE IF NOT EXISTS confidence_calibration_samples (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  calibration_id uuid NOT NULL REFERENCES confidence_calibration(id),
  predicted_confidence numeric(5,4) NOT NULL,
  was_correct boolean NOT NULL,
  context_hash text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE confidence_calibration_samples ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY confidence_calibration_samples_workspace_isolation ON confidence_calibration_samples
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.2: LLM invocation log (single guardrailed invocation library)
CREATE TABLE IF NOT EXISTS llm_invocation_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provider text NOT NULL,
  model text NOT NULL,
  prompt_hash text NOT NULL,
  tokens_input int NOT NULL DEFAULT 0,
  tokens_output int NOT NULL DEFAULT 0,
  cost_usd numeric(18,8) NOT NULL DEFAULT 0,
  latency_ms int NOT NULL DEFAULT 0,
  temperature numeric(3,2) NOT NULL DEFAULT 0,
  guardrails_applied text[] NOT NULL DEFAULT '{}',
  guardrails_triggered text[] NOT NULL DEFAULT '{}',
  policy_budget_remaining numeric(18,8),
  cache_hit boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE llm_invocation_log ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY llm_invocation_log_workspace_isolation ON llm_invocation_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.3: Prospective memory (future-triggered memory items)
CREATE TABLE IF NOT EXISTS prospective_memory (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL,
  trigger_condition jsonb NOT NULL,
  trigger_type text NOT NULL,
  action_payload jsonb NOT NULL,
  status prospective_memory_status NOT NULL DEFAULT 'pending',
  priority int NOT NULL DEFAULT 0,
  scheduled_at timestamptz,
  triggered_at timestamptz,
  executed_at timestamptz,
  expires_at timestamptz,
  context_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE prospective_memory ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY prospective_memory_workspace_isolation ON prospective_memory
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.3: Metacognitive monitoring
CREATE TABLE IF NOT EXISTS metacognitive_monitors (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  monitor_type text NOT NULL,
  current_state metacognitive_state NOT NULL DEFAULT 'monitoring',
  cognitive_load cognitive_load_level NOT NULL DEFAULT 'minimal',
  accuracy_estimate numeric(5,4) NOT NULL DEFAULT 0.5,
  uncertainty_level numeric(5,4) NOT NULL DEFAULT 0.5,
  last_reflection_at timestamptz,
  adjustments_made int NOT NULL DEFAULT 0,
  observations jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, monitor_type)
);

ALTER TABLE metacognitive_monitors ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY metacognitive_monitors_workspace_isolation ON metacognitive_monitors
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.3: Cognitive task decomposition
CREATE TABLE IF NOT EXISTS cognitive_task_decompositions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  parent_task_id uuid,
  task_description text NOT NULL,
  complexity_estimate cognitive_load_level NOT NULL DEFAULT 'moderate',
  decomposition_strategy text NOT NULL DEFAULT 'sequential',
  subtask_count int NOT NULL DEFAULT 0,
  completed_subtask_count int NOT NULL DEFAULT 0,
  status text NOT NULL DEFAULT 'pending',
  reasoning_trace jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE cognitive_task_decompositions ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY cognitive_task_decompositions_workspace_isolation ON cognitive_task_decompositions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.3: Learning transfer records
CREATE TABLE IF NOT EXISTS learning_transfer_records (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  source_domain text NOT NULL,
  target_domain text NOT NULL,
  transfer_type text NOT NULL,
  success_rate numeric(5,4) NOT NULL DEFAULT 0,
  sample_count int NOT NULL DEFAULT 0,
  last_transfer_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, source_domain, target_domain)
);

ALTER TABLE learning_transfer_records ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY learning_transfer_records_workspace_isolation ON learning_transfer_records
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- V10.3: Reasoning chain audit
CREATE TABLE IF NOT EXISTS reasoning_chain_audit (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_run_id text NOT NULL,
  step_index int NOT NULL,
  reasoning_type text NOT NULL,
  input_summary text NOT NULL DEFAULT '',
  output_summary text NOT NULL DEFAULT '',
  confidence numeric(5,4) NOT NULL DEFAULT 0.5,
  duration_ms int NOT NULL DEFAULT 0,
  metacognitive_flags text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE reasoning_chain_audit ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY reasoning_chain_audit_workspace_isolation ON reasoning_chain_audit
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Indexes for V10.2/V10.3
CREATE INDEX IF NOT EXISTS idx_eq_strategy_matrix_workspace ON eq_strategy_matrix(workspace_id);
CREATE INDEX IF NOT EXISTS idx_eq_strategy_matrix_emotion ON eq_strategy_matrix(workspace_id, detected_emotion);
CREATE INDEX IF NOT EXISTS idx_emotional_context_log_workspace ON emotional_context_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_emotional_context_log_user ON emotional_context_log(workspace_id, user_id, created_at);
CREATE INDEX IF NOT EXISTS idx_confidence_calibration_workspace ON confidence_calibration(workspace_id);
CREATE INDEX IF NOT EXISTS idx_confidence_calibration_samples_cal ON confidence_calibration_samples(calibration_id);
CREATE INDEX IF NOT EXISTS idx_llm_invocation_log_workspace ON llm_invocation_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_llm_invocation_log_created ON llm_invocation_log(workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_llm_invocation_log_model ON llm_invocation_log(workspace_id, provider, model);
CREATE INDEX IF NOT EXISTS idx_prospective_memory_workspace ON prospective_memory(workspace_id);
CREATE INDEX IF NOT EXISTS idx_prospective_memory_pending ON prospective_memory(workspace_id, status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_prospective_memory_scheduled ON prospective_memory(scheduled_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_metacognitive_monitors_workspace ON metacognitive_monitors(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cognitive_task_decompositions_workspace ON cognitive_task_decompositions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_cognitive_task_decompositions_parent ON cognitive_task_decompositions(parent_task_id);
CREATE INDEX IF NOT EXISTS idx_learning_transfer_records_workspace ON learning_transfer_records(workspace_id);
CREATE INDEX IF NOT EXISTS idx_reasoning_chain_audit_workspace ON reasoning_chain_audit(workspace_id);
CREATE INDEX IF NOT EXISTS idx_reasoning_chain_audit_workflow ON reasoning_chain_audit(workspace_id, workflow_run_id);
