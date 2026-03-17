BEGIN;

CREATE TABLE IF NOT EXISTS proactive_signals (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    signal_type     TEXT NOT NULL,
    signal_data     JSONB NOT NULL,
    offer_text      TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    sent_at         TIMESTAMPTZ,
    responded_at    TIMESTAMPTZ,
    response        TEXT,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_proactive_signals_workspace_status
    ON proactive_signals (workspace_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS proactive_snooze_preferences (
    workspace_id        UUID PRIMARY KEY,
    snoozed_until       TIMESTAMPTZ,
    signal_type_snoozed TEXT[],
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
