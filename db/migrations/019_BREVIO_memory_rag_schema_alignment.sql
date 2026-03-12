-- Migration 019: Align memory_items and rag_chunks schemas with Go code.
-- Forward-only (D6). Closes mismatches identified in Prompt G traceability audit.

-- ============================================================================
-- 1. memory_items: add columns expected by Go code
-- ============================================================================
ALTER TABLE memory_items
  ADD COLUMN IF NOT EXISTS user_id text NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS embedding_version int NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS expires_at timestamptz;

-- HNSW index for cosine similarity on memory_items.embedding.
-- Enables FindSimilarByEmbedding() to use index scan instead of sequential.
CREATE INDEX IF NOT EXISTS idx_memory_items_embedding_hnsw
  ON memory_items USING hnsw (embedding vector_cosine_ops);

-- ============================================================================
-- 2. rag_chunks: add metadata column expected by Go code
-- ============================================================================
ALTER TABLE rag_chunks
  ADD COLUMN IF NOT EXISTS metadata jsonb NOT NULL DEFAULT '{}';

-- ============================================================================
-- 3. RLS policies for new columns (memory_items.user_id is tenant-scoped
--    via the existing workspace_id RLS; no additional policy needed).
-- ============================================================================

-- ============================================================================
-- 4. Backfill notes:
--    - user_id defaults to '' (empty string). Existing rows will have ''.
--      Application code should populate user_id on new writes.
--    - embedding_version defaults to 1. Safe for existing rows.
--    - expires_at defaults to NULL (no expiry). Safe for existing rows.
--    - metadata defaults to '{}'. Safe for existing rows.
-- ============================================================================
