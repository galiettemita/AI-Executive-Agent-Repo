package gateway

import "context"

// DeduplicationRepository persists message deduplication state.
type DeduplicationRepository interface {
	CheckAndStore(ctx context.Context, workspaceID string, dedupHash string, messageID string) (isDuplicate bool, err error)
	StoreNonce(ctx context.Context, workspaceID string, nonce string, messageID string) error
	IsNonceUsed(ctx context.Context, workspaceID string, nonce string) (bool, error)
}

// MessageQueueRepository persists inbound message queue.
type MessageQueueRepository interface {
	Enqueue(ctx context.Context, msg *QueueMessage) error
	Dequeue(ctx context.Context, limit int) ([]QueueMessage, error)
	Ack(ctx context.Context, messageID string) error
}
