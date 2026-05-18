package control

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
)

// ReceiptRepository persists authorization receipts and gate decisions.
type ReceiptRepository interface {
	// StoreReceipt persists a newly issued receipt.
	StoreReceipt(ctx context.Context, receipt *Receipt) error

	// GetReceipt retrieves a receipt by ID within the workspace scope.
	GetReceipt(ctx context.Context, receiptID string) (*Receipt, error)

	// ConsumeReceipt marks a receipt as consumed (one-time use). Returns
	// ErrReceiptConsumed if already consumed, ErrReceiptExpired if expired.
	ConsumeReceipt(ctx context.Context, receiptID string) error

	// RevokeReceipt marks a receipt as revoked with a reason.
	RevokeReceipt(ctx context.Context, receiptID, reason string) error

	// StoreGateDecision persists an execution gate decision with full evidence.
	StoreGateDecision(ctx context.Context, decision *GateDecisionRecord) error

	// StoreLedgerEntry persists an execution ledger entry.
	StoreLedgerEntry(ctx context.Context, entry *LedgerEntry) error

	// StoreBudgetEvent persists a budget enforcement event.
	StoreBudgetEvent(ctx context.Context, event *BudgetEvent) error
}

// GateDecisionRecord is the persisted form of a gate decision with full evidence.
type GateDecisionRecord struct {
	ID             string
	WorkspaceID    string
	IngressTurnID  string
	Decision       string
	ReasonCode     string
	InputJSON      map[string]any
	PolicyHash     string
	GateEvaluations []GateEvaluation
	CreatedAt      time.Time
}

// BudgetEvent records a budget enforcement check with evidence.
type BudgetEvent struct {
	ID          string
	WorkspaceID string
	ReceiptID   string
	Action      string // "check", "consume", "deny", "warn"
	UnitsUsed   int
	UnitsCap    int
	CostUSD     float64
	CapUSD      float64
	Evidence    map[string]any
	CreatedAt   time.Time
}

// PgReceiptRepository implements ReceiptRepository using pgx.
type PgReceiptRepository struct {
	db database.Querier
}

var _ ReceiptRepository = (*PgReceiptRepository)(nil)

// NewPgReceiptRepository creates a new pgx-backed receipt repository.
func NewPgReceiptRepository(db database.Querier) *PgReceiptRepository {
	return &PgReceiptRepository{db: db}
}

func (r *PgReceiptRepository) StoreReceipt(ctx context.Context, receipt *Receipt) error {
	gateResultsJSON, err := json.Marshal(receipt.GateResults)
	if err != nil {
		return fmt.Errorf("marshal gate_results: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO authorization_receipts (
			id, workspace_id, workflow_run_id, plan_id, decision,
			policy_bundle_hash, evaluated_gates, gate_results,
			tool_keys, risk_level, issued_by, issued_at, expires_at,
			consumed_at, revoked_at, revocation_reason, created_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8,
			$9, $10, $11, $12, $13,
			$14, $15, $16, $17
		)`,
		receipt.ID, receipt.WorkspaceID, receipt.WorkflowRunID, receipt.PlanID,
		receipt.Decision,
		receipt.PolicyBundleHash, receipt.EvaluatedGates, gateResultsJSON,
		receipt.ToolKeys, coalesceRiskLevel(receipt.RiskLevel),
		receipt.IssuedBy, receipt.IssuedAt, receipt.ExpiresAt,
		receipt.ConsumedAt, receipt.RevokedAt, nil, receipt.IssuedAt,
	)
	if err != nil {
		return fmt.Errorf("store receipt: %w", err)
	}
	return nil
}

func (r *PgReceiptRepository) GetReceipt(ctx context.Context, receiptID string) (*Receipt, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, workflow_run_id, plan_id, decision,
			policy_bundle_hash, evaluated_gates, gate_results,
			tool_keys, risk_level, issued_by, issued_at, expires_at,
			consumed_at, revoked_at
		FROM authorization_receipts
		WHERE id = $1`, receiptID)

	var receipt Receipt
	var gateResultsJSON []byte
	err := row.Scan(
		&receipt.ID, &receipt.WorkspaceID, &receipt.WorkflowRunID, &receipt.PlanID,
		&receipt.Decision,
		&receipt.PolicyBundleHash, &receipt.EvaluatedGates, &gateResultsJSON,
		&receipt.ToolKeys, &receipt.RiskLevel, &receipt.IssuedBy, &receipt.IssuedAt,
		&receipt.ExpiresAt, &receipt.ConsumedAt, &receipt.RevokedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrNoReceipt
		}
		return nil, fmt.Errorf("get receipt: %w", err)
	}

	if len(gateResultsJSON) > 0 {
		receipt.GateResults = make(map[string]string)
		if err := json.Unmarshal(gateResultsJSON, &receipt.GateResults); err != nil {
			return nil, fmt.Errorf("unmarshal gate_results: %w", err)
		}
	}
	return &receipt, nil
}

