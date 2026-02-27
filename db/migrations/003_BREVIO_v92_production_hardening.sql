-- BREVIO V9.2 production hardening addendum
-- Order: enums -> tables -> rls -> indexes

CREATE TYPE context_budget_status AS ENUM ('active','exhausted','paused');
CREATE TYPE context_item_type AS ENUM ('system','memory','history','tool','retrieval');
CREATE TYPE rag_chunk_status AS ENUM ('pending','indexed','failed');
CREATE TYPE rag_retrieval_mode AS ENUM ('dense','sparse','hybrid');
CREATE TYPE rag_eval_status AS ENUM ('pending','passed','failed');
CREATE TYPE session_status_v92 AS ENUM ('active','expired','closed');
CREATE TYPE session_intent_status AS ENUM ('inferred','confirmed','corrected');
CREATE TYPE temporal_constraint_priority AS ENUM ('low','medium','high','critical');
CREATE TYPE temporal_resolution_status AS ENUM ('resolved','conflicted','unresolved');
CREATE TYPE travel_mode AS ENUM ('driving','walking','transit','flight');
CREATE TYPE guardrail_severity AS ENUM ('info','warn','block');
CREATE TYPE guardrail_action AS ENUM ('allow','warn','block');
CREATE TYPE tool_health_status AS ENUM ('healthy','degraded','quarantined');
CREATE TYPE quarantine_status AS ENUM ('open','overridden','recovered');
CREATE TYPE feature_flag_type AS ENUM ('boolean','percentage','ruleset');
CREATE TYPE feature_flag_status AS ENUM ('enabled','disabled');
CREATE TYPE feature_flag_match_type AS ENUM ('workspace','user','role','segment');
CREATE TYPE crdt_conflict_status AS ENUM ('auto_resolved','manual_review');
CREATE TYPE streaming_mode AS ENUM ('off','typing','progressive');
CREATE TYPE streaming_ack_status AS ENUM ('sent','delayed','failed');
CREATE TYPE error_category AS ENUM ('validation','policy','provider','system');
CREATE TYPE error_severity AS ENUM ('low','medium','high','critical');
CREATE TYPE event_schema_status AS ENUM ('active','deprecated','retired');
CREATE TYPE compatibility_level AS ENUM ('compatible','breaking');
CREATE TYPE compliance_framework AS ENUM ('soc2','gdpr','ccpa');
CREATE TYPE compliance_evidence_status AS ENUM ('pending','collected','verified');
CREATE TYPE dsr_request_status AS ENUM ('received','in_progress','completed','rejected');
CREATE TYPE cache_scope AS ENUM ('prompt_embedding','tool_result','compiled_context');
CREATE TYPE cache_policy_status AS ENUM ('active','disabled');
CREATE TYPE model_tier AS ENUM ('T0','T1','T2','T3');
CREATE TYPE model_override_status AS ENUM ('active','expired','revoked');
CREATE TYPE react_exit_reason AS ENUM ('complete','max_steps','policy_block','error');
CREATE TYPE pii_policy_status AS ENUM ('required','optional','disabled');
CREATE TYPE sandbox_profile_status AS ENUM ('active','disabled');

CREATE TABLE context_budgets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  budget_tokens int NOT NULL,
  status context_budget_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE context_budget_allocations (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  context_budget_id uuid NOT NULL REFERENCES context_budgets(id),
  item_type context_item_type NOT NULL,
  allocated_tokens int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE context_budget_audit (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  context_budget_id uuid NOT NULL REFERENCES context_budgets(id),
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rag_collections (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  collection_key text NOT NULL,
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, collection_key)
);

CREATE TABLE rag_chunks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rag_collection_id uuid NOT NULL REFERENCES rag_collections(id),
  chunk_text text NOT NULL,
  bm25_tokens tsvector,
  embedding vector(1536),
  status rag_chunk_status NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rag_retrievals (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  mode rag_retrieval_mode NOT NULL,
  retrieval_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rag_reranker_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE conversation_sessions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  status session_status_v92 NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE session_entities (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  conversation_session_id uuid NOT NULL REFERENCES conversation_sessions(id),
  entity_key text NOT NULL,
  entity_value text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE session_intents (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  conversation_session_id uuid NOT NULL REFERENCES conversation_sessions(id),
  intent_key text NOT NULL,
  status session_intent_status NOT NULL DEFAULT 'inferred',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE temporal_reasoning_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE temporal_constraints (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  constraint_text text NOT NULL,
  priority temporal_constraint_priority NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE travel_time_cache (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  origin text NOT NULL,
  destination text NOT NULL,
  mode travel_mode NOT NULL,
  duration_minutes int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, origin, destination, mode)
);

CREATE TABLE guardrails_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE guardrails_rule_sets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rule_set_key text NOT NULL,
  rules_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, rule_set_key)
);

CREATE TABLE guardrails_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  severity guardrail_severity NOT NULL,
  action guardrail_action NOT NULL,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tool_health_scores (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_key text NOT NULL,
  status tool_health_status NOT NULL,
  score numeric(5,4) NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tool_quarantine_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_key text NOT NULL,
  status quarantine_status NOT NULL DEFAULT 'open',
  rule_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, tool_key)
);

CREATE TABLE tool_health_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_key text NOT NULL,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE feature_flags (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  flag_key text NOT NULL,
  flag_type feature_flag_type NOT NULL,
  status feature_flag_status NOT NULL DEFAULT 'disabled',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, flag_key)
);

