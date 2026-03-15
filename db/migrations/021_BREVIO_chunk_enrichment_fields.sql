-- Migration: 021_BREVIO_chunk_enrichment_fields
-- Add contextual enrichment fields to the RAG chunks table.

ALTER TABLE rag_chunks
    ADD COLUMN IF NOT EXISTS original_content  TEXT,
    ADD COLUMN IF NOT EXISTS enriched_content  TEXT;

UPDATE rag_chunks
    SET original_content = content
WHERE original_content IS NULL;

COMMENT ON COLUMN rag_chunks.original_content IS
    'Raw chunk text before contextual enrichment. Displayed to users in search results.';

COMMENT ON COLUMN rag_chunks.enriched_content IS
    'Context-header-prepended text used for embedding. Never displayed to users.';
