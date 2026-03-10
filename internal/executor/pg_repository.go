package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ToolExecutionRepository persists tool executions with idempotency enforcement.
type ToolExecutionRepository interface {
	// InsertExecution inserts a new execution or returns the existing one if
	// the idempotency key already exists. Returns (exec, created, error).
	InsertExecution(ctx context.Context, exec *ToolExecution) (ToolExecution, bool, error)

	// GetExecution retrieves a tool execution by ID.
	GetExecution(ctx context.Context, id uuid.UUID) (*ToolExecution, error)

	// InsertReceipt persists a trust receipt for a committed execution.
	// Enforces one-receipt-per-execution via unique constraint.
	InsertReceipt(ctx context.Context, receipt *TrustReceipt) error

	// GetReceiptByExecution retrieves the trust receipt for an execution.
	GetReceiptByExecution(ctx context.Context, executionID uuid.UUID) (*TrustReceipt, error)

	// IncrementSideEffect atomically increments the side effect counter and
	// returns (before, after) counts.
	IncrementSideEffect(ctx context.Context, workspaceID, toolKey string) (int, int, error)

	// GetSideEffectCount returns the current side effect count.
	GetSideEffectCount(ctx context.Context, workspaceID, toolKey string) (int, error)

	// InsertAuditEntry persists an executor audit log entry with chain hash.
	InsertAuditEntry(ctx context.Context, entry *AuditLogEntry) error
}

// PgToolExecutionRepository implements ToolExecutionRepository using pgx.
type PgToolExecutionRepository struct {
	db database.Querier
}

var _ ToolExecutionRepository = (*PgToolExecutionRepository)(nil)

// NewPgToolExecutionRepository creates a pgx-backed tool execution repository.
func NewPgToolExecutionRepository(db database.Querier) *PgToolExecutionRepository {
	return &PgToolExecutionRepository{db: db}
}

func (r *PgToolExecutionRepository) InsertExecution(ctx context.Context, exec *ToolExecution) (ToolExecution, bool, error) {
	if exec.ID == uuid.Nil {
		exec.ID = uuid.Must(uuid.NewV7())
	}
	if exec.CreatedAt.IsZero() {
		exec.CreatedAt = time.Now().UTC()
	}

	// ON CONFLICT on idempotency_key: return existing row without modifying.
	row := r.db.QueryRow(ctx, `
		WITH ins AS (
			INSERT INTO tool_executions (
				id, phase, workspace_id, tool_key, logical_action,
				idempotency_key, provider, is_mcp, mcp_server_id,
				content_provenance, pii_content, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (idempotency_key) DO NOTHING
			RETURNING id, phase, workspace_id, tool_key, logical_action,
				idempotency_key, provider, is_mcp, mcp_server_id,
				content_provenance, pii_content, created_at, true AS created
		)
		SELECT * FROM ins
		UNION ALL
		SELECT id, phase, workspace_id, tool_key, logical_action,
			idempotency_key, provider, is_mcp, mcp_server_id,
			content_provenance, pii_content, created_at, false AS created
		FROM tool_executions
		WHERE idempotency_key = $6
			AND NOT EXISTS (SELECT 1 FROM ins)
		LIMIT 1`,
		exec.ID, string(exec.Phase), exec.WorkspaceID, exec.ToolKey,
		exec.LogicalAction, exec.IdempotencyKey, exec.Provider,
		exec.IsMCP, exec.MCPServerID, exec.ContentProvenance,
		exec.PIIContent, exec.CreatedAt,
	)

	var result ToolExecution
	var created bool
	err := row.Scan(
		&result.ID, &result.Phase, &result.WorkspaceID, &result.ToolKey,
		&result.LogicalAction, &result.IdempotencyKey, &result.Provider,
		&result.IsMCP, &result.MCPServerID, &result.ContentProvenance,
		&result.PIIContent, &result.CreatedAt, &created,
	)
	if err != nil {
		return ToolExecution{}, false, fmt.Errorf("insert execution: %w", err)
	}
	return result, created, nil
}

func (r *PgToolExecutionRepository) GetExecution(ctx context.Context, id uuid.UUID) (*ToolExecution, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, phase, workspace_id, tool_key, logical_action,
			idempotency_key, provider, is_mcp, mcp_server_id,
			content_provenance, pii_content, created_at
		FROM tool_executions
		WHERE id = $1`, id)

	var exec ToolExecution
	err := row.Scan(
		&exec.ID, &exec.Phase, &exec.WorkspaceID, &exec.ToolKey,
		&exec.LogicalAction, &exec.IdempotencyKey, &exec.Provider,
		&exec.IsMCP, &exec.MCPServerID, &exec.ContentProvenance,
		&exec.PIIContent, &exec.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("execution not found: %s", id)
		}
		return nil, fmt.Errorf("get execution: %w", err)
	}
	return &exec, nil
}

func (r *PgToolExecutionRepository) InsertReceipt(ctx context.Context, receipt *TrustReceipt) error {
	if receipt.ID == uuid.Nil {
		receipt.ID = uuid.Must(uuid.NewV7())
	}
	if receipt.CreatedAt.IsZero() {
		receipt.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO tool_execution_receipts (id, tool_execution_id, undo_instructions, created_at)
		VALUES ($1, $2, $3, $4)`,
		receipt.ID, receipt.ToolExecutionID, receipt.UndoInstructions, receipt.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert receipt: %w", err)
	}
	return nil
}

func (r *PgToolExecutionRepository) GetReceiptByExecution(ctx context.Context, executionID uuid.UUID) (*TrustReceipt, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, tool_execution_id, undo_instructions, created_at
		FROM tool_execution_receipts
		WHERE tool_execution_id = $1`, executionID)

	var receipt TrustReceipt
	err := row.Scan(&receipt.ID, &receipt.ToolExecutionID, &receipt.UndoInstructions, &receipt.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get receipt by execution: %w", err)
	}
	return &receipt, nil
}

func (r *PgToolExecutionRepository) IncrementSideEffect(ctx context.Context, workspaceID, toolKey string) (int, int, error) {
	row := r.db.QueryRow(ctx, `
		INSERT INTO tool_side_effects (workspace_id, tool_key, effect_count, updated_at)
		VALUES ($1, $2, 1, now())
		ON CONFLICT (workspace_id, tool_key) DO UPDATE
			SET effect_count = tool_side_effects.effect_count + 1,
				updated_at = now()
		RETURNING effect_count - 1, effect_count`,
		workspaceID, toolKey,
	)

	var before, after int
	if err := row.Scan(&before, &after); err != nil {
		return 0, 0, fmt.Errorf("increment side effect: %w", err)
	}
	return before, after, nil
}

func (r *PgToolExecutionRepository) GetSideEffectCount(ctx context.Context, workspaceID, toolKey string) (int, error) {
	row := r.db.QueryRow(ctx, `
		SELECT COALESCE(effect_count, 0)
		FROM tool_side_effects
		WHERE workspace_id = $1 AND tool_key = $2`,
		workspaceID, toolKey,
	)

	var count int
	err := row.Scan(&count)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, fmt.Errorf("get side effect count: %w", err)
	}
	return count, nil
}

func (r *PgToolExecutionRepository) InsertAuditEntry(ctx context.Context, entry *AuditLogEntry) error {
	if entry.ID == uuid.Nil {
		entry.ID = uuid.Must(uuid.NewV7())
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO executor_audit_log (id, event_type, payload, hash, prev_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.ID, entry.EventType, entry.Payload, entry.Hash, entry.PrevHash, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit entry: %w", err)
	}
	return nil
}
