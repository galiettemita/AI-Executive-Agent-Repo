-- BREVIO V10.1 admin intelligence migration (BP02)
-- Implements: ALL 18 admin tables (reconciled from "16" count — see DECISIONS.md)
-- Cost ledgers, rollups, admin auth, RBAC, audit logging, kill switch admin.

-- Admin enums
DO $$ BEGIN
  CREATE TYPE admin_role AS ENUM ('super_admin','workspace_admin','billing_admin','support_admin','auditor','readonly');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE admin_action_type AS ENUM (
    'login','logout','workspace_modify','user_modify','kill_switch_toggle',
    'billing_modify','policy_modify','role_modify','config_modify','data_export',
    'impersonation_start','impersonation_end'
  );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE billing_plan_status AS ENUM ('active','past_due','cancelled','trial','suspended');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE invoice_status AS ENUM ('draft','pending','paid','overdue','cancelled','refunded');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
  CREATE TYPE usage_meter_type AS ENUM ('counter','gauge','histogram');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 1: admin_users
CREATE TABLE IF NOT EXISTS admin_users (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  display_name text NOT NULL,
  admin_role admin_role NOT NULL DEFAULT 'readonly',
  is_active boolean NOT NULL DEFAULT true,
  mfa_enabled boolean NOT NULL DEFAULT false,
  mfa_secret_encrypted text,
  last_login_at timestamptz,
  failed_login_count int NOT NULL DEFAULT 0,
  locked_until timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 2: admin_sessions
CREATE TABLE IF NOT EXISTS admin_sessions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id),
  token_hash text NOT NULL UNIQUE,
  ip_address text NOT NULL,
  user_agent text NOT NULL DEFAULT '',
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 3: admin_audit_log
CREATE TABLE IF NOT EXISTS admin_audit_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id),
  workspace_id uuid REFERENCES workspaces(id),
  action admin_action_type NOT NULL,
  resource_type text NOT NULL,
  resource_id text NOT NULL DEFAULT '',
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  ip_address text NOT NULL DEFAULT '',
  user_agent text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 4: admin_permissions
CREATE TABLE IF NOT EXISTS admin_permissions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_role admin_role NOT NULL,
  resource_type text NOT NULL,
  action text NOT NULL,
  conditions jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(admin_role, resource_type, action)
);

