package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository persists benchmark runs and task results to Postgres.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// NextRunNumber returns the next sequential run number.
func (r *Repository) NextRunNumber(ctx context.Context) (int, error) {
	if r.pool == nil {
		return 1, nil
	}
	var n int
	return n, r.pool.QueryRow(ctx, `SELECT COALESCE(MAX(run_number),0)+1 FROM gaia_benchmark_runs`).Scan(&n)
}

// LatestPassRate returns the pass_rate of the most recent completed run.
func (r *Repository) LatestPassRate(ctx context.Context) (float64, error) {
	if r.pool == nil {
		return 0, nil
	}
	var rate float64
	err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(pass_rate, 0) FROM gaia_benchmark_runs
		WHERE status='completed' ORDER BY run_number DESC LIMIT 1`).Scan(&rate)
	if err != nil {
		return 0, nil
	}
	return rate, nil
}

// InsertRun creates a new benchmark run record.
func (r *Repository) InsertRun(ctx context.Context, run BenchmarkRun) (BenchmarkRun, error) {
	run.ID = uuid.New()
	run.StartedAt = time.Now().UTC()
	run.Status = "running"
	if r.pool == nil {
		return run, nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO gaia_benchmark_runs
			(id, run_number, triggered_by, model_version, total_tasks, prior_pass_rate, status, started_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		run.ID, run.RunNumber, run.TriggeredBy, run.ModelVersion, run.TotalTasks,
		run.PriorPassRate, run.Status, run.StartedAt)
	return run, err
}

// UpdateRunComplete finalizes a run with aggregate stats.
func (r *Repository) UpdateRunComplete(ctx context.Context, id uuid.UUID, passed, failed, skipped int,
	easyRate, medRate, hardRate *float64, durationSec float64, regressionAlert bool) error {
	if r.pool == nil {
		return nil
	}
	now := time.Now().UTC()
	passRate := 0.0
	total := passed + failed + skipped
	if total > 0 {
		passRate = float64(passed) / float64(total)
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE gaia_benchmark_runs
		SET passed=$2, failed=$3, skipped=$4, pass_rate=$5,
			easy_pass_rate=$6, medium_pass_rate=$7, hard_pass_rate=$8,
			duration_seconds=$9, regression_alert=$10,
			status='completed', completed_at=$11
		WHERE id=$1`,
		id, passed, failed, skipped, passRate, easyRate, medRate, hardRate,
		durationSec, regressionAlert, now)
	return err
}

// UpdateRunFailed marks a run as failed.
func (r *Repository) UpdateRunFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	if r.pool == nil {
		return nil
	}
	now := time.Now().UTC()
	_, err := r.pool.Exec(ctx, `
		UPDATE gaia_benchmark_runs SET status='failed', error_message=$2, completed_at=$3 WHERE id=$1`,
		id, errMsg, now)
	return err
}

// InsertTaskResult stores a single task result.
func (r *Repository) InsertTaskResult(ctx context.Context, res TaskResult) error {
	res.ID = uuid.New()
	res.CreatedAt = time.Now().UTC()
	if r.pool == nil {
		return nil
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO gaia_task_results
			(id, run_id, task_id, tier, category, intent, passed, pass_detail,
			 tools_called, expected_tools, latency_ms, error_message, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		res.ID, res.RunID, res.TaskID, res.Tier, res.Category, res.Intent,
		res.Passed, res.PassDetail,
		res.ToolsCalled, res.ExpectedTools,
		res.LatencyMs, res.ErrorMessage, res.CreatedAt)
	if err != nil {
		return fmt.Errorf("benchmark.Repository.InsertTaskResult %s: %w", res.TaskID, err)
	}
	return nil
}

// LatestRun returns the most recently completed benchmark run.
func (r *Repository) LatestRun(ctx context.Context) (*BenchmarkRun, error) {
	if r.pool == nil {
		return nil, fmt.Errorf("benchmark.Repository.LatestRun: no pool")
	}
	var run BenchmarkRun
	err := r.pool.QueryRow(ctx, `
		SELECT id, run_number, triggered_by, model_version, total_tasks, passed, failed, skipped,
		       pass_rate, easy_pass_rate, medium_pass_rate, hard_pass_rate, prior_pass_rate,
		       duration_seconds, regression_alert, status, error_message, started_at, completed_at
		FROM gaia_benchmark_runs WHERE status='completed'
		ORDER BY run_number DESC LIMIT 1`,
	).Scan(
		&run.ID, &run.RunNumber, &run.TriggeredBy, &run.ModelVersion, &run.TotalTasks,
		&run.Passed, &run.Failed, &run.Skipped,
		&run.PassRate, &run.EasyPassRate, &run.MediumPassRate, &run.HardPassRate,
		&run.PriorPassRate, &run.DurationSeconds, &run.RegressionAlert,
		&run.Status, &run.ErrorMessage, &run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("benchmark.Repository.LatestRun: %w", err)
	}
	return &run, nil
}

// RunHistory returns the last N completed runs.
func (r *Repository) RunHistory(ctx context.Context, limit int) ([]BenchmarkRun, error) {
	if r.pool == nil {
		return nil, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, run_number, triggered_by, model_version, total_tasks, passed, failed, skipped,
		       pass_rate, prior_pass_rate, duration_seconds, regression_alert, status, started_at, completed_at
		FROM gaia_benchmark_runs WHERE status IN ('completed','partial')
		ORDER BY run_number DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []BenchmarkRun
	for rows.Next() {
		var run BenchmarkRun
		if err := rows.Scan(
			&run.ID, &run.RunNumber, &run.TriggeredBy, &run.ModelVersion, &run.TotalTasks,
			&run.Passed, &run.Failed, &run.Skipped,
			&run.PassRate, &run.PriorPassRate, &run.DurationSeconds,
			&run.RegressionAlert, &run.Status, &run.StartedAt, &run.CompletedAt,
		); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
