package rag

import "context"

// Repository abstracts durable storage for RAG collections and retrievals.
// Production uses PgRepository; tests use the in-memory Service.
type Repository interface {
	// UpsertCollection creates or updates a collection.
	UpsertCollection(ctx context.Context, collection Collection) (Collection, error)
	// GetCollection retrieves a collection by ID.
	GetCollection(ctx context.Context, id string) (Collection, bool, error)
	// ListCollections returns all collections for a workspace.
	ListCollections(ctx context.Context, workspaceID string) ([]Collection, error)
	// DeleteCollection removes a collection and its chunks.
	DeleteCollection(ctx context.Context, id string) error
	// RecordRetrieval persists a retrieval result.
	RecordRetrieval(ctx context.Context, retrieval Retrieval) error
	// GetRetrieval retrieves a stored retrieval by turn ID.
	GetRetrieval(ctx context.Context, turnID string) (Retrieval, bool, error)
	// UpsertRerankerConfig saves reranker config for a workspace.
	UpsertRerankerConfig(ctx context.Context, config RerankerConfig) error
	// GetRerankerConfig retrieves reranker config for a workspace.
	GetRerankerConfig(ctx context.Context, workspaceID string) (RerankerConfig, bool, error)
}