func (r *PgReceiptRepository) ConsumeReceipt(ctx context.Context, receiptID string) error {
	now := time.Now().UTC()

	tag, err := r.db.Exec(ctx, `
		UPDATE authorization_receipts
		SET consumed_at = $1
		WHERE id = $2
			AND consumed_at IS NULL
			AND revoked_at IS NULL
			AND expires_at > $1`,
		now, receiptID)
	if err != nil {
		return fmt.Errorf("consume receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Determine specific error.
		existing, getErr := r.GetReceipt(ctx, receiptID)
		if getErr != nil {
			return ErrNoReceipt
		}
		if existing.ConsumedAt != nil {
			return ErrReceiptConsumed
		}
		if existing.RevokedAt != nil {
			return ErrReceiptRevoked
		}
		if now.After(existing.ExpiresAt) {
			return ErrReceiptExpired
		}
		return ErrNoReceipt
	}
	return nil
}

func (r *PgReceiptRepository) RevokeReceipt(ctx context.Context, receiptID, reason string) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
		UPDATE authorization_receipts
		SET revoked_at = $1, revocation_reason = $2
		WHERE id = $3
			AND consumed_at IS NULL
			AND revoked_at IS NULL`,
		now, reason, receiptID)
	if err != nil {
		return fmt.Errorf("revoke receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoReceipt
	}
	return nil
}

func (r *PgReceiptRepository) StoreGateDecision(ctx context.Context, decision *GateDecisionRecord) error {
	if decision.ID == "" {
		decision.ID = database.GenerateUUIDv7()
	}
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = time.Now().UTC()
	}

	inputJSON, err := json.Marshal(decision.InputJSON)
	if err != nil {
		return fmt.Errorf("marshal input_json: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO execution_gate_decisions (
			id, workspace_id, ingress_turn_id, decision, reason_code, input_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		decision.ID, decision.WorkspaceID, nilIfEmpty(decision.IngressTurnID),
		decision.Decision, decision.ReasonCode, inputJSON, decision.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store gate decision: %w", err)
	}
	return nil
}

func (r *PgReceiptRepository) StoreLedgerEntry(ctx context.Context, entry *LedgerEntry) error {
	if entry.ID == "" {
		entry.ID = database.GenerateUUIDv7()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO execution_ledger (
			id, workspace_id, receipt_id, tool_key, phase,
			idempotency_key, payload_hash, result_status, duration_ms,
			error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		entry.ID, entry.WorkspaceID, entry.ReceiptID, entry.ToolKey, entry.Phase,
		entry.IdempotencyKey, entry.PayloadHash, entry.ResultStatus, entry.DurationMS,
		nilIfEmpty(entry.ErrorMessage), entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store ledger entry: %w", err)
	}
	return nil
}

func (r *PgReceiptRepository) StoreBudgetEvent(ctx context.Context, event *BudgetEvent) error {
	if event.ID == "" {
		event.ID = database.GenerateUUIDv7()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	evidenceJSON, err := json.Marshal(event.Evidence)
	if err != nil {
		return fmt.Errorf("marshal budget evidence: %w", err)
	}

	// Budget events are stored as gate decisions with budget-specific evidence
	// in input_json, allowing reconstruction of why a tool was allowed/denied.
	inputJSON := map[string]any{
		"budget_action":   event.Action,
		"units_used":      event.UnitsUsed,
		"units_cap":       event.UnitsCap,
		"cost_usd":        event.CostUSD,
		"cap_usd":         event.CapUSD,
		"receipt_id":      event.ReceiptID,
		"budget_evidence": json.RawMessage(evidenceJSON),
	}
	inputBytes, err := json.Marshal(inputJSON)
	if err != nil {
		return fmt.Errorf("marshal budget input: %w", err)
	}

	decision := "allow"
	reasonCode := "BUDGET_CHECK_PASSED"
	if event.Action == "deny" {
		decision = "deny"
		reasonCode = "BUDGET_EXHAUSTED"
	} else if event.Action == "warn" {
		reasonCode = "BUDGET_WARNING_THRESHOLD"
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO execution_gate_decisions (
			id, workspace_id, decision, reason_code, input_json, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)`,
		event.ID, event.WorkspaceID, decision, reasonCode, inputBytes, event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("store budget event: %w", err)
	}
	return nil
}

func coalesceRiskLevel(level string) string {
	if level == "" {
		return "LOW"
	}
	return level
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
