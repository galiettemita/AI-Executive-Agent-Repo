-- ColBERT-lite sub-sentence vectors for fine-grained query matching.

CREATE TABLE IF NOT EXISTS rag_chunk_subvectors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chunk_id        TEXT NOT NULL,
    workspace_id    TEXT NOT NULL,
    collection_id   TEXT NOT NULL,
    segment_index   SMALLINT NOT NULL,
    segment_text    TEXT NOT NULL,
    embedding       vector(1536) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT rag_chunk_subvectors_unique UNIQUE (chunk_id, segment_index)
);

CREATE INDEX IF NOT EXISTS idx_rag_chunk_subvectors_chunk_id
    ON rag_chunk_subvectors (chunk_id);

CREATE INDEX IF NOT EXISTS idx_rag_chunk_subvectors_workspace
    ON rag_chunk_subvectors (workspace_id);
