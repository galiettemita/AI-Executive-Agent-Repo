-- Generic pgvector store for semantic search assets (files/photos/emails)
CREATE TABLE IF NOT EXISTS vector_store_items (
  id TEXT NOT NULL,
  namespace TEXT NOT NULL,
  embedding vector(1536) NOT NULL,
  metadata JSONB DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ DEFAULT now(),
  updated_at TIMESTAMPTZ DEFAULT now(),
  PRIMARY KEY (id, namespace)
);

CREATE INDEX IF NOT EXISTS idx_vector_store_embedding
  ON vector_store_items USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 64);

CREATE INDEX IF NOT EXISTS idx_vector_store_metadata
  ON vector_store_items USING gin (metadata jsonb_path_ops);
