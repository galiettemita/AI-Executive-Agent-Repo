-- migrations/057_eu_ai_act.up.sql
-- EU AI Act compliance tables: risk management (Art. 9), data governance (Art. 10),
-- incident reporting (Art. 73). Enforcement deadline: August 2026.

-- Article 9: Risk Management System
CREATE TABLE IF NOT EXISTS eu_ai_act_risks (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,
    category        TEXT        NOT NULL,
    description     TEXT        NOT NULL,
    likelihood      TEXT        NOT NULL DEFAULT 'medium',
    impact          TEXT        NOT NULL DEFAULT 'medium',
    mitigation_status TEXT      NOT NULL DEFAULT 'open',
    mitigation_notes TEXT,
    review_date     TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '90 days',
    source_event    TEXT,
    source_ref      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_eu_risks_workspace   ON eu_ai_act_risks (workspace_id);
CREATE INDEX IF NOT EXISTS idx_eu_risks_category    ON eu_ai_act_risks (category);
CREATE INDEX IF NOT EXISTS idx_eu_risks_mitigation  ON eu_ai_act_risks (mitigation_status);
CREATE INDEX IF NOT EXISTS idx_eu_risks_review      ON eu_ai_act_risks (review_date);

-- Article 10: Data Governance Log
CREATE TABLE IF NOT EXISTS eu_ai_act_data_governance (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,
    dataset_name    TEXT        NOT NULL,
    provenance      TEXT        NOT NULL,
    quality_score   FLOAT8,
    bias_indicators JSONB,
    dpo_pair_ref    UUID,
    logged_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_eu_dg_workspace  ON eu_ai_act_data_governance (workspace_id);
CREATE INDEX IF NOT EXISTS idx_eu_dg_logged_at  ON eu_ai_act_data_governance (logged_at);
CREATE INDEX IF NOT EXISTS idx_eu_dg_dpo_ref    ON eu_ai_act_data_governance (dpo_pair_ref)
    WHERE dpo_pair_ref IS NOT NULL;

-- Article 73: Incident Log
CREATE TABLE IF NOT EXISTS eu_ai_act_incidents (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,
    incident_type   TEXT        NOT NULL,
    trigger_metric  TEXT        NOT NULL,
    severity        TEXT        NOT NULL DEFAULT 'high',
    description     TEXT        NOT NULL,
    dsr_request_id  UUID,
    resolved_at     TIMESTAMPTZ,
    reported_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_eu_incidents_workspace ON eu_ai_act_incidents (workspace_id);
CREATE INDEX IF NOT EXISTS idx_eu_incidents_type      ON eu_ai_act_incidents (incident_type);
CREATE INDEX IF NOT EXISTS idx_eu_incidents_reported  ON eu_ai_act_incidents (reported_at);
