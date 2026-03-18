-- migrations/063_archival_memory.up.sql
-- MemGPT-style paged infinite context: archival memory storage.

CREATE TABLE IF NOT EXISTS archival_memories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      TEXT NOT NULL,
    workspace_id    UUID NOT NULL,
    content         TEXT NOT NULL,
    embedding       vector(1536) NOT NULL,
    page_generation INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    source_turns    INT[] NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_archival_memories_session
    ON archival_memories (session_id, workspace_id);

CREATE INDEX IF NOT EXISTS idx_archival_memories_embedding
    ON archival_memories
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100);
