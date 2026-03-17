BEGIN;

CREATE TABLE IF NOT EXISTS vision_extractions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID NOT NULL,
    turn_id         TEXT NOT NULL,
    image_type      TEXT NOT NULL,
    normalized_text TEXT NOT NULL,
    entity_count    INT NOT NULL DEFAULT 0,
    confidence      FLOAT NOT NULL,
    extracted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vision_extractions_workspace
    ON vision_extractions (workspace_id, extracted_at DESC);

COMMIT;
