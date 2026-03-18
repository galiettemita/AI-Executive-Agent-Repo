-- Red-team adversarial attack attempt log.
CREATE TABLE red_team_attempts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    attack_type   TEXT NOT NULL,
    payload_hash  TEXT NOT NULL,
    blocked       BOOLEAN NOT NULL,
    block_layer   TEXT,
    latency_ms    INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_red_team_attempts_attack_type ON red_team_attempts(attack_type);
CREATE INDEX idx_red_team_attempts_created_at ON red_team_attempts(created_at);

-- Security evaluation scores per category per run.
CREATE TABLE pg_security_scores (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id      UUID NOT NULL,
    category    TEXT NOT NULL,
    pass_rate   FLOAT NOT NULL,
    run_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_pg_security_scores_run_id ON pg_security_scores(run_id);
CREATE INDEX idx_pg_security_scores_run_at ON pg_security_scores(run_at);
