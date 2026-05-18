package executor

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/database"
)

// LatencyBudgetLogRow represents a persisted latency preemption decision.
type LatencyBudgetLogRow struct {
	ID                string  `json:"id"`
	WorkspaceID       string  `json:"workspace_id"`
	WorkflowRunID     string  `json:"workflow_run_id"`
	BudgetMs          float64 `json:"budget_ms"`
	ElapsedMs         float64 `json:"elapsed_ms"`
	EstimatedNextMs   float64 `json:"estimated_next_ms"`
	ShouldProceed     bool    `json:"should_proceed"`
	Reason            string  `json:"reason"`
	RemainingBudgetMs float64 `json:"remaining_budget_ms"`
}

// LatencyRepository persists latency budget decisions.
type LatencyRepository interface {
	RecordDecision(ctx context.Context, row LatencyBudgetLogRow) error
	GetDecisions(ctx context.Context, workspaceID string, limit int) ([]LatencyBudgetLogRow, error)
	EvaluateAndPersist(ctx context.Context, workspaceID, workflowRunID string, budgetMs, elapsedMs, estimatedNextMs float64) (PreemptionDecision, error)
}

// PgLatencyRepository implements LatencyRepository backed by pgx.
type PgLatencyRepository struct {
	q         database.Querier
	preemptor *LatencyPreemptor
}

// NewPgLatencyRepository creates a new PgLatencyRepository.
func NewPgLatencyRepository(q database.Querier) *PgLatencyRepository {
	return &PgLatencyRepository{
		q:         q,
		preemptor: NewLatencyPreemptor(),
	}
}

// RecordDecision persists a latency budget decision.
func (r *PgLatencyRepository) RecordDecision(ctx context.Context, row LatencyBudgetLogRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO latency_budget_log (workspace_id, workflow_run_id, budget_ms, elapsed_ms, estimated_next_ms, should_proceed, reason, remaining_budget_ms)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)`,
		row.WorkspaceID, row.WorkflowRunID, row.BudgetMs, row.ElapsedMs, row.EstimatedNextMs, row.ShouldProceed, row.Reason, row.RemainingBudgetMs,
	)
	if err != nil {
		return fmt.Errorf("record latency decision: %w", err)
	}
	return nil
}

// GetDecisions returns recent latency budget decisions for a workspace.
func (r *PgLatencyRepository) GetDecisions(ctx context.Context, workspaceID string, limit int) ([]LatencyBudgetLogRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, workflow_run_id, budget_ms, elapsed_ms, estimated_next_ms, should_proceed, reason, remaining_budget_ms
		 FROM latency_budget_log
		 WHERE workspace_id = $1::uuid
		 ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get latency decisions: %w", err)
	}
	defer rows.Close()

	var result []LatencyBudgetLogRow
	for rows.Next() {
		var d LatencyBudgetLogRow
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.WorkflowRunID, &d.BudgetMs, &d.ElapsedMs,
			&d.EstimatedNextMs, &d.ShouldProceed, &d.Reason, &d.RemainingBudgetMs); err != nil {
			return nil, fmt.Errorf("scan latency decision: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// EvaluateAndPersist runs preemption evaluation and persists the decision.
func (r *PgLatencyRepository) EvaluateAndPersist(ctx context.Context, workspaceID, workflowRunID string, budgetMs, elapsedMs, estimatedNextMs float64) (PreemptionDecision, error) {
	decision := r.preemptor.ShouldProceed(budgetMs, elapsedMs, estimatedNextMs)

	row := LatencyBudgetLogRow{
		WorkspaceID:       workspaceID,
		WorkflowRunID:     workflowRunID,
		BudgetMs:          budgetMs,
		ElapsedMs:         elapsedMs,
		EstimatedNextMs:   estimatedNextMs,
		ShouldProceed:     decision.ShouldProceed,
		Reason:            decision.Reason,
		RemainingBudgetMs: decision.RemainingBudgetMs,
	}
	if err := r.RecordDecision(ctx, row); err != nil {
		return decision, err
	}
	return decision, nil
}
