package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgvector "github.com/pgvector/pgvector-go"
)

// PgItemRepository implements ItemRepository using PostgreSQL with pgvector.
type PgItemRepository struct {
	db database.Querier
}

// NewPgItemRepository creates a new PgItemRepository.
func NewPgItemRepository(db database.Querier) *PgItemRepository {
	return &PgItemRepository{db: db}
}

func (r *PgItemRepository) Store(ctx context.Context, item *Item) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO memory_items (id, workspace_id, user_id, memory_type, status, body,
			data_class, sensitivity_label, retention_policy_id, allowed_processors,
			content_trust, embedding_version, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (id) DO UPDATE SET
			status = EXCLUDED.status,
			body = EXCLUDED.body,
			data_class = EXCLUDED.data_class,
			sensitivity_label = EXCLUDED.sensitivity_label,
			allowed_processors = EXCLUDED.allowed_processors,
			content_trust = EXCLUDED.content_trust,
			embedding_version = EXCLUDED.embedding_version,
			expires_at = EXCLUDED.expires_at,
			updated_at = EXCLUDED.updated_at`,
		item.ID, item.WorkspaceID, item.UserID, item.MemoryType, item.Status, item.Body,
		item.DataClass, item.SensitivityLabel, item.RetentionPolicyID, item.AllowedProcessors,
		item.ContentTrust, item.EmbeddingVersion, item.ExpiresAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store memory item: %w", err)
	}
	return nil
}

func (r *PgItemRepository) GetByID(ctx context.Context, id string) (*Item, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid item id: %w", err)
	}
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, user_id, memory_type, status, body,
			data_class, sensitivity_label, retention_policy_id, allowed_processors,
			content_trust, embedding_version, expires_at, created_at, updated_at
		FROM memory_items WHERE id = $1`, parsedID)

	item := &Item{}
	err = row.Scan(&item.ID, &item.WorkspaceID, &item.UserID, &item.MemoryType,
		&item.Status, &item.Body, &item.DataClass, &item.SensitivityLabel,
		&item.RetentionPolicyID, &item.AllowedProcessors, &item.ContentTrust,
		&item.EmbeddingVersion, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("memory item not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get memory item: %w", err)
	}
	return item, nil
}

func (r *PgItemRepository) ListByWorkspace(ctx context.Context, workspaceID string, limit int) ([]Item, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, workspace_id, user_id, memory_type, status, body,
			data_class, sensitivity_label, retention_policy_id, allowed_processors,
			content_trust, embedding_version, expires_at, created_at, updated_at
		FROM memory_items
		WHERE workspace_id = $1
		ORDER BY created_at ASC
		LIMIT $2`, workspaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list memory items: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.ID, &item.WorkspaceID, &item.UserID, &item.MemoryType,
			&item.Status, &item.Body, &item.DataClass, &item.SensitivityLabel,
			&item.RetentionPolicyID, &item.AllowedProcessors, &item.ContentTrust,
			&item.EmbeddingVersion, &item.ExpiresAt, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory item: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *PgItemRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid item id: %w", err)
	}
	tag, err := r.db.Exec(ctx, `
		UPDATE memory_items SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), parsedID)
	if err != nil {
		return fmt.Errorf("update memory item status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory item not found: %s", id)
	}
	return nil
}

func (r *PgItemRepository) FindSimilarByEmbedding(ctx context.Context, workspaceID string, embedding []float32, limit int, minScore float64) ([]ScoredItem, error) {
	if limit <= 0 {
		limit = 10
	}
	vec := pgvector.NewVector(embedding)
	rows, err := r.db.Query(ctx, `
		SELECT id, workspace_id, user_id, memory_type, status, body,
			data_class, sensitivity_label, retention_policy_id, allowed_processors,
			content_trust, embedding_version, expires_at, created_at, updated_at,
			1 - (embedding <=> $1) AS similarity
		FROM memory_items
		WHERE workspace_id = $2 AND 1 - (embedding <=> $1) >= $3
		ORDER BY embedding <=> $1
		LIMIT $4`, vec, workspaceID, minScore, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar memory items: %w", err)
	}
	defer rows.Close()

	var results []ScoredItem
	for rows.Next() {
		var si ScoredItem
		err := rows.Scan(&si.Item.ID, &si.Item.WorkspaceID, &si.Item.UserID,
			&si.Item.MemoryType, &si.Item.Status, &si.Item.Body,
			&si.Item.DataClass, &si.Item.SensitivityLabel, &si.Item.RetentionPolicyID,
			&si.Item.AllowedProcessors, &si.Item.ContentTrust, &si.Item.EmbeddingVersion,
			&si.Item.ExpiresAt, &si.Item.CreatedAt, &si.Item.UpdatedAt, &si.Similarity)
		if err != nil {
			return nil, fmt.Errorf("scan similar memory item: %w", err)
		}
		results = append(results, si)
	}
	return results, rows.Err()
}

func (r *PgItemRepository) Delete(ctx context.Context, id string) error {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid item id: %w", err)
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM memory_items WHERE id = $1`, parsedID)
	if err != nil {
		return fmt.Errorf("delete memory item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory item not found: %s", id)
	}
	return nil
}