CREATE TABLE feature_flag_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  feature_flag_id uuid NOT NULL REFERENCES feature_flags(id),
  match_type feature_flag_match_type NOT NULL,
  rule_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE feature_flag_evaluations (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  feature_flag_id uuid NOT NULL REFERENCES feature_flags(id),
  result boolean NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_vector_clocks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_item_id uuid REFERENCES memory_items(id),
  clock_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_conflict_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_item_id uuid REFERENCES memory_items(id),
  status crdt_conflict_status NOT NULL,
  conflict_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE latency_budgets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tier model_tier NOT NULL,
  budget_ms int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, tier)
);

CREATE TABLE streaming_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  mode streaming_mode NOT NULL DEFAULT 'off',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE error_taxonomy (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  code text NOT NULL,
  category error_category NOT NULL,
  severity error_severity NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, code)
);

CREATE TABLE error_templates (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  error_taxonomy_id uuid NOT NULL REFERENCES error_taxonomy(id),
  template_text text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE event_schema_registry (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  event_type text NOT NULL,
  status event_schema_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, event_type)
);

CREATE TABLE event_schema_versions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  event_schema_registry_id uuid NOT NULL REFERENCES event_schema_registry(id),
  version_int int NOT NULL,
  compatibility compatibility_level NOT NULL,
  schema_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, event_schema_registry_id, version_int)
);

CREATE TABLE compliance_frameworks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  framework compliance_framework NOT NULL,
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, framework)
);

CREATE TABLE compliance_evidence (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  compliance_framework_id uuid NOT NULL REFERENCES compliance_frameworks(id),
  status compliance_evidence_status NOT NULL,
  hash_sha256 text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE compliance_dsr_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid REFERENCES users(id),
  status dsr_request_status NOT NULL DEFAULT 'received',
  created_at timestamptz NOT NULL DEFAULT now(),
  due_at timestamptz
);

CREATE TABLE cache_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  scope cache_scope NOT NULL,
  status cache_policy_status NOT NULL DEFAULT 'active',
  ttl_seconds int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, scope)
);

CREATE TABLE cache_audit_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  cache_scope cache_scope NOT NULL,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE model_tier_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tier model_tier NOT NULL,
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, tier)
);

CREATE TABLE model_tier_overrides (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tier model_tier NOT NULL,
  status model_override_status NOT NULL DEFAULT 'active',
  reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE react_execution_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tier model_tier NOT NULL,
  max_steps int NOT NULL,
  exit_reason react_exit_reason,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, tier)
);

CREATE TABLE pii_encryption_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  status pii_policy_status NOT NULL DEFAULT 'required',
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE mcp_sandbox_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  profile_key text NOT NULL,
  status sandbox_profile_status NOT NULL DEFAULT 'active',
  profile_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, profile_key)
);

CREATE TABLE constrained_decoding_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE admin_dashboard_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE admin_saved_views (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  view_key text NOT NULL,
  view_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, view_key)
);

CREATE TABLE admin_alert_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rule_key text NOT NULL,
  rule_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, rule_key)
);

CREATE TABLE admin_alert_channels (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  channel_key text NOT NULL,
  channel_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, channel_key)
);

