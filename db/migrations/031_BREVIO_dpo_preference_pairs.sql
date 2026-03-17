BEGIN;

CREATE TABLE IF NOT EXISTS preference_pairs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL,
    user_id             UUID NOT NULL,
    workflow_run_id     TEXT NOT NULL,
    prompt_hash         TEXT NOT NULL,
    prompt_text         TEXT NOT NULL,
    chosen_response     TEXT NOT NULL,
    rejected_response   TEXT NOT NULL,
    signal_type         TEXT NOT NULL
                            CHECK (signal_type IN ('undo','edit','retry','skip','explicit_thumbsdown')),
    correction_context  JSONB NOT NULL DEFAULT '{}',
    quality_score_before FLOAT8,
    quality_score_after  FLOAT8,
    used_in_round       INT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_preference_pairs_workspace_round
    ON preference_pairs (workspace_id, used_in_round, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_preference_pairs_prompt_hash
    ON preference_pairs (workspace_id, prompt_hash);

CREATE TABLE IF NOT EXISTS dpo_rounds (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            UUID,
    round_number            INT NOT NULL,
    pair_count              INT NOT NULL,
    base_model              TEXT NOT NULL,
    fine_tune_job_id        TEXT,
    checkpoint_id           TEXT,
    status                  TEXT NOT NULL DEFAULT 'pending'
                                CHECK (status IN ('pending','running','completed','failed','rolled_back')),
    quality_score_baseline  FLOAT8,
    quality_score_after     FLOAT8,
    deployed_at             TIMESTAMPTZ,
    rolled_back_at          TIMESTAMPTZ,
    error_message           TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dpo_rounds_workspace_round
    ON dpo_rounds (workspace_id, round_number DESC);

COMMIT;
