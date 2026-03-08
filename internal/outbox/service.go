package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OutboxEntry status constants.
const (
	StatusPending    = "pending"
	StatusDispatched = "dispatched"
	StatusFailed     = "failed"
	StatusDLQ        = "dlq"
)

// OutboxEntry represents a single transactional outbox record.
type OutboxEntry struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	AggregateType string    `json:"aggregate_type"`
	AggregateID   string    `json:"aggregate_id"`
	EventType     string    `json:"event_type"`
	Payload       []byte    `json:"payload"`
	Target        string    `json:"target"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	MaxAttempts   int       `json:"max_attempts"`
	CreatedAt     time.Time `json:"created_at"`
	DispatchedAt  *time.Time `json:"dispatched_at,omitempty"`
	NextRetryAt   *time.Time `json:"next_retry_at,omitempty"`
	FailReason    string    `json:"fail_reason,omitempty"`
}

// Service provides transactional outbox operations backed by PostgreSQL.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a new outbox service with a pgx connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Enqueue inserts a new outbox entry within an existing database transaction.
// This ensures the outbox write is atomic with the business operation.
func (s *Service) Enqueue(ctx context.Context, tx pgx.Tx, entry OutboxEntry) error {
	if entry.ID == "" {
		return fmt.Errorf("outbox entry ID is required")
	}
	if entry.EventType == "" {
		return fmt.Errorf("outbox entry event_type is required")
	}
	if entry.Status == "" {
		entry.Status = StatusPending
	}
	if entry.MaxAttempts <= 0 {
		entry.MaxAttempts = 5
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	_, err := tx.Exec(ctx,
		`INSERT INTO outbox (id, workspace_id, aggregate_type, aggregate_id, event_type, payload, target, status, attempts, max_attempts, created_at, next_retry_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		entry.ID,
		entry.WorkspaceID,
		entry.AggregateType,
		entry.AggregateID,
		entry.EventType,
		entry.Payload,
		entry.Target,
		entry.Status,
		entry.Attempts,
		entry.MaxAttempts,
		entry.CreatedAt,
		entry.NextRetryAt,
	)
	return err
}

// FetchPending retrieves a batch of pending outbox entries ordered by creation time.
// It uses SELECT ... FOR UPDATE SKIP LOCKED to allow concurrent consumers.
func (s *Service) FetchPending(ctx context.Context, batchSize int) ([]OutboxEntry, error) {
	if batchSize <= 0 {
		batchSize = 100
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, workspace_id, aggregate_type, aggregate_id, event_type, payload, target, status, attempts, max_attempts, created_at, dispatched_at, next_retry_at
		 FROM outbox
		 WHERE status = $1
		   AND (next_retry_at IS NULL OR next_retry_at <= now())
		 ORDER BY created_at ASC
		 LIMIT $2
		 FOR UPDATE SKIP LOCKED`,
		StatusPending, batchSize,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox entries: %w", err)
	}
	defer rows.Close()

	var entries []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		err := rows.Scan(
			&e.ID, &e.WorkspaceID, &e.AggregateType, &e.AggregateID,
			&e.EventType, &e.Payload, &e.Target, &e.Status,
			&e.Attempts, &e.MaxAttempts, &e.CreatedAt, &e.DispatchedAt, &e.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox entries: %w", err)
	}
	return entries, nil
}

// MarkDispatched marks an outbox entry as successfully dispatched.
func (s *Service) MarkDispatched(ctx context.Context, entryID string) error {
	now := time.Now().UTC()
	tag, err := s.pool.Exec(ctx,
		`UPDATE outbox SET status = $1, dispatched_at = $2 WHERE id = $3`,
		StatusDispatched, now, entryID,
	)
	if err != nil {
		return fmt.Errorf("mark dispatched: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox entry %s not found", entryID)
	}
	return nil
}

// MarkFailed increments the attempt counter and records the failure reason.
// It computes an exponential backoff for the next retry.
func (s *Service) MarkFailed(ctx context.Context, entryID string, reason string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE outbox
		 SET status = $1,
		     attempts = attempts + 1,
		     fail_reason = $2,
		     next_retry_at = now() + (interval '1 second' * power(2, attempts))
		 WHERE id = $3`,
		StatusFailed, reason, entryID,
	)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox entry %s not found", entryID)
	}

	// Check if the entry has exceeded max attempts and should move to DLQ.
	var attempts, maxAttempts int
	err = s.pool.QueryRow(ctx,
		`SELECT attempts, max_attempts FROM outbox WHERE id = $1`, entryID,
	).Scan(&attempts, &maxAttempts)
	if err != nil {
		return nil // Entry was updated, just can't check DLQ condition.
	}
	if attempts >= maxAttempts {
		return s.MoveToDLQ(ctx, entryID)
	}

	// Reset status to pending for retry.
	_, err = s.pool.Exec(ctx,
		`UPDATE outbox SET status = $1 WHERE id = $2 AND status = $3`,
		StatusPending, entryID, StatusFailed,
	)
	return err
}

// MoveToDLQ moves an outbox entry to dead-letter queue status.
func (s *Service) MoveToDLQ(ctx context.Context, entryID string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE outbox SET status = $1 WHERE id = $2`,
		StatusDLQ, entryID,
	)
	if err != nil {
		return fmt.Errorf("move to DLQ: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("outbox entry %s not found", entryID)
	}
	return nil
}

// ProcessBatch fetches a batch of pending entries, dispatches each using the
// provided dispatcher function, and marks entries as dispatched or failed.
func (s *Service) ProcessBatch(ctx context.Context, batchSize int, dispatcher func(OutboxEntry) error) (dispatched int, failed int, err error) {
	entries, err := s.FetchPending(ctx, batchSize)
	if err != nil {
		return 0, 0, fmt.Errorf("process batch fetch: %w", err)
	}

	for _, entry := range entries {
		dispatchErr := dispatcher(entry)
		if dispatchErr != nil {
			markErr := s.MarkFailed(ctx, entry.ID, dispatchErr.Error())
			if markErr != nil {
				// Log but continue processing remaining entries.
				_ = markErr
			}
			failed++
			continue
		}
		markErr := s.MarkDispatched(ctx, entry.ID)
		if markErr != nil {
			_ = markErr
			failed++
			continue
		}
		dispatched++
	}

	return dispatched, failed, nil
}

// PurgeDispatched removes dispatched entries older than the specified duration.
func (s *Service) PurgeDispatched(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM outbox WHERE status = $1 AND dispatched_at < $2`,
		StatusDispatched, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("purge dispatched: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
