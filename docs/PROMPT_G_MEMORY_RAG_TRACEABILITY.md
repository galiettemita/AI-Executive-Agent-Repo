# Prompt G — Memory + RAG Traceability Map

## Component → Schema → Go Code → Tests

| Component | SQL Table / Column | Migration | Go Code | Tests | Status |
|-----------|-------------------|-----------|---------|-------|--------|
| memory_items.id | `id uuid PK` | 001 L236 | `pg_repository.go:25` | `pg_repository_test.go` | ✓ OK |
| memory_items.workspace_id | `workspace_id uuid NOT NULL` | 001 L237 | `pg_repository.go:25` | ✓ | ✓ OK |
| memory_items.user_id | **❌ NOT IN SCHEMA** | — | `pg_repository.go:26` expects it | — | **BROKEN** |
| memory_items.memory_type | `memory_type memory_type` | 001 L238 | `pg_repository.go:26` | ✓ | ✓ OK |
| memory_items.status | `status memory_status` | 001 L239 | `pg_repository.go:26` | ✓ | ✓ OK |
| memory_items.body | `body text NOT NULL` | 001 L240 | `pg_repository.go:26` | ✓ | ✓ OK |
| memory_items.embedding | `embedding vector(1536)` | 001 L241 | **❌ Not written in Store()** | — | **UNUSED** |
| memory_items.data_class | `data_class data_class` | 001 L242 | `pg_repository.go:27` | ✓ | ✓ OK |
| memory_items.sensitivity_label | `sensitivity_label sensitivity_label` | 001 L243 | `pg_repository.go:27` | ✓ | ✓ OK |
| memory_items.retention_policy_id | `retention_policy_id text` | 001 L244 | `pg_repository.go:27` | ✓ | ✓ OK |
| memory_items.allowed_processors | `allowed_processors text[]` | 001 L245 | `pg_repository.go:28` | ✓ | ✓ OK |
| memory_items.content_trust | `content_trust content_trust` | 001 L246 | `pg_repository.go:28` | ✓ | ✓ OK |
| memory_items.embedding_version | **❌ NOT IN SCHEMA** | — | `pg_repository.go:28` expects it | — | **BROKEN** |
| memory_items.expires_at | **❌ NOT IN SCHEMA** | — | `pg_repository.go:28` expects it | — | **BROKEN** |
| memory_items vector index | **❌ NO INDEX** | — | `pg_repository.go:130` uses `<=>` | — | **MISSING** |
| rag_chunks.id | `id uuid PK` | 003 L74 | Go expects `chunk_id` | — | **MISMATCH** |
| rag_chunks.workspace_id | `workspace_id uuid NOT NULL` | 003 L75 | **❌ Not referenced in Go INSERT** | — | **MISMATCH** |
| rag_chunks.rag_collection_id | `rag_collection_id uuid` | 003 L76 | Go expects `collection_id` | — | **MISMATCH** |
| rag_chunks.chunk_text | `chunk_text text NOT NULL` | 003 L77 | Go expects `content` | — | **MISMATCH** |
| rag_chunks.bm25_tokens | `bm25_tokens tsvector` | 003 L78 | **❌ Not populated by Go** | — | **UNUSED** |
| rag_chunks.embedding | `embedding vector(1536)` | 003 L79 | `pg_vector_store.go:44` `$4::vector` | ✓ | ✓ OK |
| rag_chunks.metadata | **❌ NOT IN SCHEMA** | — | `pg_vector_store.go:44` tries INSERT | — | **BROKEN** |
| rag_chunks.status | `status rag_chunk_status` | 003 L80 | **❌ Not set in Go INSERT** | — | **UNUSED** |
| rag_chunks HNSW index | `idx_rag_chunks_embedding_hnsw` | 003 L529 | `pg_vector_store.go:66` uses `<=>` | ✓ | ✓ OK |
| memory_vector_clocks | See below | 001 | `pg_repository.go` | — | **NEEDS AUDIT** |
| embedding_chunk_specs | Full table | 018 L47-59 | `pg_chunk_spec_repository.go` | ✓ | ✓ OK |

## memory_items Mismatch Evidence

### Go code expects columns not in SQL schema

**Go INSERT** (`internal/memory/pg_repository.go:25-28`):
```go
INSERT INTO memory_items (id, workspace_id, user_id, memory_type, status, body,
  data_class, sensitivity_label, retention_policy_id, allowed_processors,
  content_trust, embedding_version, expires_at, created_at, updated_at)
```

