-- Migration: Add confidence, retrieval_count, and base_half_life_days to memory items.

ALTER TABLE memory_items
    ADD COLUMN IF NOT EXISTS confidence         NUMERIC(4,3) NOT NULL DEFAULT 1.0
        CHECK (confidence >= 0.0 AND confidence <= 1.0),
    ADD COLUMN IF NOT EXISTS retrieval_count   INTEGER      NOT NULL DEFAULT 0
        CHECK (retrieval_count >= 0),
    ADD COLUMN IF NOT EXISTS base_half_life_days NUMERIC(10,4) NOT NULL DEFAULT 30.0
        CHECK (base_half_life_days > 0);

UPDATE memory_items
    SET confidence           = 1.0,
        retrieval_count      = 0,
        base_half_life_days  = CASE type
            WHEN 'rule'       THEN 730.0
            WHEN 'preference' THEN 730.0
            WHEN 'procedural' THEN 180.0
            WHEN 'semantic'   THEN 90.0
            WHEN 'fact'       THEN 30.0
            WHEN 'episodic'   THEN 30.0
            WHEN 'daily_log'  THEN 7.0
            WHEN 'heartbeat'  THEN 0.1667
            WHEN 'transient'  THEN 0.1667
            ELSE 30.0
        END
WHERE base_half_life_days = 30.0;

CREATE INDEX IF NOT EXISTS idx_memory_items_confidence
    ON memory_items (workspace_id, confidence DESC)
    WHERE confidence >= 0.3;

COMMENT ON COLUMN memory_items.confidence IS
    'Certainty 0.0-1.0. Explicit writes=1.0; inferred writes=0.6-0.9. Items <0.3 excluded from context.';
COMMENT ON COLUMN memory_items.retrieval_count IS
    'Times successfully retrieved. Used in Ebbinghaus spacing-effect: more recalls = slower decay.';
COMMENT ON COLUMN memory_items.base_half_life_days IS
    'Starting half-life before spacing amplification. Immutable after first write.';
