BEGIN;

CREATE TABLE IF NOT EXISTS experiments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    description     TEXT,
    status          TEXT NOT NULL DEFAULT 'draft',
    control_prompt  TEXT NOT NULL,
    variant_prompt  TEXT NOT NULL,
    metric          TEXT NOT NULL DEFAULT 'quality_score',
    target_p_value  FLOAT NOT NULL DEFAULT 0.05,
    min_samples     INT  NOT NULL DEFAULT 50,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    concluded_at    TIMESTAMPTZ,
    winner          TEXT
);

CREATE TABLE IF NOT EXISTS experiment_assignments (
    workspace_id    UUID NOT NULL,
    experiment_id   UUID NOT NULL REFERENCES experiments(id),
    variant         TEXT NOT NULL,
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, experiment_id)
);

CREATE TABLE IF NOT EXISTS experiment_scores (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id   UUID NOT NULL REFERENCES experiments(id),
    workspace_id    UUID NOT NULL,
    variant         TEXT NOT NULL,
    workflow_id     TEXT NOT NULL,
    quality_score   FLOAT NOT NULL,
    recorded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (experiment_id, workflow_id)
);

CREATE INDEX IF NOT EXISTS idx_experiment_scores_experiment
    ON experiment_scores (experiment_id, variant, recorded_at DESC);

CREATE TABLE IF NOT EXISTS feature_flags (
    key          TEXT NOT NULL,
    enabled      BOOLEAN NOT NULL DEFAULT false,
    workspace_id UUID,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (key, workspace_id)
);

COMMIT;