**SQL schema** (`db/migrations/001_BREVIO_v9_init.sql:236-250`):
```sql
CREATE TABLE memory_items (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  memory_type memory_type NOT NULL,
  status memory_status NOT NULL DEFAULT 'proposed',
  body text NOT NULL,
  embedding vector(1536),
  data_class data_class NOT NULL DEFAULT 'internal',
  sensitivity_label sensitivity_label NOT NULL DEFAULT 'low',
  retention_policy_id text NOT NULL DEFAULT 'default',
  allowed_processors text[] NOT NULL DEFAULT ARRAY[]::text[],
  content_trust content_trust NOT NULL DEFAULT 'mixed',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
```

**Missing columns:** `user_id`, `embedding_version`, `expires_at`

**Unused SQL column:** `embedding vector(1536)` — defined in schema but Store() never writes to it.

**Missing index:** No HNSW/IVFFlat index on `memory_items.embedding`. FindSimilarByEmbedding() uses cosine `<=>` operator which will do sequential scan.

## rag_chunks Mismatch Evidence

### Column name mismatches between SQL and Go

| SQL Column Name | Go Code Expects | Impact |
|----------------|-----------------|--------|
| `id` | `chunk_id` | INSERT will fail: column "chunk_id" does not exist |
| `rag_collection_id` | `collection_id` | INSERT will fail: column "collection_id" does not exist |
| `chunk_text` | `content` | INSERT will fail: column "content" does not exist |
| (none) | `metadata` | INSERT will fail: column "metadata" does not exist |
| `workspace_id` | (not referenced) | NOT NULL constraint may fail |
| `bm25_tokens` | (not populated) | tsvector never built |
| `status` | (not set) | Uses default 'pending' |

**Go INSERT** (`internal/rag/pg_vector_store.go:41-46`):
```go
INSERT INTO rag_chunks (chunk_id, collection_id, content, embedding, metadata)
VALUES ($1, $2, $3, $4::vector, $5)
```

**SQL schema** (`db/migrations/003_BREVIO_v92_production_hardening.sql:74-83`):
```sql
CREATE TABLE rag_chunks (
  id uuid PRIMARY KEY DEFAULT uuid_v7_generate(),
  workspace_id uuid NOT NULL REFERENCES workspaces(id),
  rag_collection_id uuid NOT NULL REFERENCES rag_collections(id),
  chunk_text text NOT NULL,
  bm25_tokens tsvector,
  embedding vector(1536),
  status rag_chunk_status NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);
```

## memory_vector_clocks Mismatch Evidence

**SQL schema** (`db/migrations/001_BREVIO_v9_init.sql`): Defines `memory_vector_clocks` table with columns for vector clock-based conflict resolution.

**Go code** (`internal/memory/pg_repository.go`): References this table for clock operations. Needs full audit to verify column alignment once memory_items fixes are in place.

## pgvector-go Dependency Status

**go.mod**: Does **NOT** contain `github.com/pgvector/pgvector-go`. Vector values are passed as `[]float32` slices and rely on implicit `::vector` SQL casts. This works but:
- No compile-time type safety for vector dimensions
- No pgx AfterConnect type registration
- No `pgvector.Vector` type available

**Required action**: Add `github.com/pgvector/pgvector-go` and wire `pgvector.RegisterTypes` via pgx AfterConnect callback.

## Pool / RLS Bypass Risk

**`cmd/temporal-worker/main.go:67`** uses raw `pgxpool.New()` which bypasses the custom `internal/database.Pool` wrapper that enforces workspace RLS via `SET LOCAL brevio.workspace_id`.

## Decisions Compliance

| Decision | Requirement | Current Status |
|----------|-------------|----------------|
| D4 | workspace_id universal tenant key via RLS | ✓ RLS policies exist in SQL; ⚠️ bypassed in temporal-worker |
| D5 | UUIDv7 for new PKs | ✓ All tables use `uuid_v7_generate()` |
| D6 | Forward-only migrations | ✓ migrate.sh rejects "down" files |
| D9 | All state in PostgreSQL via pgx | ✓ No in-memory production wiring |
| D10 | text-embedding-3-small, 1536 dims, pgvector, cosine `<=>` | ✓ Consistent across embeddings.go + SQL |

## Fix Plan (Segments 1-8)

1. **Segment 1**: Add pgvector-go to go.mod, wire AfterConnect type registration
2. **Segment 2**: Vector encoding/decoding helpers using pgvector.Vector
3. **Segment 3**: Forward migration adding `user_id`, `embedding_version`, `expires_at` to memory_items; fix rag_chunks Go code to match SQL column names
4. **Segment 4**: Embedding caching with TTL (L1 in-process, L2 Redis)
5. **Segment 5**: HNSW index on memory_items.embedding, ops docs
6. **Segment 6**: Wire real RAG ingestion/retrieval into Temporal activities
7. **Segment 7**: Integration tests against real Postgres+pgvector
8. **Segment 8**: Self-audit, no-stubs check, RUNBOOK update
