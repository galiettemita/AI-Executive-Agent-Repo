-- BREVIO V9 initial schema (Phase 1 Step 2 scaffold)
-- Order: extensions/functions -> enums -> tables -> rls -> indexes

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE OR REPLACE FUNCTION uuid_v7_generate()
RETURNS uuid
LANGUAGE sql
AS $$
  SELECT gen_random_uuid();
$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'brevio_audit_writer') THEN
    CREATE ROLE brevio_audit_writer NOLOGIN;
  END IF;
END
$$;

CREATE TYPE autonomy_level AS ENUM ('A0','A1','A2','A3','A4');
CREATE TYPE plan_tier AS ENUM ('T0','T1','T2','T3');
CREATE TYPE risk_level AS ENUM ('LOW','MEDIUM','ELEVATED','CRITICAL');
CREATE TYPE data_class AS ENUM ('public','internal','confidential','restricted');
CREATE TYPE sensitivity_label AS ENUM ('none','low','moderate','high','regulated');
CREATE TYPE content_trust AS ENUM ('trusted','untrusted','mixed');
CREATE TYPE account_status AS ENUM ('active','suspended','archived');
CREATE TYPE user_status AS ENUM ('active','suspended','deleted');
CREATE TYPE workspace_status AS ENUM ('active','archived');
CREATE TYPE channel_type AS ENUM ('whatsapp','imessage','web','email','voice');
CREATE TYPE role_key AS ENUM ('owner','admin','delegate','auditor','operator');
CREATE TYPE delegation_status AS ENUM ('active','revoked','expired');
CREATE TYPE pairing_status AS ENUM ('pending','accepted','expired','revoked');
CREATE TYPE ingress_status AS ENUM ('received','deduplicated','rejected');
CREATE TYPE gate_decision AS ENUM ('allow','deny','require_approval','require_operator_review');
CREATE TYPE consent_status AS ENUM ('required','approved','denied','expired');
CREATE TYPE workflow_status AS ENUM ('running','completed','failed','cancelled');
CREATE TYPE workflow_step_status AS ENUM ('pending','running','completed','failed','compensated');
CREATE TYPE memory_type AS ENUM ('semantic','episodic','preference','rule','contact_fact','task_fact','daily_log','heartbeat');
CREATE TYPE memory_status AS ENUM ('proposed','needs_confirmation','active','superseded','deleted');
CREATE TYPE connector_status AS ENUM ('enabled','disabled','degraded','offline');
CREATE TYPE tool_execution_phase AS ENUM ('simulate','commit');
CREATE TYPE provisioning_status AS ENUM ('requested','in_progress','active','failed','quarantined','declined');
CREATE TYPE provisioning_step_status AS ENUM ('pending','running','succeeded','failed','compensated');
CREATE TYPE incident_status AS ENUM ('open','investigating','resolved','closed');
CREATE TYPE portability_status AS ENUM ('requested','exported','delivered','completed');
CREATE TYPE review_status AS ENUM ('open','approved','rejected','expired');

CREATE TABLE accounts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  plan_key text NOT NULL DEFAULT 'free',
  status account_status NOT NULL DEFAULT 'active',
  billing_customer_ref text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  account_id uuid NOT NULL REFERENCES accounts(id),
  email text NOT NULL UNIQUE,
  phone_e164 text,
  global_autonomy autonomy_level NOT NULL DEFAULT 'A1',
  timezone text NOT NULL DEFAULT 'UTC',
  status user_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  account_id uuid NOT NULL REFERENCES accounts(id),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  status workspace_status NOT NULL DEFAULT 'active',
  memory_namespace text NOT NULL,
  domain_autonomy_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  allowed_connector_keys text[] NOT NULL DEFAULT ARRAY[]::text[],
  proactive_enabled boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_channels (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  user_id uuid NOT NULL REFERENCES users(id),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  channel_type channel_type NOT NULL,
  channel_identifier text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(channel_type, channel_identifier)
);

CREATE TABLE channel_bindings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_channel_id uuid NOT NULL REFERENCES user_channels(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_channel_id)
);

