-- Audit log of ShouldMergeDuplicate() decisions for consolidation precision measurement.

CREATE TABLE IF NOT EXISTS memory_merge_decision_log (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    TEXT NOT NULL,
    new_item_id     TEXT NOT NULL,
    candidate_id    TEXT NOT NULL,
    cosine_score    NUMERIC(5,4) NOT NULL,
    merged          BOOLEAN NOT NULL,
    new_body        TEXT NOT NULL,
    candidate_body  TEXT NOT NULL,
    decided_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_merge_log_workspace_time
    ON memory_merge_decision_log (workspace_id, decided_at DESC)
    WHERE merged = TRUE;
