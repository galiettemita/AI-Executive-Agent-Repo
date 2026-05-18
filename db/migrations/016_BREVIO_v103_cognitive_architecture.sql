-- BREVIO V10.3 cognitive architecture migration (BP04 gap closure)
-- Implements: 11 new tables from COG-01 through COG-12.
-- Applies after: 015_BREVIO_v101_cost_revenue_intelligence.sql
-- All tables: UUIDv7 PKs, workspace_id RLS, forward-only.
-- Note: prospective_memory (singular) already exists in migration 011.
--       Blueprint uses "prospective_memories" (plural); see DECISIONS.md D13.
--       A compatibility view is created for blueprint alignment.

-- ============================================================
-- ENUMS
-- ============================================================

DO $$ BEGIN
  CREATE TYPE thought_node_type AS ENUM ('root','branch','merge','conclusion');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE thought_graph_status AS ENUM ('building','evaluating','concluded','aborted');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE knowledge_type AS ENUM ('known_fact','knowledge_gap','belief');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE user_awareness AS ENUM ('aware','unaware','unknown');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE implicit_signal_type AS ENUM (
    'draft_edited','response_abandoned','immediate_followup',
    'explicit_regenerate','shared_externally','saved_to_memory',
    'response_timing'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE consolidation_status AS ENUM ('running','complete','failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE prospective_trigger_type AS ENUM ('person','topic','event','time','location');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- TABLES
-- ============================================================

-- Table 1: system1_heuristics — learned INSTANT-tier patterns (COG-01)
CREATE TABLE IF NOT EXISTS system1_heuristics (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  pattern_embedding vector(1536) NOT NULL,
  skill_sequence jsonb NOT NULL,
  response_template text,
  activation_count int NOT NULL DEFAULT 0,
  success_rate_30d numeric(4,3) NOT NULL DEFAULT 0.0,
  heuristic_confidence numeric(4,3) NOT NULL DEFAULT 0.0,
  last_activated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 2: thought_graphs — GoT reasoning artifacts (COG-02)
CREATE TABLE IF NOT EXISTS thought_graphs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL,
  root_thought_id uuid,
  conclusion_id uuid,
  node_count int NOT NULL DEFAULT 0,
  branch_count int NOT NULL DEFAULT 0,
  merge_count int NOT NULL DEFAULT 0,
  max_depth int NOT NULL DEFAULT 4,
  status thought_graph_status NOT NULL DEFAULT 'building',
  cost_usd numeric(18,8),
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 3: thought_nodes — individual thought graph nodes (COG-02)
CREATE TABLE IF NOT EXISTS thought_nodes (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  graph_id uuid NOT NULL REFERENCES thought_graphs(id),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  parent_ids uuid[] NOT NULL DEFAULT '{}',
  node_type thought_node_type NOT NULL DEFAULT 'root',
  thought_content text NOT NULL,
  critic_score numeric(4,3),
  is_pruned boolean NOT NULL DEFAULT false,
  depth int NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 4: domain_performance_history — per-domain empirical success rates (COG-03)
CREATE TABLE IF NOT EXISTS domain_performance_history (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  domain text NOT NULL,
  skill_id text,
  total_executions_30d int NOT NULL DEFAULT 0,
  successful_executions_30d int NOT NULL DEFAULT 0,
  user_corrections_30d int NOT NULL DEFAULT 0,
  empirical_success_rate numeric(4,3),
  metacognitive_tier_floor text NOT NULL DEFAULT 'SHALLOW',
  confidence_adjustment numeric(4,3) NOT NULL DEFAULT 0.0,
  last_recalculated_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, domain, skill_id)
);

-- Table 5: user_knowledge_model — user belief and knowledge gap tracking (COG-04)
CREATE TABLE IF NOT EXISTS user_knowledge_model (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  knowledge_type knowledge_type NOT NULL,
  subject text NOT NULL,
  content text NOT NULL,
  confidence numeric(4,3) NOT NULL DEFAULT 0.8,
  source text,
  user_awareness user_awareness NOT NULL DEFAULT 'unknown',
  surfaced_at timestamptz,
  resolved_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 6: belief_distributions — Bayesian preference distributions (COG-05)
CREATE TABLE IF NOT EXISTS belief_distributions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  preference_dim text NOT NULL,
  context_key text,
  context_value text,
  mean numeric(5,4) NOT NULL,
  variance numeric(5,4) NOT NULL DEFAULT 0.1,
  observation_count int NOT NULL DEFAULT 0,
  prior_source text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, preference_dim, context_key, context_value)
);

-- Table 7: implicit_behavior_signals — behavioral preference signal capture (COG-07)
CREATE TABLE IF NOT EXISTS implicit_behavior_signals (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL,
  signal_type implicit_signal_type NOT NULL,
  raw_signal_data jsonb NOT NULL,
  inferred_pref text,
  inferred_value numeric(4,3),
  confidence numeric(4,3) NOT NULL DEFAULT 0.6,
  processed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 8: case_library — problem-solution cases for analogical reasoning (COG-08)
CREATE TABLE IF NOT EXISTS case_library (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  problem_embedding vector(1536) NOT NULL,
  problem_summary text NOT NULL,
  domain text NOT NULL,
  task_graph_json jsonb NOT NULL,
  execution_summary text NOT NULL,
  outcome_score numeric(4,3) NOT NULL,
  is_negative_case boolean NOT NULL DEFAULT false,
  reuse_count int NOT NULL DEFAULT 0,
  last_reused_at timestamptz,
  suitable_for_prompts text[] NOT NULL DEFAULT '{}',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 9: clarification_candidates — optimal clarification question scoring (COG-09)
CREATE TABLE IF NOT EXISTS clarification_candidates (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL,
  question_text text NOT NULL,
  estimated_gain numeric(4,3) NOT NULL,
  disambiguates text[] NOT NULL DEFAULT '{}',
  was_selected boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 10: consolidation_runs — episodic-to-semantic consolidation audit (COG-10)
CREATE TABLE IF NOT EXISTS consolidation_runs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  run_date date NOT NULL,
  episodes_analyzed int NOT NULL DEFAULT 0,
  patterns_extracted int NOT NULL DEFAULT 0,
  patterns_promoted int NOT NULL DEFAULT 0,
  patterns_discarded int NOT NULL DEFAULT 0,
  cost_usd numeric(18,8),
  status consolidation_status NOT NULL DEFAULT 'running',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, run_date)
);

-- Table 11: behavioral_baselines — behavioral drift baseline computation (COG-11)
CREATE TABLE IF NOT EXISTS behavioral_baselines (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  baseline_window_start date NOT NULL,
  baseline_window_end date NOT NULL,
  topic_distribution jsonb NOT NULL DEFAULT '{}'::jsonb,
  skill_usage_distribution jsonb NOT NULL DEFAULT '{}'::jsonb,
  avg_message_hour numeric(4,1),
  override_rate numeric(4,3),
  correction_rate numeric(4,3),
  is_current_baseline boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, baseline_window_start)
);

-- ============================================================
-- ALTER existing tables for COG-10 and COG-12 columns
-- ============================================================

-- COG-10: Add consolidation tracking columns to memory_items
ALTER TABLE memory_items ADD COLUMN IF NOT EXISTS source_episode_ids uuid[];
ALTER TABLE memory_items ADD COLUMN IF NOT EXISTS consolidation_run_id uuid;
ALTER TABLE memory_items ADD COLUMN IF NOT EXISTS pattern_frequency int;

-- ============================================================
-- COMPATIBILITY VIEW: prospective_memories → prospective_memory (D13)
-- ============================================================

CREATE OR REPLACE VIEW prospective_memories AS SELECT * FROM prospective_memory;

-- ============================================================
-- ROW LEVEL SECURITY
-- ============================================================

ALTER TABLE system1_heuristics ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY system1_heuristics_workspace_isolation ON system1_heuristics
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE thought_graphs ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY thought_graphs_workspace_isolation ON thought_graphs
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE thought_nodes ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY thought_nodes_workspace_isolation ON thought_nodes
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE domain_performance_history ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY domain_performance_history_workspace_isolation ON domain_performance_history
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE user_knowledge_model ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY user_knowledge_model_workspace_isolation ON user_knowledge_model
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE belief_distributions ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY belief_distributions_workspace_isolation ON belief_distributions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE implicit_behavior_signals ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY implicit_behavior_signals_workspace_isolation ON implicit_behavior_signals
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE case_library ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY case_library_workspace_isolation ON case_library
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE clarification_candidates ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY clarification_candidates_workspace_isolation ON clarification_candidates
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE consolidation_runs ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY consolidation_runs_workspace_isolation ON consolidation_runs
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

ALTER TABLE behavioral_baselines ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY behavioral_baselines_workspace_isolation ON behavioral_baselines
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- INDEXES
-- ============================================================

-- COG-01: system1_heuristics
CREATE INDEX IF NOT EXISTS idx_system1_heuristics_workspace ON system1_heuristics(workspace_id);
CREATE INDEX IF NOT EXISTS idx_system1_heuristics_embedding ON system1_heuristics
  USING ivfflat (pattern_embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX IF NOT EXISTS idx_system1_heuristics_confidence ON system1_heuristics(workspace_id, heuristic_confidence DESC)
  WHERE activation_count >= 10;

-- COG-02: thought_graphs + thought_nodes
CREATE INDEX IF NOT EXISTS idx_thought_graphs_workspace ON thought_graphs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thought_graphs_ingress ON thought_graphs(ingress_turn_id);
CREATE INDEX IF NOT EXISTS idx_thought_nodes_graph ON thought_nodes(graph_id);
CREATE INDEX IF NOT EXISTS idx_thought_nodes_workspace ON thought_nodes(workspace_id);
CREATE INDEX IF NOT EXISTS idx_thought_nodes_unpruned ON thought_nodes(graph_id, depth) WHERE NOT is_pruned;

-- COG-03: domain_performance_history
CREATE INDEX IF NOT EXISTS idx_domain_performance_workspace ON domain_performance_history(workspace_id);
CREATE INDEX IF NOT EXISTS idx_domain_performance_domain ON domain_performance_history(workspace_id, domain);

-- COG-04: user_knowledge_model
CREATE INDEX IF NOT EXISTS idx_user_knowledge_model_workspace ON user_knowledge_model(workspace_id);
CREATE INDEX IF NOT EXISTS idx_user_knowledge_model_type ON user_knowledge_model(workspace_id, knowledge_type)
  WHERE resolved_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_user_knowledge_model_unaware ON user_knowledge_model(workspace_id, user_awareness)
  WHERE user_awareness = 'unaware' AND resolved_at IS NULL;

-- COG-05: belief_distributions
CREATE INDEX IF NOT EXISTS idx_belief_distributions_workspace ON belief_distributions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_belief_distributions_dim ON belief_distributions(workspace_id, preference_dim);

-- COG-07: implicit_behavior_signals
CREATE INDEX IF NOT EXISTS idx_implicit_behavior_signals_workspace ON implicit_behavior_signals(workspace_id);
CREATE INDEX IF NOT EXISTS idx_implicit_behavior_signals_turn ON implicit_behavior_signals(ingress_turn_id);
CREATE INDEX IF NOT EXISTS idx_implicit_behavior_signals_unprocessed ON implicit_behavior_signals(workspace_id)
  WHERE processed_at IS NULL;

-- COG-08: case_library
CREATE INDEX IF NOT EXISTS idx_case_library_workspace ON case_library(workspace_id);
CREATE INDEX IF NOT EXISTS idx_case_library_embedding ON case_library
  USING ivfflat (problem_embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX IF NOT EXISTS idx_case_library_domain ON case_library(workspace_id, domain);
CREATE INDEX IF NOT EXISTS idx_case_library_score ON case_library(workspace_id, outcome_score DESC)
  WHERE NOT is_negative_case;

-- COG-09: clarification_candidates
CREATE INDEX IF NOT EXISTS idx_clarification_candidates_workspace ON clarification_candidates(workspace_id);
CREATE INDEX IF NOT EXISTS idx_clarification_candidates_turn ON clarification_candidates(ingress_turn_id);

-- COG-10: consolidation_runs
CREATE INDEX IF NOT EXISTS idx_consolidation_runs_workspace ON consolidation_runs(workspace_id);
CREATE INDEX IF NOT EXISTS idx_consolidation_runs_date ON consolidation_runs(workspace_id, run_date);

-- COG-11: behavioral_baselines
CREATE INDEX IF NOT EXISTS idx_behavioral_baselines_workspace ON behavioral_baselines(workspace_id);
CREATE INDEX IF NOT EXISTS idx_behavioral_baselines_current ON behavioral_baselines(workspace_id)
  WHERE is_current_baseline = true;
