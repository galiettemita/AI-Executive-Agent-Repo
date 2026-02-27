-- BREVIO V9.1 soft intelligence addendum
-- Order: enums -> tables -> rls -> indexes

CREATE TYPE goal_horizon AS ENUM ('daily','weekly','monthly','quarterly','yearly');
CREATE TYPE goal_status AS ENUM ('draft','active','stalled','completed','archived');
CREATE TYPE goal_priority AS ENUM ('low','medium','high','critical');
CREATE TYPE debt_severity AS ENUM ('minor','major','critical');
CREATE TYPE debt_category AS ENUM ('architecture','performance','security','reliability','maintainability');
CREATE TYPE debt_status AS ENUM ('open','in_progress','resolved','deferred');
CREATE TYPE debt_task_status AS ENUM ('open','in_progress','done','blocked');
CREATE TYPE template_status AS ENUM ('draft','published','retired');
CREATE TYPE dependency_type AS ENUM ('runtime','build','test','infrastructure');
CREATE TYPE pattern_scope AS ENUM ('repo','workspace','cross_repo');
CREATE TYPE feedback_type AS ENUM ('positive','negative','correction','suggestion');
CREATE TYPE feedback_disposition AS ENUM ('new','accepted','rejected','superseded');
CREATE TYPE lesson_status AS ENUM ('proposed','confirmed','retired');
CREATE TYPE capture_trigger AS ENUM ('end_of_day','session_end','cron','manual');
CREATE TYPE exploration_status AS ENUM ('candidate','accepted','deferred','rejected');
CREATE TYPE trust_event_type AS ENUM ('success','failure','override');
CREATE TYPE promotion_status AS ENUM ('proposed','approved','denied','expired');
CREATE TYPE self_mod_action AS ENUM ('deny','require_approval','allow_with_audit');
CREATE TYPE widget_type AS ENUM ('kpi','list','timeline','alert','insight');
CREATE TYPE followup_trigger AS ENUM ('missing_data','low_confidence','conflict','manual');
CREATE TYPE context_export_format AS ENUM ('json','markdown','yaml');

CREATE TABLE goal_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  title text NOT NULL,
  horizon goal_horizon NOT NULL,
  status goal_status NOT NULL DEFAULT 'draft',
  priority goal_priority NOT NULL DEFAULT 'medium',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE goal_milestones (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  goal_item_id uuid NOT NULL REFERENCES goal_items(id),
  title text NOT NULL,
  due_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE goal_progress_logs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  goal_item_id uuid NOT NULL REFERENCES goal_items(id),
  progress_note text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mission_control_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  refresh_cadence_minutes int NOT NULL DEFAULT 60,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE mission_control_widgets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  widget_key text NOT NULL,
  widget_type widget_type NOT NULL,
  config_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, widget_key)
);

CREATE TABLE autonomy_trust_scores (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  score numeric(5,4) NOT NULL,
  success_count_30d int NOT NULL DEFAULT 0,
  failure_count_30d int NOT NULL DEFAULT 0,
  override_count_30d int NOT NULL DEFAULT 0,
  computed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE autonomy_promotions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  proposed_level autonomy_level NOT NULL,
  status promotion_status NOT NULL DEFAULT 'proposed',
  created_at timestamptz NOT NULL DEFAULT now(),
  decided_at timestamptz
);

CREATE TABLE learning_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  max_active_lessons int NOT NULL DEFAULT 25,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE interaction_feedback (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  feedback_type feedback_type NOT NULL,
  disposition feedback_disposition NOT NULL DEFAULT 'new',
  feedback_text text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE learned_lessons (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  lesson_text text NOT NULL,
  status lesson_status NOT NULL DEFAULT 'proposed',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE daily_captures (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  capture_date date NOT NULL,
  trigger capture_trigger NOT NULL,
  capture_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, capture_date)
);

CREATE TABLE introspection_templates (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  template_key text NOT NULL,
  status template_status NOT NULL DEFAULT 'draft',
  body text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, template_key)
);

