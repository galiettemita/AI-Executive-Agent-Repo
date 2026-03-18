-- migrations/059_multimodal_vectors.down.sql
DROP INDEX IF EXISTS idx_rag_chunks_image_emb;
ALTER TABLE rag_chunks DROP COLUMN IF EXISTS image_embedding;
ALTER TABLE rag_chunks DROP COLUMN IF EXISTS content_mime_type;