-- Table 5: admin_impersonation_log
CREATE TABLE IF NOT EXISTS admin_impersonation_log (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  target_user_id uuid NOT NULL,
  reason text NOT NULL,
  started_at timestamptz NOT NULL DEFAULT now(),
  ended_at timestamptz,
  actions_taken jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 6: billing_plans
CREATE TABLE IF NOT EXISTS billing_plans (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  name text NOT NULL UNIQUE,
  tier text NOT NULL,
  monthly_price_usd numeric(18,8) NOT NULL DEFAULT 0,
  annual_price_usd numeric(18,8) NOT NULL DEFAULT 0,
  included_credits_usd numeric(18,8) NOT NULL DEFAULT 0,
  max_workspaces int NOT NULL DEFAULT 1,
  max_users_per_workspace int NOT NULL DEFAULT 5,
  features jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 7: billing_subscriptions
CREATE TABLE IF NOT EXISTS billing_subscriptions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  billing_plan_id uuid NOT NULL REFERENCES billing_plans(id),
  status billing_plan_status NOT NULL DEFAULT 'trial',
  current_period_start timestamptz NOT NULL,
  current_period_end timestamptz NOT NULL,
  trial_ends_at timestamptz,
  cancelled_at timestamptz,
  stripe_subscription_id text,
  stripe_customer_id text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE billing_subscriptions ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY billing_subscriptions_workspace_isolation ON billing_subscriptions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 8: invoices
CREATE TABLE IF NOT EXISTS invoices (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  subscription_id uuid REFERENCES billing_subscriptions(id),
  invoice_number text NOT NULL UNIQUE,
  status invoice_status NOT NULL DEFAULT 'draft',
  subtotal_usd numeric(18,8) NOT NULL DEFAULT 0,
  tax_usd numeric(18,8) NOT NULL DEFAULT 0,
  total_usd numeric(18,8) NOT NULL DEFAULT 0,
  currency text NOT NULL DEFAULT 'USD',
  due_date timestamptz NOT NULL,
  paid_at timestamptz,
  stripe_invoice_id text,
  line_items jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE invoices ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY invoices_workspace_isolation ON invoices
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 9: usage_meters
CREATE TABLE IF NOT EXISTS usage_meters (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  meter_key text NOT NULL,
  meter_type usage_meter_type NOT NULL DEFAULT 'counter',
  current_value numeric(18,8) NOT NULL DEFAULT 0,
  limit_value numeric(18,8),
  reset_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, meter_key)
);

ALTER TABLE usage_meters ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY usage_meters_workspace_isolation ON usage_meters
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 10: usage_events
CREATE TABLE IF NOT EXISTS usage_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  meter_id uuid NOT NULL REFERENCES usage_meters(id),
  delta numeric(18,8) NOT NULL,
  source text NOT NULL DEFAULT '',
  reference_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE usage_events ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY usage_events_workspace_isolation ON usage_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 11: workspace_limits
CREATE TABLE IF NOT EXISTS workspace_limits (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id) UNIQUE,
  max_daily_cost_usd numeric(18,8) NOT NULL DEFAULT 100,
  max_monthly_cost_usd numeric(18,8) NOT NULL DEFAULT 3000,
  max_concurrent_workflows int NOT NULL DEFAULT 50,
  max_tool_executions_per_hour int NOT NULL DEFAULT 1000,
  max_llm_tokens_per_day bigint NOT NULL DEFAULT 1000000,
  custom_limits jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workspace_limits ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY workspace_limits_workspace_isolation ON workspace_limits
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 12: admin_notifications
CREATE TABLE IF NOT EXISTS admin_notifications (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid REFERENCES admin_users(id),
  workspace_id uuid REFERENCES workspaces(id),
  notification_type text NOT NULL,
  title text NOT NULL,
  body text NOT NULL DEFAULT '',
  severity text NOT NULL DEFAULT 'info',
  is_read boolean NOT NULL DEFAULT false,
  action_url text,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 13: admin_api_keys
CREATE TABLE IF NOT EXISTS admin_api_keys (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id),
  name text NOT NULL,
  key_hash text NOT NULL UNIQUE,
  key_prefix text NOT NULL,
  permissions text[] NOT NULL DEFAULT '{}',
  expires_at timestamptz,
  last_used_at timestamptz,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Table 14: system_config
CREATE TABLE IF NOT EXISTS system_config (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  config_key text NOT NULL UNIQUE,
  config_value jsonb NOT NULL,
  description text NOT NULL DEFAULT '',
  updated_by uuid REFERENCES admin_users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 15: workspace_health_snapshots
CREATE TABLE IF NOT EXISTS workspace_health_snapshots (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  health_score numeric(5,4) NOT NULL DEFAULT 1.0,
  active_workflows int NOT NULL DEFAULT 0,
  failed_workflows_24h int NOT NULL DEFAULT 0,
  dlq_depth int NOT NULL DEFAULT 0,
  avg_latency_ms int NOT NULL DEFAULT 0,
  error_rate_pct numeric(5,2) NOT NULL DEFAULT 0,
  cost_today_usd numeric(18,8) NOT NULL DEFAULT 0,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE workspace_health_snapshots ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY workspace_health_snapshots_workspace_isolation ON workspace_health_snapshots
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 16: admin_scheduled_reports
CREATE TABLE IF NOT EXISTS admin_scheduled_reports (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  admin_user_id uuid NOT NULL REFERENCES admin_users(id),
  report_type text NOT NULL,
  schedule_cron text NOT NULL,
  recipients text[] NOT NULL DEFAULT '{}',
  filters jsonb NOT NULL DEFAULT '{}'::jsonb,
  is_active boolean NOT NULL DEFAULT true,
  last_run_at timestamptz,
  next_run_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Table 17: admin_feature_overrides
CREATE TABLE IF NOT EXISTS admin_feature_overrides (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  feature_key text NOT NULL,
  override_value jsonb NOT NULL,
  reason text NOT NULL DEFAULT '',
  set_by uuid NOT NULL REFERENCES admin_users(id),
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, feature_key)
);

ALTER TABLE admin_feature_overrides ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY admin_feature_overrides_workspace_isolation ON admin_feature_overrides
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Table 18: admin_data_export_requests
CREATE TABLE IF NOT EXISTS admin_data_export_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  requested_by uuid NOT NULL REFERENCES admin_users(id),
  export_type text NOT NULL,
  filters jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL DEFAULT 'pending',
  file_path text,
  file_size_bytes bigint,
  completed_at timestamptz,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE admin_data_export_requests ENABLE ROW LEVEL SECURITY;
DO $$ BEGIN
  CREATE POLICY admin_data_export_requests_workspace_isolation ON admin_data_export_requests
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Indexes for V10.1
CREATE INDEX IF NOT EXISTS idx_admin_sessions_user ON admin_sessions(admin_user_id);
CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_user ON admin_audit_log(admin_user_id);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_workspace ON admin_audit_log(workspace_id);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_created ON admin_audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_admin_audit_log_action ON admin_audit_log(action, created_at);
CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_workspace ON billing_subscriptions(workspace_id);
CREATE INDEX IF NOT EXISTS idx_invoices_workspace ON invoices(workspace_id);
CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices(status) WHERE status IN ('pending','overdue');
CREATE INDEX IF NOT EXISTS idx_usage_events_workspace ON usage_events(workspace_id);
CREATE INDEX IF NOT EXISTS idx_usage_events_meter ON usage_events(meter_id, created_at);
CREATE INDEX IF NOT EXISTS idx_workspace_health_snapshots_workspace ON workspace_health_snapshots(workspace_id, created_at);
CREATE INDEX IF NOT EXISTS idx_admin_notifications_user ON admin_notifications(admin_user_id) WHERE NOT is_read;
CREATE INDEX IF NOT EXISTS idx_admin_api_keys_user ON admin_api_keys(admin_user_id);
CREATE INDEX IF NOT EXISTS idx_admin_data_export_requests_workspace ON admin_data_export_requests(workspace_id);
