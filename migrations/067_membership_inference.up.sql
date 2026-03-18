-- Membership inference audit results.
CREATE TABLE membership_inference_results (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL,
    auc          FLOAT NOT NULL,
    alert_fired  BOOLEAN NOT NULL DEFAULT FALSE,
    run_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_mi_results_workspace_id ON membership_inference_results(workspace_id);
