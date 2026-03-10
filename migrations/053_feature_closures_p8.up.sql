-- Migration 053: P8 Feature Closures
-- Federation persistence, edge sync, browser sessions, experiments,
-- fast-path routes, billing webhooks, onboarding sessions.

-- ===================== FEDERATION PERSISTENCE =====================

CREATE TABLE IF NOT EXISTS federation_sync_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    peer_workspace_id UUID NOT NULL,
    sync_type       TEXT NOT NULL CHECK (sync_type IN ('full', 'incremental')),
    items_synced    INT NOT NULL DEFAULT 0,
    conflicts_found INT NOT NULL DEFAULT 0,
    compensated     BOOLEAN NOT NULL DEFAULT FALSE,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed', 'compensating', 'compensated')),
    workflow_run_id TEXT,
    evidence_hash   TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_federation_sync_log_workspace ON federation_sync_log (workspace_id);
CREATE INDEX idx_federation_sync_log_peer ON federation_sync_log (peer_workspace_id);

ALTER TABLE federation_sync_log ENABLE ROW LEVEL SECURITY;
CREATE POLICY federation_sync_log_workspace_isolation ON federation_sync_log
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== EDGE OFFLINE SYNC =====================

CREATE TABLE IF NOT EXISTS edge_sync_tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    agent_id        TEXT NOT NULL,
    task_type       TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    priority        INT NOT NULL DEFAULT 0,
    idempotency_key TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('queued', 'syncing', 'synced', 'executed', 'failed', 'conflict')),
    conflict_resolution TEXT,
    result          JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    synced_at       TIMESTAMPTZ,
    executed_at     TIMESTAMPTZ,
    UNIQUE (workspace_id, idempotency_key)
);

CREATE INDEX idx_edge_sync_tasks_workspace ON edge_sync_tasks (workspace_id);
CREATE INDEX idx_edge_sync_tasks_agent ON edge_sync_tasks (agent_id);
CREATE INDEX idx_edge_sync_tasks_status ON edge_sync_tasks (status) WHERE status IN ('queued', 'syncing');

ALTER TABLE edge_sync_tasks ENABLE ROW LEVEL SECURITY;
CREATE POLICY edge_sync_tasks_workspace_isolation ON edge_sync_tasks
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== BROWSER AUTOMATION SESSIONS =====================

CREATE TABLE IF NOT EXISTS browser_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    workflow_run_id TEXT,
    url             TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('active', 'completed', 'failed', 'timeout')),
    session_type    TEXT NOT NULL CHECK (session_type IN ('scrape', 'form_fill', 'booking', 'price_watch', 'screenshot')),
    result          JSONB,
    receipt_id      TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_browser_sessions_workspace ON browser_sessions (workspace_id);
CREATE INDEX idx_browser_sessions_status ON browser_sessions (status) WHERE status = 'active';

ALTER TABLE browser_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY browser_sessions_workspace_isolation ON browser_sessions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== EXPERIMENTS / FEATURE FLAGS =====================

CREATE TABLE IF NOT EXISTS experiment_definitions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('draft', 'running', 'stopped', 'archived')),
    variants        JSONB NOT NULL DEFAULT '[]',
    started_at      TIMESTAMPTZ,
    ended_at        TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, name)
);

CREATE TABLE IF NOT EXISTS experiment_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    experiment_id   UUID NOT NULL REFERENCES experiment_definitions(id),
    subject_id      TEXT NOT NULL,
    variant_id      TEXT NOT NULL,
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, experiment_id, subject_id)
);

CREATE TABLE IF NOT EXISTS experiment_conversions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    experiment_id   UUID NOT NULL REFERENCES experiment_definitions(id),
    assignment_id   UUID NOT NULL REFERENCES experiment_assignments(id),
    event_type      TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_experiment_definitions_workspace ON experiment_definitions (workspace_id);
CREATE INDEX idx_experiment_assignments_experiment ON experiment_assignments (experiment_id);
CREATE INDEX idx_experiment_conversions_experiment ON experiment_conversions (experiment_id);

ALTER TABLE experiment_definitions ENABLE ROW LEVEL SECURITY;
CREATE POLICY experiment_definitions_workspace_isolation ON experiment_definitions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

