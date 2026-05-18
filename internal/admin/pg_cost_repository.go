package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// CostReadRepository provides read-only access to pre-aggregated cost data (NNR-106).
// All methods query rollup tables, never raw ledgers.
type CostReadRepository interface {
	GetCostSummary(ctx context.Context, workspaceID string) (CostSummaryResult, error)
	GetDailyRollups(ctx context.Context, workspaceID string, from, to time.Time, limit int) ([]UserCostDailyRollup, error)
	GetTaskRollups(ctx context.Context, workspaceID string, limit int) ([]TaskCostRollup, error)
	GetCostProjections(ctx context.Context, workspaceID string) (CostProjection, error)
}

// CostLedgerWriter writes raw cost events to ledger tables (NNR-105: only via Temporal).
type CostLedgerWriter interface {
	InsertLLMCost(ctx context.Context, evt LLMCostEvent) error
	InsertConnectorCost(ctx context.Context, evt ConnectorCostEvent) error
	UpsertTaskRollup(ctx context.Context, workspaceID, workflowRunID, userID string, llmCost, connCost float64, llmCalls, connCalls, durationMs int) error
	UpsertDailyRollup(ctx context.Context, workspaceID, userID string, date time.Time, llmCost, connCost float64, taskCount, llmCalls, connCalls int) error
}

// PgCostRepository implements both CostReadRepository and CostLedgerWriter backed by pgx.
type PgCostRepository struct {
	q database.Querier
}

// NewPgCostRepository creates a new PgCostRepository.
func NewPgCostRepository(q database.Querier) *PgCostRepository {
	return &PgCostRepository{q: q}
}

// GetCostSummary reads from user_cost_daily_rollup to compute workspace summary (NNR-106).
func (r *PgCostRepository) GetCostSummary(ctx context.Context, workspaceID string) (CostSummaryResult, error) {
	var result CostSummaryResult
	result.WorkspaceID = workspaceID

	err := r.q.QueryRow(ctx,
		`SELECT COALESCE(SUM(llm_cost_usd), 0), COALESCE(SUM(connector_cost_usd), 0),
		        COALESCE(SUM(total_cost_usd), 0), COALESCE(SUM(task_count), 0)
		 FROM user_cost_daily_rollup
		 WHERE workspace_id = $1::uuid`,
		workspaceID,
	).Scan(&result.TotalLLMCostUSD, &result.TotalConnCostUSD, &result.TotalCostUSD, &result.RecordCount)
	if err != nil {
		return result, fmt.Errorf("get cost summary: %w", err)
	}
	return result, nil
}

// GetDailyRollups reads pre-aggregated daily rollups (NNR-106).
func (r *PgCostRepository) GetDailyRollups(ctx context.Context, workspaceID string, from, to time.Time, limit int) ([]UserCostDailyRollup, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, user_id, rollup_date, llm_cost_usd, connector_cost_usd, total_cost_usd
		 FROM user_cost_daily_rollup
		 WHERE workspace_id = $1::uuid AND rollup_date >= $2 AND rollup_date < $3
		 ORDER BY rollup_date DESC
		 LIMIT $4`,
		workspaceID, from, to, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get daily rollups: %w", err)
	}
	defer rows.Close()

	var rollups []UserCostDailyRollup
	for rows.Next() {
		var r UserCostDailyRollup
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &r.UserID, &r.Date, &r.LLMCostUSD, &r.ConnectorCostUSD, &r.TotalCostUSD); err != nil {
			return nil, fmt.Errorf("scan daily rollup: %w", err)
		}
		rollups = append(rollups, r)
	}
	return rollups, rows.Err()
}

// GetTaskRollups reads pre-aggregated task rollups (NNR-106).
func (r *PgCostRepository) GetTaskRollups(ctx context.Context, workspaceID string, limit int) ([]TaskCostRollup, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, workflow_run_id, llm_cost_usd, connector_cost_usd, total_cost_usd, created_at
		 FROM task_cost_rollup
		 WHERE workspace_id = $1::uuid
		 ORDER BY created_at DESC
		 LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get task rollups: %w", err)
	}
	defer rows.Close()

	var rollups []TaskCostRollup
	for rows.Next() {
		var r TaskCostRollup
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &r.WorkflowExecutionID, &r.LLMCostUSD, &r.ConnectorCostUSD, &r.TotalCostUSD, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task rollup: %w", err)
		}
		rollups = append(rollups, r)
	}
	return rollups, rows.Err()
}

// GetCostProjections computes projections from daily rollups (NNR-106).
func (r *PgCostRepository) GetCostProjections(ctx context.Context, workspaceID string) (CostProjection, error) {
	var projection CostProjection
	projection.WorkspaceID = workspaceID

	err := r.q.QueryRow(ctx,
		`SELECT COALESCE(AVG(total_cost_usd), 0), COUNT(DISTINCT rollup_date)
		 FROM user_cost_daily_rollup
		 WHERE workspace_id = $1::uuid
		   AND rollup_date >= CURRENT_DATE - INTERVAL '30 days'`,
		workspaceID,
	).Scan(&projection.DailyAvgCostUSD, &projection.DaysObserved)
	if err != nil {
		return projection, fmt.Errorf("get cost projections: %w", err)
	}
	projection.ProjectedMonthUSD = projection.DailyAvgCostUSD * 30
	return projection, nil
}

// InsertLLMCost writes a raw LLM cost event to the ledger table.
func (r *PgCostRepository) InsertLLMCost(ctx context.Context, evt LLMCostEvent) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO llm_cost_ledger (workspace_id, user_id, workflow_run_id, provider, model, tokens_input, tokens_output, cost_usd, latency_ms, cache_hit)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10)`,
		evt.WorkspaceID, evt.UserID, evt.WorkflowRunID, evt.Provider, evt.Model,
		evt.TokensInput, evt.TokensOutput, evt.CostUSD, evt.LatencyMs, evt.CacheHit,
	)
	return err
}

