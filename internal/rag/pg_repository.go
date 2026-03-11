package rag

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
)

// PgRepository implements Repository backed by PostgreSQL for RAG collections,
// retrievals, and reranker config. Chunk storage and vector search are handled
// by PgVectorProdStore separately.
type PgRepository struct {
	db database.Querier
}

// NewPgRepository creates a production RAG repository.
func NewPgRepository(db database.Querier) *PgRepository {
	return &PgRepository{db: db}
}

func (r *PgRepository) UpsertCollection(ctx context.Context, collection Collection) (Collection, error) {
	wsID := normalizeWorkspaceID(collection.WorkspaceID)
	collection.WorkspaceID = wsID
	if collection.ID == "" {
		collection.ID = uuid.Must(uuid.NewV7()).String()
	}
	collection.CollectionID = collection.ID

	configJSON, _ := json.Marshal(map[string]any{
		"embedding_model": collection.EmbeddingModel,
		"chunk_size":      collection.ChunkSize,
		"bm25_enabled":    collection.BM25Enabled,
		"name":            collection.Name,
		"description":     collection.Description,
		"status":          collection.Status,
	})

	_, err := r.db.Exec(ctx, `
		INSERT INTO rag_collections (id, workspace_id, collection_key, config_json)
		VALUES ($1::uuid, $2::uuid, $3, $4)
		ON CONFLICT (workspace_id, collection_key) DO UPDATE SET
			config_json = EXCLUDED.config_json`,
		collection.ID, wsID, collection.ID, configJSON)
	if err != nil {
		return Collection{}, fmt.Errorf("upsert collection: %w", err)
	}
	return collection, nil
}

func (r *PgRepository) GetCollection(ctx context.Context, id string) (Collection, bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, collection_key, config_json
		FROM rag_collections WHERE id = $1::uuid`, id)

	var c Collection
	var configJSON []byte
	err := row.Scan(&c.ID, &c.WorkspaceID, &c.CollectionID, &configJSON)
	if err != nil {
		return Collection{}, false, nil
	}
	parseCollectionConfig(&c, configJSON)
	return c, true, nil
}

func (r *PgRepository) ListCollections(ctx context.Context, workspaceID string) ([]Collection, error) {
	wsID := normalizeWorkspaceID(workspaceID)
	rows, err := r.db.Query(ctx, `
		SELECT id, workspace_id, collection_key, config_json
		FROM rag_collections WHERE workspace_id = $1::uuid ORDER BY id`, wsID)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()

	var out []Collection
	for rows.Next() {
		var c Collection
		var configJSON []byte
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.CollectionID, &configJSON); err != nil {
			return nil, fmt.Errorf("scan collection: %w", err)
		}
		parseCollectionConfig(&c, configJSON)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *PgRepository) DeleteCollection(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM rag_collections WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete collection: %w", err)
	}
	return nil
}

func (r *PgRepository) RecordRetrieval(ctx context.Context, retrieval Retrieval) error {
	wsID := normalizeWorkspaceID(retrieval.WorkspaceID)
	retrievalJSON, _ := json.Marshal(retrieval)

	_, err := r.db.Exec(ctx, `
		INSERT INTO rag_retrievals (id, workspace_id, mode, retrieval_json)
		VALUES ($1::uuid, $2::uuid, 'hybrid', $3)`,
		retrieval.RetrievalID, wsID, retrievalJSON)
	if err != nil {
		return fmt.Errorf("record retrieval: %w", err)
	}
	return nil
}

func (r *PgRepository) GetRetrieval(ctx context.Context, turnID string) (Retrieval, bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT retrieval_json FROM rag_retrievals WHERE id = $1::uuid`, turnID)

	var retrievalJSON []byte
	err := row.Scan(&retrievalJSON)
	if err != nil {
		return Retrieval{}, false, nil
	}
	var ret Retrieval
	_ = json.Unmarshal(retrievalJSON, &ret)
	return ret, true, nil
}

func (r *PgRepository) UpsertRerankerConfig(ctx context.Context, config RerankerConfig) error {
	wsID := normalizeWorkspaceID(config.WorkspaceID)
	configJSON, _ := json.Marshal(config)

	_, err := r.db.Exec(ctx, `
		INSERT INTO rag_reranker_config (workspace_id, config_json)
		VALUES ($1::uuid, $2)
		ON CONFLICT (workspace_id) DO UPDATE SET config_json = EXCLUDED.config_json`,
		wsID, configJSON)
	if err != nil {
		return fmt.Errorf("upsert reranker config: %w", err)
	}
	return nil
}

func (r *PgRepository) GetRerankerConfig(ctx context.Context, workspaceID string) (RerankerConfig, bool, error) {
	wsID := normalizeWorkspaceID(workspaceID)
	row := r.db.QueryRow(ctx, `
		SELECT config_json FROM rag_reranker_config WHERE workspace_id = $1::uuid`, wsID)

	var configJSON []byte
	err := row.Scan(&configJSON)
	if err != nil {
		return RerankerConfig{}, false, nil
	}
	var cfg RerankerConfig
	_ = json.Unmarshal(configJSON, &cfg)
	return cfg, true, nil
}

func parseCollectionConfig(c *Collection, configJSON []byte) {
	if len(configJSON) == 0 {
		return
	}
	var cfg map[string]any
	_ = json.Unmarshal(configJSON, &cfg)
	if v, ok := cfg["name"].(string); ok {
		c.Name = v
	}
	if v, ok := cfg["description"].(string); ok {
		c.Description = v
	}
	if v, ok := cfg["embedding_model"].(string); ok {
		c.EmbeddingModel = v
	}
	if v, ok := cfg["chunk_size"].(float64); ok {
		c.ChunkSize = int(v)
	}
	if v, ok := cfg["bm25_enabled"].(bool); ok {
		c.BM25Enabled = v
	}
	if v, ok := cfg["status"].(string); ok {
		c.Status = v
	}
}
