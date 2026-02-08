# Vector DB Options

Select one of the following for semantic search:

1. Pinecone (managed)
2. Weaviate (self-hosted or managed)
3. Postgres + pgvector (self-hosted)

Set VECTOR_DB_BACKEND accordingly and configure the provider keys.

## Namespaces in Use
- `emails` — semantic email search
- `files` — file embeddings (filename + extracted text + tags)
- `photos` — photo embeddings (tags + captions)
