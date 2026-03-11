package contextlayer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brevio/brevio/internal/database"
)

// PgRepository implements Repository backed by PostgreSQL.
type PgRepository struct {
	db database.Querier
}

// NewPgRepository creates a production context budget repository.
func NewPgRepository(db database.Querier) *PgRepository {
	return &PgRepository{db: db}
}

func (r *PgRepository) UpsertBudget(ctx context.Context, budget Budget) (Budget, error) {
	wsID := normalizeWorkspaceID(budget.WorkspaceID)
	if budget.MaxContextTokens <= 0 {
		budget.MaxContextTokens = 2048
	}
	if budget.Status == "" {
		budget.Status = "active"
	}
	budget.BudgetTokens = budget.MaxContextTokens

	_, err := r.db.Exec(ctx, `
		INSERT INTO context_budgets (workspace_id, budget_tokens, status)
		VALUES ($1::uuid, $2, $3::context_budget_status)
		ON CONFLICT (workspace_id) DO UPDATE SET
			budget_tokens = EXCLUDED.budget_tokens,
			status = EXCLUDED.status,
			updated_at = now()`,
		wsID, budget.MaxContextTokens, budget.Status)
	if err != nil {
		return Budget{}, fmt.Errorf("upsert budget: %w", err)
	}
	budget.WorkspaceID = wsID
	return budget, nil
}

func (r *PgRepository) GetBudget(ctx context.Context, workspaceID string) (Budget, bool, error) {
	wsID := normalizeWorkspaceID(workspaceID)
	row := r.db.QueryRow(ctx, `
		SELECT workspace_id, budget_tokens, status
		FROM context_budgets
		WHERE workspace_id = $1::uuid`, wsID)

	var b Budget
	err := row.Scan(&b.WorkspaceID, &b.MaxContextTokens, &b.Status)
	if err != nil {
		return Budget{}, false, nil // Not found — not an error.
	}
	b.BudgetTokens = b.MaxContextTokens
	b.Tier = "T2"
	b.ReservedResponseTokens = 256
	b.MaxRAGTokens = 512
	return b, true, nil
}

func (r *PgRepository) SetAllocations(ctx context.Context, workspaceID string, allocations []Allocation) error {
	wsID := normalizeWorkspaceID(workspaceID)

	// Get budget ID for FK.
	var budgetID string
	err := r.db.QueryRow(ctx, `SELECT id FROM context_budgets WHERE workspace_id = $1::uuid`, wsID).Scan(&budgetID)
	if err != nil {
		return fmt.Errorf("budget not found for workspace %s: %w", wsID, err)
	}

	// Delete existing allocations and insert new ones (idempotent replace).
	_, err = r.db.Exec(ctx, `DELETE FROM context_budget_allocations WHERE context_budget_id = $1::uuid`, budgetID)
	if err != nil {
		return fmt.Errorf("clear allocations: %w", err)
	}

	for _, alloc := range allocations {
		_, err = r.db.Exec(ctx, `
			INSERT INTO context_budget_allocations (workspace_id, context_budget_id, item_type, allocated_tokens)
			VALUES ($1::uuid, $2::uuid, $3::context_item_type, $4)`,
			wsID, budgetID, alloc.ItemType, alloc.AllocatedTokens)
		if err != nil {
			return fmt.Errorf("insert allocation %s: %w", alloc.ItemType, err)
		}
	}
	return nil
}

func (r *PgRepository) GetAllocations(ctx context.Context, workspaceID string) ([]Allocation, error) {
	wsID := normalizeWorkspaceID(workspaceID)
	rows, err := r.db.Query(ctx, `
		SELECT cba.item_type, cba.allocated_tokens
		FROM context_budget_allocations cba
		JOIN context_budgets cb ON cb.id = cba.context_budget_id
		WHERE cb.workspace_id = $1::uuid
		ORDER BY cba.item_type`, wsID)
	if err != nil {
		return nil, fmt.Errorf("get allocations: %w", err)
	}
	defer rows.Close()

	var out []Allocation
	for rows.Next() {
		var a Allocation
		if err := rows.Scan(&a.ItemType, &a.AllocatedTokens); err != nil {
			return nil, fmt.Errorf("scan allocation: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *PgRepository) RecordAuditEvent(ctx context.Context, workspaceID string, event map[string]any) error {
	wsID := normalizeWorkspaceID(workspaceID)

	var budgetID string
	err := r.db.QueryRow(ctx, `SELECT id FROM context_budgets WHERE workspace_id = $1::uuid`, wsID).Scan(&budgetID)
	if err != nil {
		return fmt.Errorf("budget not found for audit: %w", err)
	}

	eventJSON, _ := json.Marshal(event)
	_, err = r.db.Exec(ctx, `
		INSERT INTO context_budget_audit (workspace_id, context_budget_id, event_json)
		VALUES ($1::uuid, $2::uuid, $3)`,
		wsID, budgetID, eventJSON)
	if err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}
