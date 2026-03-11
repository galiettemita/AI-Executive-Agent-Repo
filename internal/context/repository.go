package contextlayer

import "context"

// Repository abstracts durable storage for context budgets.
// Production uses PgRepository; tests use the in-memory Service.
type Repository interface {
	// UpsertBudget creates or updates a context budget for a workspace.
	UpsertBudget(ctx context.Context, budget Budget) (Budget, error)
	// GetBudget retrieves the budget for a workspace.
	GetBudget(ctx context.Context, workspaceID string) (Budget, bool, error)
	// SetAllocations stores allocations for a workspace.
	SetAllocations(ctx context.Context, workspaceID string, allocations []Allocation) error
	// GetAllocations retrieves allocations for a workspace.
	GetAllocations(ctx context.Context, workspaceID string) ([]Allocation, error)
	// RecordAuditEvent logs a budget audit event.
	RecordAuditEvent(ctx context.Context, workspaceID string, event map[string]any) error
}