CREATE TABLE admin_kpi_reports (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  report_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rag_eval_scores (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rag_retrieval_id uuid REFERENCES rag_retrievals(id),
  status rag_eval_status NOT NULL DEFAULT 'pending',
  faithfulness numeric(5,4),
  relevance numeric(5,4),
  created_at timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE
  t text;
  workspace_tables text[] := ARRAY[
    'context_budgets','context_budget_allocations','context_budget_audit','rag_collections','rag_chunks','rag_retrievals',
    'rag_reranker_config','conversation_sessions','session_entities','session_intents','temporal_reasoning_config',
    'temporal_constraints','travel_time_cache','guardrails_config','guardrails_rule_sets','guardrails_events',
    'tool_health_scores','tool_quarantine_rules','tool_health_events','feature_flags','feature_flag_rules',
    'feature_flag_evaluations','memory_vector_clocks','memory_conflict_log','latency_budgets','streaming_config',
    'error_taxonomy','error_templates','event_schema_registry','event_schema_versions','compliance_frameworks',
    'compliance_evidence','compliance_dsr_requests','cache_policies','cache_audit_log','model_tier_policies',
    'model_tier_overrides','react_execution_policies','pii_encryption_policies','mcp_sandbox_profiles',
    'constrained_decoding_config','admin_dashboard_config','admin_saved_views','admin_alert_rules',
    'admin_alert_channels','admin_kpi_reports','rag_eval_scores'
  ];
BEGIN
  FOREACH t IN ARRAY workspace_tables LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
    EXECUTE format(
      'CREATE POLICY %I_workspace_isolation ON %I USING (workspace_id = current_setting(''app.workspace_id'')::uuid)',
      t,
      t
    );
  END LOOP;
END
$$;

DO $$
DECLARE
  t text;
BEGIN
  FOR t IN
    SELECT table_name
    FROM information_schema.columns
    WHERE table_schema = 'public' AND column_name = 'workspace_id'
      AND table_name IN (
        'context_budgets','context_budget_allocations','context_budget_audit','rag_collections','rag_chunks','rag_retrievals',
        'rag_reranker_config','conversation_sessions','session_entities','session_intents','temporal_reasoning_config',
        'temporal_constraints','travel_time_cache','guardrails_config','guardrails_rule_sets','guardrails_events',
        'tool_health_scores','tool_quarantine_rules','tool_health_events','feature_flags','feature_flag_rules',
        'feature_flag_evaluations','memory_vector_clocks','memory_conflict_log','latency_budgets','streaming_config',
        'error_taxonomy','error_templates','event_schema_registry','event_schema_versions','compliance_frameworks',
        'compliance_evidence','compliance_dsr_requests','cache_policies','cache_audit_log','model_tier_policies',
        'model_tier_overrides','react_execution_policies','pii_encryption_policies','mcp_sandbox_profiles',
        'constrained_decoding_config','admin_dashboard_config','admin_saved_views','admin_alert_rules',
        'admin_alert_channels','admin_kpi_reports','rag_eval_scores'
      )
  LOOP
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_workspace_id ON %I(workspace_id)', t, t);
  END LOOP;
END
$$;

CREATE INDEX idx_context_budget_allocations_budget_id ON context_budget_allocations(context_budget_id);
CREATE INDEX idx_context_budget_audit_budget_id ON context_budget_audit(context_budget_id);
CREATE INDEX idx_rag_chunks_collection_id ON rag_chunks(rag_collection_id);
CREATE INDEX idx_rag_retrievals_ingress_turn_id ON rag_retrievals(ingress_turn_id);
CREATE INDEX idx_session_entities_session_id ON session_entities(conversation_session_id);
CREATE INDEX idx_session_intents_session_id ON session_intents(conversation_session_id);
CREATE INDEX idx_feature_flag_rules_flag_id ON feature_flag_rules(feature_flag_id);
CREATE INDEX idx_feature_flag_evaluations_flag_id ON feature_flag_evaluations(feature_flag_id);
CREATE INDEX idx_memory_vector_clocks_item_id ON memory_vector_clocks(memory_item_id);
CREATE INDEX idx_memory_conflict_log_item_id ON memory_conflict_log(memory_item_id);
CREATE INDEX idx_error_templates_taxonomy_id ON error_templates(error_taxonomy_id);
CREATE INDEX idx_event_schema_versions_registry_id ON event_schema_versions(event_schema_registry_id);
CREATE INDEX idx_compliance_evidence_framework_id ON compliance_evidence(compliance_framework_id);
CREATE INDEX idx_compliance_dsr_requests_user_id ON compliance_dsr_requests(user_id);
CREATE INDEX idx_rag_eval_scores_retrieval_id ON rag_eval_scores(rag_retrieval_id);

CREATE INDEX idx_rag_chunks_bm25_tokens_gin ON rag_chunks USING gin (bm25_tokens);
CREATE INDEX idx_rag_chunks_embedding_hnsw ON rag_chunks USING hnsw (embedding vector_cosine_ops);
