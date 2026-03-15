-- Cross-workspace preference transfer infrastructure.
-- Opt-in only. Same user only. Local preferences always override.

ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS preference_transfer_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS preference_transfer_scope    VARCHAR(20) NOT NULL DEFAULT 'none'
        CHECK (preference_transfer_scope IN ('none', 'universal', 'all'));

CREATE TABLE IF NOT EXISTS preference_transfer_index (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 TEXT NOT NULL,
    source_workspace_id     TEXT NOT NULL,
    source_item_id          TEXT NOT NULL,
    preference_category     VARCHAR(50) NOT NULL,
    preference_summary      TEXT NOT NULL,
    embedding               vector(1536),
    confidence              NUMERIC(4,3) NOT NULL DEFAULT 0.8
        CHECK (confidence > 0 AND confidence <= 1.0),
    is_universal            BOOLEAN NOT NULL DEFAULT FALSE,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pti_source_item_unique UNIQUE (source_workspace_id, source_item_id)
);

CREATE INDEX IF NOT EXISTS idx_pti_user_universal
    ON preference_transfer_index (user_id, is_universal, confidence DESC)
    WHERE is_universal = TRUE;

CREATE INDEX IF NOT EXISTS idx_pti_user_embedding
    ON preference_transfer_index USING ivfflat (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;

CREATE TABLE IF NOT EXISTS preference_transfer_log (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_workspace_id TEXT NOT NULL,
    source_workspace_id TEXT NOT NULL,
    transfer_index_id   UUID NOT NULL,
    transfer_confidence NUMERIC(4,3) NOT NULL,
    applied_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active           BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX IF NOT EXISTS idx_ptl_target_workspace
    ON preference_transfer_log (target_workspace_id, is_active, applied_at DESC);
