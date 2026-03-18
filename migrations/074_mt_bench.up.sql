-- MT-Bench evaluation scores (P3-12).
CREATE TABLE mt_bench_runs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    overall_score FLOAT NOT NULL,
    model         TEXT NOT NULL,
    triggered_by  TEXT NOT NULL DEFAULT 'cron'
);

CREATE INDEX idx_mt_bench_runs_run_at ON mt_bench_runs(run_at DESC);

CREATE TABLE mt_bench_scores (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id             UUID NOT NULL REFERENCES mt_bench_runs(id),
    category           TEXT NOT NULL,
    avg_score          FLOAT NOT NULL,
    conversation_count INTEGER NOT NULL,
    run_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mt_bench_scores_run_id ON mt_bench_scores(run_id);
