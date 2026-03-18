-- Differential privacy budget tracking per workspace.
CREATE TABLE workspace_dp_budgets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL UNIQUE,
    cumulative_epsilon  FLOAT NOT NULL DEFAULT 0.0,
    epsilon_max         FLOAT NOT NULL DEFAULT 10.0,
    delta_target        FLOAT NOT NULL DEFAULT 1e-5,
    rounds_completed    INTEGER NOT NULL DEFAULT 0,
    halted              BOOLEAN NOT NULL DEFAULT FALSE,
    last_updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspace_dp_budgets_workspace_id ON workspace_dp_budgets(workspace_id);

-- Add DP-SGD columns to existing dpo_rounds table.
ALTER TABLE dpo_rounds ADD COLUMN IF NOT EXISTS dp_epsilon FLOAT;
ALTER TABLE dpo_rounds ADD COLUMN IF NOT EXISTS dp_delta FLOAT DEFAULT 1e-5;
ALTER TABLE dpo_rounds ADD COLUMN IF NOT EXISTS sigma FLOAT;
ALTER TABLE dpo_rounds ADD COLUMN IF NOT EXISTS sampling_rate FLOAT;
