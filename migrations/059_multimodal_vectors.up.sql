-- migrations/059_multimodal_vectors.up.sql
-- Add 1408-dim image embedding column and content_mime_type for multimodal RAG.

ALTER TABLE rag_chunks
    ADD COLUMN IF NOT EXISTS image_embedding vector(1408);

ALTER TABLE rag_chunks
    ADD COLUMN IF NOT EXISTS content_mime_type TEXT DEFAULT 'text/plain';

-- IVFFlat index for image embedding search.
CREATE INDEX IF NOT EXISTS idx_rag_chunks_image_emb
    ON rag_chunks USING ivfflat (image_embedding vector_cosine_ops)
    WITH (lists = 100)
    WHERE image_embedding IS NOT NULL;
