-- HIPAA Business Associate Agreements.
CREATE TABLE business_associate_agreements (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    baa_type        TEXT NOT NULL DEFAULT 'standard',
    signed_by       TEXT NOT NULL,
    signed_at       TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ,
    document_hash   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_baa_workspace_id ON business_associate_agreements(workspace_id);
CREATE INDEX idx_baa_active ON business_associate_agreements(workspace_id)
    WHERE revoked_at IS NULL;

-- HIPAA access log (6-year retention per §164.530(j)).
CREATE TABLE hipaa_access_log (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    workspace_id  UUID NOT NULL,
    phi_category  TEXT NOT NULL,
    data_accessed TEXT NOT NULL,
    purpose       TEXT NOT NULL,
    accessed_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hipaa_access_log_user_id ON hipaa_access_log(user_id);
CREATE INDEX idx_hipaa_access_log_accessed_at ON hipaa_access_log(accessed_at);
CREATE INDEX idx_hipaa_access_log_workspace_id ON hipaa_access_log(workspace_id);

-- HIPAA breach log.
CREATE TABLE hipaa_breach_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    user_id         UUID,
    phi_category    TEXT NOT NULL,
    breach_type     TEXT NOT NULL,
    detected_at     TIMESTAMPTZ NOT NULL,
    acknowledged_at TIMESTAMPTZ,
    notified_at     TIMESTAMPTZ,
    details         TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_hipaa_breach_log_workspace_id ON hipaa_breach_log(workspace_id);
CREATE INDEX idx_hipaa_breach_log_detected_at ON hipaa_breach_log(detected_at);

-- Add workspace_type column to workspaces if not exists.
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS workspace_type TEXT DEFAULT 'standard';

-- Add encryption settings to workspace_settings if table exists.
CREATE TABLE IF NOT EXISTS workspace_settings (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id          UUID NOT NULL UNIQUE,
    encryption_at_rest    BOOLEAN NOT NULL DEFAULT false,
    encryption_in_transit BOOLEAN NOT NULL DEFAULT false,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
