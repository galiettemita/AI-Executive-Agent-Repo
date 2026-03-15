-- Migration: Knowledge graph table + entity embedding columns for HippoRAG retrieval.

CREATE TABLE IF NOT EXISTS memory_knowledge_graph (
    id               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    predicate        TEXT        NOT NULL,
    object           TEXT        NOT NULL,
    subject_type     TEXT        NOT NULL DEFAULT '',
    object_type      TEXT        NOT NULL DEFAULT '',
    confidence       NUMERIC(4,3) NOT NULL DEFAULT 0.80
        CHECK (confidence >= 0.0 AND confidence <= 1.0),
    source_turn_id   TEXT,
    subject_embedding vector(1536),
    object_embedding  vector(1536),
    subject_embedded_at TIMESTAMPTZ,
    object_embedded_at  TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT kg_no_self_loop CHECK (lower(trim(subject)) != lower(trim(object))),
    CONSTRAINT kg_unique_triple UNIQUE (workspace_id, lower(trim(subject)), predicate, lower(trim(object)))
);

CREATE INDEX IF NOT EXISTS idx_kg_subject_lower
    ON memory_knowledge_graph (workspace_id, lower(trim(subject)));

CREATE INDEX IF NOT EXISTS idx_kg_object_lower
    ON memory_knowledge_graph (workspace_id, lower(trim(object)));

CREATE INDEX IF NOT EXISTS idx_kg_predicate
    ON memory_knowledge_graph (workspace_id, predicate)
    WHERE confidence >= 0.7;

CREATE INDEX IF NOT EXISTS idx_kg_subject_embedding
    ON memory_knowledge_graph
    USING ivfflat (subject_embedding vector_cosine_ops)
    WHERE subject_embedding IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_kg_object_embedding
    ON memory_knowledge_graph
    USING ivfflat (object_embedding vector_cosine_ops)
    WHERE object_embedding IS NOT NULL;

COMMENT ON TABLE memory_knowledge_graph IS
    'HippoRAG knowledge graph: subject-predicate-object triples with entity embeddings.';