// InsertConnectorCost writes a raw connector cost event to the ledger table.
func (r *PgCostRepository) InsertConnectorCost(ctx context.Context, evt ConnectorCostEvent) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO connector_cost_ledger (workspace_id, user_id, workflow_run_id, connector_id, connector_name, operation, cost_usd, latency_ms)
		 VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6, $7, $8)`,
		evt.WorkspaceID, evt.UserID, evt.WorkflowRunID, evt.ConnectorID, evt.ConnectorName,
		evt.Operation, evt.CostUSD, evt.LatencyMs,
	)
	return err
}

// UpsertTaskRollup upserts a task cost rollup (NNR-105: only called by Temporal).
func (r *PgCostRepository) UpsertTaskRollup(ctx context.Context, workspaceID, workflowRunID, userID string, llmCost, connCost float64, llmCalls, connCalls, durationMs int) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO task_cost_rollup (workspace_id, workflow_run_id, user_id, llm_cost_usd, connector_cost_usd, total_cost_usd, llm_calls, connector_calls, duration_ms)
		 VALUES ($1::uuid, $2, $3::uuid, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (workflow_run_id) DO UPDATE SET
		   llm_cost_usd = EXCLUDED.llm_cost_usd,
		   connector_cost_usd = EXCLUDED.connector_cost_usd,
		   total_cost_usd = EXCLUDED.total_cost_usd,
		   llm_calls = EXCLUDED.llm_calls,
		   connector_calls = EXCLUDED.connector_calls,
		   duration_ms = EXCLUDED.duration_ms,
		   updated_at = now()`,
		workspaceID, workflowRunID, userID, llmCost, connCost, llmCost+connCost, llmCalls, connCalls, durationMs,
	)
	return err
}

// UpsertDailyRollup upserts a daily user cost rollup (NNR-105: only called by Temporal).
func (r *PgCostRepository) UpsertDailyRollup(ctx context.Context, workspaceID, userID string, date time.Time, llmCost, connCost float64, taskCount, llmCalls, connCalls int) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO user_cost_daily_rollup (workspace_id, user_id, rollup_date, llm_cost_usd, connector_cost_usd, total_cost_usd, task_count, llm_calls, connector_calls)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (workspace_id, user_id, rollup_date) DO UPDATE SET
		   llm_cost_usd = EXCLUDED.llm_cost_usd,
		   connector_cost_usd = EXCLUDED.connector_cost_usd,
		   total_cost_usd = EXCLUDED.total_cost_usd,
		   task_count = EXCLUDED.task_count,
		   llm_calls = EXCLUDED.llm_calls,
		   connector_calls = EXCLUDED.connector_calls`,
		workspaceID, userID, date, llmCost, connCost, llmCost+connCost, taskCount, llmCalls, connCalls,
	)
	return err
}
