-- Migration 018: V10.2 Memory, Context, RAG, and Latency Gap Closure
-- Prompt 8 of 12 — DB persistence for decay, conflict resolution,
-- compression artifacts, embedding chunking spec, and latency budget.

BEGIN;

-- -----------------------------------------------------------------------
-- T8.1  Memory decay log — persists decay sweep results
-- -----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS memory_decay_log (
    id              UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id    UUID NOT NULL,
    decay_function  TEXT NOT NULL DEFAULT 'exponential',
    half_life_days  NUMERIC(10,2) NOT NULL,
    items_decayed   INT NOT NULL DEFAULT 0,
    items_purged    INT NOT NULL DEFAULT 0,
    min_weight      NUMERIC(6,4) NOT NULL DEFAULT 0.05,
    swept_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_memory_decay_log_ws ON memory_decay_log (workspace_id, swept_at DESC);

-- -----------------------------------------------------------------------
-- T8.2  Lesson conflict detection + resolution persistence
-- -----------------------------------------------------------------------
CREATE TYPE lesson_conflict_resolution AS ENUM (
    'newer_wins', 'higher_confidence', 'manual_review', 'merged'
);

CREATE TABLE IF NOT EXISTS lesson_conflicts (
    id                UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id      UUID NOT NULL,
    existing_lesson_id UUID NOT NULL,
    incoming_lesson_id UUID,
    conflict_type     TEXT NOT NULL,
    resolution        lesson_conflict_resolution NOT NULL DEFAULT 'manual_review',
    resolved_by       TEXT,
    resolution_detail TEXT,
    resolved_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_lesson_conflicts_ws ON lesson_conflicts (workspace_id, created_at DESC);

-- -----------------------------------------------------------------------
-- T8.3  Embedding chunking spec — persists chunking strategy per collection
-- -----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS embedding_chunk_specs (
    id              UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id    UUID NOT NULL,
    collection_id   UUID,
    chunk_strategy  TEXT NOT NULL DEFAULT 'fixed_token',
    chunk_size      INT NOT NULL DEFAULT 512,
    chunk_overlap   INT NOT NULL DEFAULT 64,
    embedding_model TEXT NOT NULL DEFAULT 'text-embedding-3-small',
    dimensions      INT NOT NULL DEFAULT 1536,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, collection_id)
);

-- -----------------------------------------------------------------------
-- T8.6  Compression artifacts — auditable conversation compression
-- -----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS compression_artifacts (
    id                  UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id        UUID NOT NULL,
    session_id          TEXT NOT NULL,
    original_turn_count INT NOT NULL,
    compressed_count    INT NOT NULL,
    entity_refs         JSONB NOT NULL DEFAULT '[]',
    summary_text        TEXT NOT NULL,
    token_savings       INT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_compression_artifacts_ws ON compression_artifacts (workspace_id, session_id, created_at DESC);

-- -----------------------------------------------------------------------
-- T8.8  Latency budget log — persists preemption decisions
-- -----------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS latency_budget_log (
    id                  UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    workspace_id        UUID NOT NULL,
    workflow_run_id     TEXT NOT NULL,
    budget_ms           NUMERIC(10,2) NOT NULL,
    elapsed_ms          NUMERIC(10,2) NOT NULL,
    estimated_next_ms   NUMERIC(10,2) NOT NULL,
    should_proceed      BOOLEAN NOT NULL,
    reason              TEXT NOT NULL,
    remaining_budget_ms NUMERIC(10,2) NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_latency_budget_log_ws ON latency_budget_log (workspace_id, created_at DESC);

COMMIT;
