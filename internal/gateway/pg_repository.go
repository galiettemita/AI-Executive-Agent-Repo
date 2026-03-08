package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
)

// PgDeduplicationRepository implements DeduplicationRepository using PostgreSQL.
type PgDeduplicationRepository struct {
	db database.Querier
}

// NewPgDeduplicationRepository creates a new PgDeduplicationRepository.
func NewPgDeduplicationRepository(db database.Querier) *PgDeduplicationRepository {
	return &PgDeduplicationRepository{db: db}
}

func (r *PgDeduplicationRepository) CheckAndStore(ctx context.Context, workspaceID string, dedupHash string, messageID string) (bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM gateway_dedup WHERE workspace_id = $1 AND dedup_hash = $2)`,
		workspaceID, dedupHash)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("check dedup hash: %w", err)
	}
	if exists {
		return true, nil
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO gateway_dedup (workspace_id, dedup_hash, message_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, dedup_hash) DO NOTHING`,
		workspaceID, dedupHash, messageID, time.Now().UTC())
	if err != nil {
		return false, fmt.Errorf("store dedup hash: %w", err)
	}
	return false, nil
}

func (r *PgDeduplicationRepository) StoreNonce(ctx context.Context, workspaceID string, nonce string, messageID string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO gateway_nonces (workspace_id, nonce, message_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (workspace_id, nonce) DO NOTHING`,
		workspaceID, nonce, messageID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("store nonce: %w", err)
	}
	return nil
}

func (r *PgDeduplicationRepository) IsNonceUsed(ctx context.Context, workspaceID string, nonce string) (bool, error) {
	row := r.db.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM gateway_nonces WHERE workspace_id = $1 AND nonce = $2)`,
		workspaceID, nonce)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("check nonce: %w", err)
	}
	return exists, nil
}

// PgMessageQueueRepository implements MessageQueueRepository using PostgreSQL.
type PgMessageQueueRepository struct {
	db database.Querier
}

// NewPgMessageQueueRepository creates a new PgMessageQueueRepository.
func NewPgMessageQueueRepository(db database.Querier) *PgMessageQueueRepository {
	return &PgMessageQueueRepository{db: db}
}

func (r *PgMessageQueueRepository) Enqueue(ctx context.Context, msg *QueueMessage) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO gateway_queue (ingress_turn_id, workspace_id, channel, channel_identifier,
			user_channel_id, group_key, dedup_key, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		msg.IngressTurnID, msg.WorkspaceID, msg.Channel, msg.ChannelIdentifier,
		msg.UserChannelID, msg.GroupKey, msg.DedupKey, msg.Payload, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("enqueue message: %w", err)
	}
	return nil
}

func (r *PgMessageQueueRepository) Dequeue(ctx context.Context, limit int) ([]QueueMessage, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.Query(ctx, `
		DELETE FROM gateway_queue
		WHERE ingress_turn_id IN (
			SELECT ingress_turn_id FROM gateway_queue
			ORDER BY created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ingress_turn_id, workspace_id, channel, channel_identifier,
			user_channel_id, group_key, dedup_key, payload`, limit)
	if err != nil {
		return nil, fmt.Errorf("dequeue messages: %w", err)
	}
	defer rows.Close()

	var msgs []QueueMessage
	for rows.Next() {
		var msg QueueMessage
		err := rows.Scan(&msg.IngressTurnID, &msg.WorkspaceID, &msg.Channel,
			&msg.ChannelIdentifier, &msg.UserChannelID, &msg.GroupKey,
			&msg.DedupKey, &msg.Payload)
		if err != nil {
			return nil, fmt.Errorf("scan queue message: %w", err)
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

func (r *PgMessageQueueRepository) Ack(ctx context.Context, messageID string) error {
	parsedID, err := uuid.Parse(messageID)
	if err != nil {
		return fmt.Errorf("invalid message id: %w", err)
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM gateway_queue WHERE ingress_turn_id = $1`, parsedID)
	if err != nil {
		return fmt.Errorf("ack message: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("message not found: %s", messageID)
	}
	return nil
}

// Compile-time interface compliance checks.
var (
	_ DeduplicationRepository = (*PgDeduplicationRepository)(nil)
	_ MessageQueueRepository  = (*PgMessageQueueRepository)(nil)
)
