-- Federated fine-tuning (P3-14).
CREATE TABLE federated_rounds (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    participating_count  INTEGER NOT NULL,
    gradient_dimensions  INTEGER NOT NULL,
    max_epsilon_used     FLOAT NOT NULL,
    run_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Ensure workspace_settings exists and has dp_sgd_enabled column.
CREATE TABLE IF NOT EXISTS workspace_settings (
    workspace_id UUID PRIMARY KEY,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE workspace_settings ADD COLUMN IF NOT EXISTS dp_sgd_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE workspace_settings ADD COLUMN IF NOT EXISTS encryption_at_rest BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE workspace_settings ADD COLUMN IF NOT EXISTS encryption_in_transit BOOLEAN NOT NULL DEFAULT FALSE;