CREATE TABLE roles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  role_name role_key NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_bindings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  role_name role_key NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE delegation_grants (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  grantee_user_id uuid NOT NULL REFERENCES users(id),
  tool_allowlist text[] NOT NULL DEFAULT ARRAY[]::text[],
  shared_memory_keys text[] NOT NULL DEFAULT ARRAY[]::text[],
  status delegation_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz
);

CREATE TABLE pairing_invitations (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  owner_user_id uuid NOT NULL REFERENCES users(id),
  invite_code text NOT NULL UNIQUE,
  status pairing_status NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL
);

CREATE TABLE ingress_turns (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_channel_id uuid NOT NULL REFERENCES user_channels(id),
  dedup_hash text NOT NULL,
  raw_payload jsonb NOT NULL,
  status ingress_status NOT NULL DEFAULT 'received',
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, dedup_hash)
);

CREATE TABLE channel_identity_envelopes (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid NOT NULL REFERENCES ingress_turns(id),
  envelope jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE content_firewall_logs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  firewall_result jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE execution_gate_decisions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  decision gate_decision NOT NULL,
  reason_code text NOT NULL,
  input_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE consents (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  consent_scope text NOT NULL,
  status consent_status NOT NULL,
  token_hmac text,
  nonce text,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE approvals (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  consent_id uuid REFERENCES consents(id),
  approver_user_id uuid REFERENCES users(id),
  status consent_status NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE semantic_verifier_failures (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_execution_id uuid,
  failure_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workflow_instances (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_key text NOT NULL,
  temporal_workflow_id text NOT NULL,
  status workflow_status NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workflow_steps (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workflow_instance_id uuid NOT NULL REFERENCES workflow_instances(id),
  step_key text NOT NULL,
  status workflow_step_status NOT NULL,
  step_order int NOT NULL,
  detail_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE outbox_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  payload jsonb NOT NULL,
  deliver_after timestamptz,
  sent_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_type memory_type NOT NULL,
  status memory_status NOT NULL DEFAULT 'proposed',
  body text NOT NULL,
  embedding vector(1536),
  data_class data_class NOT NULL DEFAULT 'internal',
  sensitivity_label sensitivity_label NOT NULL DEFAULT 'low',
  retention_policy_id text NOT NULL DEFAULT 'default',
  allowed_processors text[] NOT NULL DEFAULT ARRAY[]::text[],
  content_trust content_trust NOT NULL DEFAULT 'mixed',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_revisions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_item_id uuid NOT NULL REFERENCES memory_items(id),
  revision_body text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_write_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_item_id uuid REFERENCES memory_items(id),
  request_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE memory_exclusion_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid REFERENCES users(id),
  rule_text text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trust_receipts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  tool_execution_id uuid,
  undo_instructions text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trust_receipt_evidence (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  trust_receipt_id uuid NOT NULL REFERENCES trust_receipts(id),
  evidence_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE synthesis_evidence_receipts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE synthesis_evidence_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  synthesis_evidence_receipt_id uuid NOT NULL REFERENCES synthesis_evidence_receipts(id),
  evidence_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auto_commit_proofs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  proof_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trajectories (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  plan_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE trajectory_tool_calls (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  trajectory_id uuid NOT NULL REFERENCES trajectories(id),
  tool_key text NOT NULL,
  args_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE prompt_versions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  prompt_key text NOT NULL,
  version_int int NOT NULL,
  body text NOT NULL,
  parent_version_id uuid,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE document_parse_results (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  document_ref text NOT NULL,
  parse_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connectors (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  connector_key text NOT NULL UNIQUE,
  domain text NOT NULL,
  risk_level risk_level NOT NULL,
  data_class data_class NOT NULL,
  mcp_server_url text,
  status connector_status NOT NULL DEFAULT 'enabled',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connector_tools (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  connector_id uuid NOT NULL REFERENCES connectors(id),
  tool_key text NOT NULL UNIQUE,
  write_capable boolean NOT NULL DEFAULT false,
  reversible boolean NOT NULL DEFAULT false,
  autonomy_floor autonomy_level NOT NULL DEFAULT 'A1',
  input_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
  output_schema jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_oauth_tokens (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  connector_id uuid NOT NULL REFERENCES connectors(id),
  ciphertext bytea NOT NULL,
  nonce bytea NOT NULL,
  key_version text NOT NULL,
  encrypted_at timestamptz NOT NULL DEFAULT now(),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_connector_settings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  connector_id uuid NOT NULL REFERENCES connectors(id),
  enabled boolean NOT NULL DEFAULT true,
  custom_rate_limit int,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, connector_id)
);

CREATE TABLE connector_health (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  connector_id uuid NOT NULL REFERENCES connectors(id),
  p95_latency_ms int,
  error_rate numeric(6,3),
  recorded_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE voice_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  profile_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transcription_logs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  transcript text,
  confidence numeric(5,4),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE airport_knowledge (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  airport_code text NOT NULL,
  knowledge_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE canvas_sessions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  session_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE canvas_interactions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  canvas_session_id uuid NOT NULL REFERENCES canvas_sessions(id),
  interaction_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ha_entity_cache (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  entity_key text NOT NULL,
  state_json jsonb NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE environment_signals (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  signal_key text NOT NULL,
  signal_value jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rate_limit_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid REFERENCES users(id),
  tool_key text,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE financial_merchant_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  merchant_key text NOT NULL,
  rule_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE financial_anomaly_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  anomaly_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE system_autonomy_overrides (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  override_level autonomy_level NOT NULL,
  reason text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_domain_autonomy_settings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  domain_key text NOT NULL,
  autonomy_level autonomy_level NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, domain_key)
);

CREATE TABLE key_versions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  key_purpose text NOT NULL,
  key_version text NOT NULL,
  activated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(key_purpose, key_version)
);

CREATE TABLE portability_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  status portability_status NOT NULL DEFAULT 'requested',
  created_at timestamptz NOT NULL DEFAULT now(),
  completed_at timestamptz
);

CREATE TABLE incident_notifications (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  incident_key text NOT NULL,
  status incident_status NOT NULL DEFAULT 'open',
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_entity_fingerprints (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  user_id uuid NOT NULL REFERENCES users(id),
  fingerprint_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, user_id, fingerprint_hash)
);

CREATE TABLE audit_log_entries (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  actor_user_id uuid REFERENCES users(id),
  event_type text NOT NULL,
  event_json jsonb NOT NULL,
  previous_hash text,
  event_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

GRANT INSERT ON audit_log_entries TO brevio_audit_writer;

CREATE OR REPLACE FUNCTION prevent_audit_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  RAISE EXCEPTION 'audit_log_entries is append-only';
END;
$$;

CREATE TRIGGER trg_audit_log_no_update
BEFORE UPDATE ON audit_log_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_mutation();

CREATE TRIGGER trg_audit_log_no_delete
BEFORE DELETE ON audit_log_entries
FOR EACH ROW
EXECUTE FUNCTION prevent_audit_mutation();

CREATE TABLE tool_executions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  ingress_turn_id uuid REFERENCES ingress_turns(id),
  tool_key text NOT NULL,
  phase tool_execution_phase NOT NULL,
  logical_action_hash text NOT NULL,
  idempotency_key text NOT NULL,
  request_json jsonb NOT NULL,
  response_json jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, tool_key, logical_action_hash, phase)
);

CREATE TABLE server_catalog (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  server_key text NOT NULL UNIQUE,
  risk_level risk_level NOT NULL,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_requests (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  server_catalog_id uuid REFERENCES server_catalog(id),
  status provisioning_status NOT NULL DEFAULT 'requested',
  request_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_declined (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provisioning_request_id uuid NOT NULL REFERENCES provisioning_requests(id),
  reason text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_steps (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provisioning_request_id uuid NOT NULL REFERENCES provisioning_requests(id),
  step_key text NOT NULL,
  status provisioning_step_status NOT NULL,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  provisioning_request_id uuid NOT NULL REFERENCES provisioning_requests(id),
  event_type text NOT NULL,
  event_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_mcp_servers (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  server_catalog_id uuid NOT NULL REFERENCES server_catalog(id),
  status provisioning_status NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, server_catalog_id)
);

CREATE TABLE mcp_tool_schema_snapshots (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workspace_mcp_server_id uuid NOT NULL REFERENCES workspace_mcp_servers(id),
  snapshot_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_policy_versions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  version_int int NOT NULL,
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, version_int)
);

CREATE TABLE provisioning_monthly_budgets (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  yyyymm text NOT NULL,
  max_calls int,
  max_cost_usd numeric(12,2),
  max_concurrency int,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, yyyymm)
);

CREATE TABLE provisioning_monthly_usage (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  yyyymm text NOT NULL,
  calls_used int NOT NULL DEFAULT 0,
  cost_used_usd numeric(12,2) NOT NULL DEFAULT 0,
  concurrency_used int NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, yyyymm)
);

CREATE TABLE workspace_server_rules (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  allowed_server_ids text[] NOT NULL DEFAULT ARRAY[]::text[],
  denied_server_ids text[] NOT NULL DEFAULT ARRAY[]::text[],
  max_allowed_risk_level risk_level NOT NULL DEFAULT 'MEDIUM',
  require_operator_review_at_or_above risk_level,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE server_artifacts (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  server_catalog_id uuid REFERENCES server_catalog(id),
  image_digest text NOT NULL,
  sbom_s3_uri text,
  signature_bundle_json jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE runtime_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  profile_key text NOT NULL,
  profile_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mcp_egress_audit_events (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  workspace_mcp_server_id uuid REFERENCES workspace_mcp_servers(id),
  request_host text NOT NULL,
  request_path text NOT NULL,
  blocked boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE provisioning_ranker_versions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  version_int int NOT NULL UNIQUE,
  weights_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE connector_success_stats (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  connector_id uuid NOT NULL REFERENCES connectors(id),
  success_count int NOT NULL DEFAULT 0,
  failure_count int NOT NULL DEFAULT 0,
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, connector_id)
);

CREATE TABLE capability_defs (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  capability_key text NOT NULL UNIQUE,
  description text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE capability_aliases (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  capability_def_id uuid NOT NULL REFERENCES capability_defs(id),
  alias text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(capability_def_id, alias)
);

CREATE TABLE tool_capability_bindings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  tool_key text NOT NULL,
  capability_def_id uuid NOT NULL REFERENCES capability_defs(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tool_key, capability_def_id)
);

CREATE TABLE server_capability_bindings (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  server_catalog_id uuid NOT NULL REFERENCES server_catalog(id),
  capability_def_id uuid NOT NULL REFERENCES capability_defs(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(server_catalog_id, capability_def_id)
);

CREATE TABLE capability_resolution_cache (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  query_hash text NOT NULL,
  result_json jsonb NOT NULL,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, query_hash)
);

CREATE TABLE llm_output_replay (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  request_hash text NOT NULL,
  response_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, request_hash)
);

CREATE TABLE discovery_sessions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  stage_key text NOT NULL,
  status workflow_status NOT NULL DEFAULT 'running',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE discovery_questions (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  stage_key text NOT NULL,
  question_key text NOT NULL,
  prompt text NOT NULL,
  sort_order int NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(stage_key, question_key)
);

CREATE TABLE discovery_answers (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  discovery_session_id uuid NOT NULL REFERENCES discovery_sessions(id),
  discovery_question_id uuid NOT NULL REFERENCES discovery_questions(id),
  answer_text text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE discovery_unparsed_lines (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  discovery_session_id uuid NOT NULL REFERENCES discovery_sessions(id),
  raw_line text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  version_int int NOT NULL DEFAULT 1,
  profile_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_personas (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  version_int int NOT NULL DEFAULT 1,
  persona_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspace_behavior_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  version_int int NOT NULL DEFAULT 1,
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE code_repositories (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  repo_url text NOT NULL,
  default_branch text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE code_repo_profiles (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  code_repository_id uuid NOT NULL REFERENCES code_repositories(id),
  profile_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE model_catalog (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  model_id text NOT NULL UNIQUE,
  provider_id text NOT NULL,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tool_inventory (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  tool_key text NOT NULL UNIQUE,
  metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE routing_policies (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  policy_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE specialist_agents (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid REFERENCES workspaces(id),
  agent_key text NOT NULL,
  config_json jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE review_tasks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  task_key text NOT NULL,
  status review_status NOT NULL DEFAULT 'open',
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Workspace-scoped RLS
DO $$
DECLARE
  t text;
  workspace_tables text[] := ARRAY[
    'user_channels','channel_bindings','role_bindings','delegation_grants','pairing_invitations',
    'ingress_turns','channel_identity_envelopes','content_firewall_logs','execution_gate_decisions',
    'consents','approvals','semantic_verifier_failures','workflow_instances','workflow_steps','outbox_items',
    'memory_items','memory_revisions','memory_write_requests','memory_exclusion_rules','trust_receipts',
    'trust_receipt_evidence','synthesis_evidence_receipts','synthesis_evidence_items','auto_commit_proofs',
    'trajectories','trajectory_tool_calls','document_parse_results','user_oauth_tokens','user_connector_settings',
    'connector_health','voice_profiles','transcription_logs','airport_knowledge','canvas_sessions',
    'canvas_interactions','ha_entity_cache','environment_signals','rate_limit_events','financial_merchant_rules',
    'financial_anomaly_events','system_autonomy_overrides','user_domain_autonomy_settings','key_versions',
    'portability_requests','incident_notifications','user_entity_fingerprints','audit_log_entries',
    'tool_executions',
    'provisioning_requests','provisioning_declined','provisioning_steps','provisioning_events',
    'workspace_mcp_servers','mcp_tool_schema_snapshots','provisioning_policy_versions',
    'provisioning_monthly_budgets','provisioning_monthly_usage','workspace_server_rules',
    'server_artifacts','runtime_profiles','mcp_egress_audit_events','connector_success_stats',
    'capability_resolution_cache','prompt_versions','llm_output_replay','discovery_sessions',
    'discovery_answers','discovery_unparsed_lines','workspace_profiles','workspace_personas',
    'workspace_behavior_policies','code_repositories','code_repo_profiles','routing_policies',
    'specialist_agents','review_tasks'
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

-- FK and lookup indexes
CREATE INDEX idx_users_account_id ON users(account_id);
CREATE INDEX idx_workspaces_account_id ON workspaces(account_id);
CREATE INDEX idx_workspaces_owner_user_id ON workspaces(owner_user_id);

DO $$
DECLARE
  t text;
BEGIN
  FOR t IN
    SELECT table_name
    FROM information_schema.columns
    WHERE table_schema = 'public' AND column_name = 'workspace_id'
  LOOP
    EXECUTE format('CREATE INDEX IF NOT EXISTS idx_%I_workspace_id ON %I(workspace_id)', t, t);
  END LOOP;
END
$$;

CREATE INDEX idx_user_channels_user_id ON user_channels(user_id);
CREATE INDEX idx_channel_bindings_user_channel_id ON channel_bindings(user_channel_id);
CREATE INDEX idx_role_bindings_user_id ON role_bindings(user_id);
CREATE INDEX idx_workflow_steps_workflow_instance_id ON workflow_steps(workflow_instance_id);
CREATE INDEX idx_memory_revisions_memory_item_id ON memory_revisions(memory_item_id);
CREATE INDEX idx_trajectory_tool_calls_trajectory_id ON trajectory_tool_calls(trajectory_id);
CREATE INDEX idx_connector_tools_connector_id ON connector_tools(connector_id);
CREATE INDEX idx_user_oauth_tokens_user_id ON user_oauth_tokens(user_id);
CREATE INDEX idx_user_connector_settings_user_id ON user_connector_settings(user_id);
CREATE INDEX idx_canvas_interactions_canvas_session_id ON canvas_interactions(canvas_session_id);
CREATE INDEX idx_tool_executions_ingress_turn_id ON tool_executions(ingress_turn_id);
CREATE INDEX idx_provisioning_steps_request_id ON provisioning_steps(provisioning_request_id);
CREATE INDEX idx_provisioning_events_request_id ON provisioning_events(provisioning_request_id);
CREATE INDEX idx_workspace_mcp_servers_server_catalog_id ON workspace_mcp_servers(server_catalog_id);
CREATE INDEX idx_connector_success_stats_connector_id ON connector_success_stats(connector_id);
CREATE INDEX idx_discovery_answers_session_id ON discovery_answers(discovery_session_id);
CREATE INDEX idx_code_repo_profiles_repository_id ON code_repo_profiles(code_repository_id);
