-- ============================================================
-- Migration 039 — Semantic LLM Response Cache
-- Purpose: Stores LLM responses keyed by embedding vector for
--          cosine-similarity-based cache lookup via pgvector.
-- ============================================================

CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

CREATE TABLE llm_semantic_cache (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  UUID          NOT NULL,
    query_hash    VARCHAR(64)   NOT NULL,          -- sha256(lower(trim(query_text)))
    query_text    TEXT          NOT NULL,
    embedding     vector(1536),                    -- OpenAI text-embedding-3-small output dim
    response      TEXT          NOT NULL,
    model         VARCHAR(128)  NOT NULL,
    intent        VARCHAR(256),
    hit_count     INTEGER       NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ   NOT NULL,
    CONSTRAINT llm_semantic_cache_unique UNIQUE (workspace_id, query_hash)
);

-- ── Row Level Security ─────────────────────────────────────
ALTER TABLE llm_semantic_cache ENABLE ROW LEVEL SECURITY;

CREATE POLICY llm_semantic_cache_workspace_isolation
    ON  llm_semantic_cache
    USING (workspace_id = current_setting('app.workspace_id', true)::uuid);

-- ── Indexes ────────────────────────────────────────────────
-- IVFFlat for sub-10ms cosine similarity search at scale.
-- lists=50 suits tables up to ~500k rows; increase for larger datasets.
CREATE INDEX llm_semantic_cache_embedding_idx
    ON llm_semantic_cache
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 50);

-- Supports efficient TTL cleanup queries.
CREATE INDEX llm_semantic_cache_expires_at_idx
    ON llm_semantic_cache (expires_at);

-- ── Cleanup Function ───────────────────────────────────────
-- Call from pg_cron or a scheduled job. Returns count of deleted rows.
CREATE OR REPLACE FUNCTION cleanup_llm_semantic_cache()
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    deleted_count integer;
BEGIN
    DELETE FROM llm_semantic_cache WHERE expires_at < NOW();
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    RETURN deleted_count;
END;
$$;
