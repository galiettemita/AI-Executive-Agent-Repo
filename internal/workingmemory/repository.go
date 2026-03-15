package workingmemory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Repository handles Redis persistence for working memory Items.
type Repository struct {
	redis RedisClient
}

func NewRepository(redis RedisClient) *Repository {
	if redis == nil {
		panic("workingmemory.NewRepository: redis must not be nil")
	}
	return &Repository{redis: redis}
}

func (r *Repository) key(workspaceID, taskID string) string {
	return fmt.Sprintf("%s:%s:%s", KeyPrefix, workspaceID, taskID)
}

// Upsert writes item to Redis. Sets TTL based on whether WorkflowID is bound.
func (r *Repository) Upsert(ctx context.Context, item *Item) error {
	now := time.Now().UTC()
	item.UpdatedAt = now
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.TTL <= 0 {
		item.TTL = DefaultTTL
	}
	if item.WorkflowID != "" {
		item.TTL = WorkflowTTL
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("workingmemory upsert: marshal: %w", err)
	}

	return r.redis.Set(ctx, r.key(item.WorkspaceID, item.TaskID), data, item.TTL)
}

// Get retrieves an Item. Returns nil, nil if the key does not exist.
func (r *Repository) Get(ctx context.Context, workspaceID, taskID string) (*Item, error) {
	data, err := r.redis.Get(ctx, r.key(workspaceID, taskID))
	if err != nil {
		return nil, fmt.Errorf("workingmemory get: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	var item Item
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("workingmemory get: unmarshal: %w", err)
	}
	return &item, nil
}

// Evict deletes the item for a completed task.
func (r *Repository) Evict(ctx context.Context, workspaceID, taskID string) error {
	return r.redis.Del(ctx, r.key(workspaceID, taskID))
}

// EvictByWorkflowID deletes all items for a given Temporal workflow run.
func (r *Repository) EvictByWorkflowID(ctx context.Context, workspaceID, workflowID string) error {
	pattern := fmt.Sprintf("%s:%s:*", KeyPrefix, workspaceID)
	keys, err := r.redis.Scan(ctx, pattern)
	if err != nil {
		return fmt.Errorf("workingmemory evict_by_workflow: scan: %w", err)
	}

	for _, k := range keys {
		data, err := r.redis.Get(ctx, k)
		if err != nil || data == nil {
			continue
		}
		var item Item
		if json.Unmarshal(data, &item) == nil && item.WorkflowID == workflowID {
			_ = r.redis.Del(ctx, k)
		}
	}
	return nil
}

// RefreshTTL resets TTL without changing content.
func (r *Repository) RefreshTTL(ctx context.Context, workspaceID, taskID string, ttl time.Duration) error {
	return r.redis.Expire(ctx, r.key(workspaceID, taskID), ttl)
}

// taskIDFromKey extracts taskID from a raw Redis key.
func taskIDFromKey(key string) string {
	parts := strings.SplitN(key, ":", 3)
	if len(parts) == 3 {
		return parts[2]
	}
	return ""
}
