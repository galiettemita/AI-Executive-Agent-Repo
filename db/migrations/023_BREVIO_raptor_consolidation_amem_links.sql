-- Migration: RAPTOR consolidation tracking + A-MEM memory link graph.

-- 1a. Consolidation state on memory_items
ALTER TABLE memory_items
    ADD COLUMN IF NOT EXISTS consolidated           BOOLEAN  NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS consolidation_level   INTEGER  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS consolidation_cluster_id UUID;

COMMENT ON COLUMN memory_items.consolidated IS
    'TRUE when this episode has been consolidated into a higher-level summary.';
COMMENT ON COLUMN memory_items.consolidation_level IS
    '0=leaf (original), 1=cluster-level LLM summary, 2=epoch-level (future).';

CREATE INDEX IF NOT EXISTS idx_memory_items_unconsolidated
    ON memory_items (workspace_id, type, created_at DESC)
    WHERE consolidated = FALSE AND score > 0;

-- 1b. Memory link graph (A-MEM pattern)
CREATE TABLE IF NOT EXISTS memory_links (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id TEXT        NOT NULL,
    source_id    UUID        NOT NULL,
    target_id    UUID        NOT NULL,
    link_type    VARCHAR(30) NOT NULL DEFAULT 'associative',
    strength     NUMERIC(4,3) NOT NULL DEFAULT 0.85
        CHECK (strength > 0 AND strength <= 1.0),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by   VARCHAR(20) NOT NULL DEFAULT 'auto',

    CONSTRAINT memory_links_no_self_loop CHECK (source_id != target_id),
    CONSTRAINT memory_links_unique UNIQUE (workspace_id, source_id, target_id, link_type)
);

CREATE INDEX IF NOT EXISTS idx_memory_links_source ON memory_links (workspace_id, source_id);
CREATE INDEX IF NOT EXISTS idx_memory_links_target ON memory_links (workspace_id, target_id);

-- 1c. Consolidation summary tracking
CREATE TABLE IF NOT EXISTS consolidation_summaries (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     TEXT        NOT NULL,
    cluster_id       UUID        NOT NULL,
    level            INTEGER     NOT NULL,
    summary_item_id  UUID,
    episode_count    INTEGER     NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    period_start     TIMESTAMPTZ,
    period_end       TIMESTAMPTZ
);
