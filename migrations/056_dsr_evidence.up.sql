-- migrations/056_dsr_evidence.up.sql
-- Creates compliance_dsr_requests and compliance_evidence tables for GDPR DSR persistence,
-- and adds deleted_counts JSONB column to compliance_evidence for tracking erasure cascades.

CREATE TABLE IF NOT EXISTS compliance_dsr_requests (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,
    user_id         UUID        NOT NULL,
    subject_user_id UUID,
    request_type    TEXT        NOT NULL DEFAULT 'erasure',
    status          TEXT        NOT NULL DEFAULT 'pending',
    deadline_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dsr_workspace ON compliance_dsr_requests (workspace_id);
CREATE INDEX IF NOT EXISTS idx_dsr_status    ON compliance_dsr_requests (status, deadline_at);

CREATE TABLE IF NOT EXISTS compliance_evidence (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID        NOT NULL,
    framework_id    TEXT        NOT NULL DEFAULT '',
    event_type      TEXT        NOT NULL DEFAULT '',
    artifact_uri    TEXT        NOT NULL DEFAULT '',
    sha256          TEXT        NOT NULL DEFAULT '',
    collected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_counts  JSONB
);

CREATE INDEX IF NOT EXISTS idx_ce_workspace ON compliance_evidence (workspace_id);
CREATE INDEX IF NOT EXISTS idx_ce_deleted_counts
    ON compliance_evidence ((deleted_counts IS NOT NULL))
    WHERE deleted_counts IS NOT NULL;
