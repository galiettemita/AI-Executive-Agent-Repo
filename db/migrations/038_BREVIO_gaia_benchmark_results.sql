BEGIN;

CREATE TABLE IF NOT EXISTS gaia_benchmark_runs (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_number       INT NOT NULL,
    triggered_by     TEXT NOT NULL DEFAULT 'cron',
    model_version    TEXT NOT NULL,
    total_tasks      INT NOT NULL,
    passed           INT NOT NULL DEFAULT 0,
    failed           INT NOT NULL DEFAULT 0,
    skipped          INT NOT NULL DEFAULT 0,
    pass_rate        FLOAT8,
    easy_pass_rate   FLOAT8,
    medium_pass_rate FLOAT8,
    hard_pass_rate   FLOAT8,
    duration_seconds FLOAT8,
    prior_pass_rate  FLOAT8,
    status           TEXT NOT NULL DEFAULT 'running'
                         CHECK (status IN ('running','completed','failed','partial')),
    regression_alert BOOLEAN NOT NULL DEFAULT FALSE,
    error_message    TEXT,
    started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_gaia_runs_number ON gaia_benchmark_runs(run_number DESC);

CREATE TABLE IF NOT EXISTS gaia_task_results (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id         UUID NOT NULL REFERENCES gaia_benchmark_runs(id) ON DELETE CASCADE,
    task_id        TEXT NOT NULL,
    tier           TEXT NOT NULL,
    category       TEXT NOT NULL,
    intent         TEXT NOT NULL,
    passed         BOOL NOT NULL,
    pass_detail    TEXT,
    tools_called   TEXT[] NOT NULL DEFAULT '{}',
    expected_tools TEXT[] NOT NULL DEFAULT '{}',
    latency_ms     INT,
    error_message  TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gaia_task_results_run ON gaia_task_results(run_id);

COMMIT;