ALTER TABLE experiment_assignments ENABLE ROW LEVEL SECURITY;
CREATE POLICY experiment_assignments_workspace_isolation ON experiment_assignments
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

ALTER TABLE experiment_conversions ENABLE ROW LEVEL SECURITY;
CREATE POLICY experiment_conversions_workspace_isolation ON experiment_conversions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== FAST-PATH ROUTES =====================

CREATE TABLE IF NOT EXISTS fast_path_routes (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL,
    pattern             TEXT NOT NULL,
    response            TEXT NOT NULL,
    confidence_threshold NUMERIC(3,2) NOT NULL DEFAULT 0.90,
    hit_count           BIGINT NOT NULL DEFAULT 0,
    avg_latency_ms      NUMERIC(10,2) NOT NULL DEFAULT 0,
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,
    precomputed_answer  TEXT,
    precomputed_expires_at TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_fast_path_routes_workspace ON fast_path_routes (workspace_id);
CREATE INDEX idx_fast_path_routes_enabled ON fast_path_routes (workspace_id) WHERE enabled = TRUE;

ALTER TABLE fast_path_routes ENABLE ROW LEVEL SECURITY;
CREATE POLICY fast_path_routes_workspace_isolation ON fast_path_routes
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== BILLING WEBHOOK EVENTS =====================

CREATE TABLE IF NOT EXISTS billing_webhook_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    provider        TEXT NOT NULL CHECK (provider IN ('stripe', 'manual')),
    event_type      TEXT NOT NULL,
    event_id        TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL CHECK (status IN ('received', 'processed', 'failed', 'ignored')),
    idempotency_key TEXT NOT NULL,
    processed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (idempotency_key)
);

CREATE TABLE IF NOT EXISTS billing_ledger_entries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    entry_type      TEXT NOT NULL CHECK (entry_type IN ('charge', 'credit', 'refund', 'adjustment')),
    amount_cents    BIGINT NOT NULL,
    currency        TEXT NOT NULL DEFAULT 'USD',
    description     TEXT NOT NULL,
    reference_id    TEXT,
    period          TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_billing_webhook_events_workspace ON billing_webhook_events (workspace_id);
CREATE INDEX idx_billing_ledger_entries_workspace ON billing_ledger_entries (workspace_id);
CREATE INDEX idx_billing_ledger_entries_period ON billing_ledger_entries (workspace_id, period);

ALTER TABLE billing_webhook_events ENABLE ROW LEVEL SECURITY;
CREATE POLICY billing_webhook_events_workspace_isolation ON billing_webhook_events
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

ALTER TABLE billing_ledger_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY billing_ledger_entries_workspace_isolation ON billing_ledger_entries
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== ONBOARDING SESSIONS =====================

CREATE TABLE IF NOT EXISTS onboarding_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    current_stage   TEXT NOT NULL,
    completed_stages TEXT[] NOT NULL DEFAULT '{}',
    skipped_stages  TEXT[] NOT NULL DEFAULT '{}',
    stage_answers   JSONB NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL CHECK (status IN ('in_progress', 'completed', 'abandoned')),
    first_value_verified BOOLEAN NOT NULL DEFAULT FALSE,
    workflow_run_id TEXT,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id)
);

CREATE INDEX idx_onboarding_sessions_workspace ON onboarding_sessions (workspace_id);

ALTER TABLE onboarding_sessions ENABLE ROW LEVEL SECURITY;
CREATE POLICY onboarding_sessions_workspace_isolation ON onboarding_sessions
    USING (workspace_id = current_setting('app.workspace_id')::uuid);

-- ===================== LOAD SHEDDING STATE =====================

CREATE TABLE IF NOT EXISTS load_shedding_state (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    current_tier    TEXT NOT NULL CHECK (current_tier IN ('D0', 'D1', 'D2', 'D3', 'D4', 'D5')),
    reason          TEXT,
    escalated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    operator_ack    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id)
);

ALTER TABLE load_shedding_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY load_shedding_state_workspace_isolation ON load_shedding_state
    USING (workspace_id = current_setting('app.workspace_id')::uuid);