CREATE TABLE capability_recommendations (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  capability_key text NOT NULL,
  status exploration_status NOT NULL DEFAULT 'candidate',
  recommendation_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE exploration_history (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  event_type trust_event_type NOT NULL,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE project_templates (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  template_key text NOT NULL,
  status template_status NOT NULL DEFAULT 'draft',
  template_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, template_key)
);

CREATE TABLE code_context_exports (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  export_format context_export_format NOT NULL,
  export_uri text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE repo_dependencies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  code_repository_id uuid REFERENCES code_repositories(id),
  dependency_name text NOT NULL,
  dependency_type dependency_type NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE cross_repo_patterns (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  pattern_scope pattern_scope NOT NULL,
  pattern_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE technical_debt_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  title text NOT NULL,
  severity debt_severity NOT NULL,
  category debt_category NOT NULL,
  status debt_status NOT NULL DEFAULT 'open',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE debt_resolution_tasks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  technical_debt_item_id uuid NOT NULL REFERENCES technical_debt_items(id),
  title text NOT NULL,
  status debt_task_status NOT NULL DEFAULT 'open',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE self_modification_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  action self_mod_action NOT NULL DEFAULT 'deny',
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id)
);

CREATE TABLE discovery_followup_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  trigger followup_trigger NOT NULL,
  rule_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE discovery_adaptive_questions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  discovery_session_id uuid REFERENCES discovery_sessions(id),
  question_text text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

DO $$
DECLARE
  t text;
  workspace_tables text[] := ARRAY[
    'goal_items','goal_milestones','goal_progress_logs','mission_control_config','mission_control_widgets',
    'autonomy_trust_scores','autonomy_promotions','learning_config','interaction_feedback','learned_lessons',
    'daily_captures','introspection_templates','capability_recommendations','exploration_history','project_templates',
    'code_context_exports','repo_dependencies','cross_repo_patterns','technical_debt_items','debt_resolution_tasks',
    'self_modification_policies','discovery_followup_rules','discovery_adaptive_questions'
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

CREATE INDEX idx_goal_items_workspace_id ON goal_items(workspace_id);
CREATE INDEX idx_goal_milestones_workspace_id ON goal_milestones(workspace_id);
CREATE INDEX idx_goal_milestones_goal_item_id ON goal_milestones(goal_item_id);
CREATE INDEX idx_goal_progress_logs_workspace_id ON goal_progress_logs(workspace_id);
CREATE INDEX idx_goal_progress_logs_goal_item_id ON goal_progress_logs(goal_item_id);
CREATE INDEX idx_mission_control_widgets_workspace_id ON mission_control_widgets(workspace_id);
CREATE INDEX idx_autonomy_trust_scores_workspace_id ON autonomy_trust_scores(workspace_id);
CREATE INDEX idx_autonomy_promotions_workspace_id ON autonomy_promotions(workspace_id);
CREATE INDEX idx_learning_config_workspace_id ON learning_config(workspace_id);
CREATE INDEX idx_interaction_feedback_workspace_id ON interaction_feedback(workspace_id);
CREATE INDEX idx_learned_lessons_workspace_id ON learned_lessons(workspace_id);
CREATE INDEX idx_daily_captures_workspace_id ON daily_captures(workspace_id);
CREATE INDEX idx_introspection_templates_workspace_id ON introspection_templates(workspace_id);
CREATE INDEX idx_capability_recommendations_workspace_id ON capability_recommendations(workspace_id);
CREATE INDEX idx_exploration_history_workspace_id ON exploration_history(workspace_id);
CREATE INDEX idx_project_templates_workspace_id ON project_templates(workspace_id);
CREATE INDEX idx_code_context_exports_workspace_id ON code_context_exports(workspace_id);
CREATE INDEX idx_repo_dependencies_workspace_id ON repo_dependencies(workspace_id);
CREATE INDEX idx_repo_dependencies_code_repository_id ON repo_dependencies(code_repository_id);
CREATE INDEX idx_cross_repo_patterns_workspace_id ON cross_repo_patterns(workspace_id);
CREATE INDEX idx_technical_debt_items_workspace_id ON technical_debt_items(workspace_id);
CREATE INDEX idx_debt_resolution_tasks_workspace_id ON debt_resolution_tasks(workspace_id);
CREATE INDEX idx_debt_resolution_tasks_item_id ON debt_resolution_tasks(technical_debt_item_id);
CREATE INDEX idx_self_modification_policies_workspace_id ON self_modification_policies(workspace_id);
CREATE INDEX idx_discovery_followup_rules_workspace_id ON discovery_followup_rules(workspace_id);
CREATE INDEX idx_discovery_adaptive_questions_workspace_id ON discovery_adaptive_questions(workspace_id);
CREATE INDEX idx_discovery_adaptive_questions_session_id ON discovery_adaptive_questions(discovery_session_id);
