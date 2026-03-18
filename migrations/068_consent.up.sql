-- GDPR-compliant consent registry.
CREATE TABLE consent_records (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID NOT NULL,
    user_id       UUID NOT NULL,
    purpose       TEXT NOT NULL CHECK (purpose IN ('executive_assistance', 'analytics', 'fine_tuning', 'marketing')),
    lawful_basis  TEXT NOT NULL CHECK (lawful_basis IN ('consent', 'contract', 'legitimate_interest')),
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ,
    revoked_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consent_records_workspace_user_purpose
    ON consent_records(workspace_id, user_id, purpose);

CREATE INDEX idx_consent_records_revoked ON consent_records(revoked_at)
    WHERE revoked_at IS NULL;

-- RLS policy: workspaces can only see their own consent records.
ALTER TABLE consent_records ENABLE ROW LEVEL SECURITY;
CREATE POLICY consent_workspace_isolation ON consent_records
    USING (workspace_id = current_setting('app.current_workspace_id', true)::UUID);

-- Purpose limitation audit trail.
CREATE TABLE purpose_audit_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id    UUID NOT NULL REFERENCES consent_records(id),
    access_type   TEXT NOT NULL,
    data_category TEXT NOT NULL,
    accessed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_purpose_audit_log_consent_id ON purpose_audit_log(consent_id);
CREATE INDEX idx_purpose_audit_log_accessed_at ON purpose_audit_log(accessed_at);

-- DSR erasure log for consent revocation tracking.
CREATE TABLE IF NOT EXISTS dsr_erasure_log (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id               UUID NOT NULL,
    workspace_id          UUID NOT NULL,
    purpose               TEXT NOT NULL,
    erased_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    records_erased_count  INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_dsr_erasure_log_workspace ON dsr_erasure_log(workspace_id);
