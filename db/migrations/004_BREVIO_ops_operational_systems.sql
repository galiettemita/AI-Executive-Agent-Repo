CREATE TABLE subscriptions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  account_id uuid NOT NULL REFERENCES accounts(id),
  plan_key text NOT NULL,
  status text NOT NULL,
  billing_customer_ref text,
  current_period_start timestamptz,
  current_period_end timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (status <> '')
);

CREATE TABLE invoices (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  subscription_id uuid NOT NULL REFERENCES subscriptions(id),
  invoice_ref text NOT NULL,
  amount_cents bigint NOT NULL CHECK (amount_cents >= 0),
  currency text NOT NULL DEFAULT 'USD',
  status text NOT NULL,
  due_at timestamptz,
  paid_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, invoice_ref),
  CHECK (status <> '')
);

CREATE TABLE eval_results (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  run_id text NOT NULL,
  suite_key text NOT NULL,
  score numeric(6,4) NOT NULL CHECK (score >= 0 AND score <= 1),
  verdict text NOT NULL,
  details_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_feedback (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid REFERENCES users(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  feedback_type text NOT NULL,
  rating int CHECK (rating IS NULL OR (rating >= 1 AND rating <= 5)),
  comment text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE moderation_queue (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  source_type text NOT NULL,
  source_id text NOT NULL,
  reason text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  reviewed_by uuid REFERENCES users(id),
  reviewed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, source_type, source_id)
);

CREATE TABLE scheduled_notifications (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid REFERENCES users(id),
  channel text NOT NULL,
  payload_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  scheduled_for timestamptz NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE analytics_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  event_type text NOT NULL,
  event_json jsonb NOT NULL,
  occurred_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE analytics_daily (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  metric_date date NOT NULL,
  metrics_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, metric_date)
);

DO $$
DECLARE
  table_name text;
  workspace_tables text[] := ARRAY[
    'subscriptions',
    'invoices',
    'eval_results',
    'user_feedback',
    'moderation_queue',
    'scheduled_notifications',
    'analytics_events',
    'analytics_daily'
  ];
BEGIN
  FOREACH table_name IN ARRAY workspace_tables LOOP
    EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', table_name);
    EXECUTE format(
      'CREATE POLICY %I_workspace_isolation ON %I USING (workspace_id = current_setting(''app.workspace_id'')::uuid)',
      table_name,
      table_name
    );
  END LOOP;
END;
$$;

CREATE INDEX idx_subscriptions_workspace_id ON subscriptions(workspace_id);
CREATE INDEX idx_subscriptions_account_id ON subscriptions(account_id);
CREATE INDEX idx_invoices_workspace_id ON invoices(workspace_id);
CREATE INDEX idx_invoices_subscription_id ON invoices(subscription_id);
CREATE INDEX idx_eval_results_workspace_id ON eval_results(workspace_id);
CREATE INDEX idx_eval_results_run_id ON eval_results(run_id);
CREATE INDEX idx_user_feedback_workspace_id ON user_feedback(workspace_id);
CREATE INDEX idx_user_feedback_user_id ON user_feedback(user_id);
CREATE INDEX idx_user_feedback_ingress_turn_id ON user_feedback(ingress_turn_id);
CREATE INDEX idx_moderation_queue_workspace_id ON moderation_queue(workspace_id);
CREATE INDEX idx_moderation_queue_reviewed_by ON moderation_queue(reviewed_by);
CREATE INDEX idx_scheduled_notifications_workspace_id ON scheduled_notifications(workspace_id);
CREATE INDEX idx_scheduled_notifications_user_id ON scheduled_notifications(user_id);
CREATE INDEX idx_scheduled_notifications_scheduled_for ON scheduled_notifications(scheduled_for);
CREATE INDEX idx_analytics_events_workspace_id ON analytics_events(workspace_id);
CREATE INDEX idx_analytics_events_occurred_at ON analytics_events(occurred_at);
CREATE INDEX idx_analytics_daily_workspace_id ON analytics_daily(workspace_id);
