-- Migration: Add embedding vectors to memory_items for episodic/procedural/proactive retrieval.

ALTER TABLE memory_items
    ADD COLUMN IF NOT EXISTS embedding vector(1536);

CREATE INDEX IF NOT EXISTS idx_memory_items_embedding
    ON memory_items USING ivfflat (embedding vector_cosine_ops)
    WHERE embedding IS NOT NULL;

COMMENT ON COLUMN memory_items.embedding IS
    'Dense vector representation for semantic retrieval. 1536-dim text-embedding-3-small.';
