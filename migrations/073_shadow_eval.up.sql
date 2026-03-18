-- Shadow evaluation results (P3-12).
CREATE TABLE shadow_eval_results (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id            UUID NOT NULL,
    workspace_id          UUID NOT NULL,
    champion_orm_score    FLOAT NOT NULL,
    challenger_orm_score  FLOAT NOT NULL,
    champion_llm_score    FLOAT NOT NULL,
    challenger_llm_score  FLOAT NOT NULL,
    challenger_model      TEXT NOT NULL,
    evaluated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_shadow_eval_workspace_id ON shadow_eval_results(workspace_id);
CREATE INDEX idx_shadow_eval_evaluated_at ON shadow_eval_results(evaluated_at);

-- Model promotion requests.
CREATE TABLE model_promotion_requests (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenger_model TEXT NOT NULL,
    metrics          JSONB NOT NULL,
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    requested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_by      UUID,
    reviewed_at      TIMESTAMPTZ
);
