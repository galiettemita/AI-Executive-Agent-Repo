-- Contradiction tracking on memory_items.

ALTER TABLE memory_items
    ADD COLUMN IF NOT EXISTS contradicts_item_id    UUID,
    ADD COLUMN IF NOT EXISTS is_contradicted        BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS contradiction_confidence NUMERIC(4,3) NOT NULL DEFAULT 0.0
        CHECK (contradiction_confidence >= 0.0 AND contradiction_confidence <= 1.0);

CREATE INDEX IF NOT EXISTS idx_memory_items_not_contradicted
    ON memory_items (workspace_id, type, score DESC)
    WHERE is_contradicted = FALSE AND score > 0;

CREATE INDEX IF NOT EXISTS idx_memory_items_contradicts
    ON memory_items (workspace_id, contradicts_item_id)
    WHERE contradicts_item_id IS NOT NULL;
