BEGIN;

CREATE TABLE IF NOT EXISTS a2a_tasks (
    id              TEXT PRIMARY KEY,
    workspace_id    UUID NOT NULL,
    requesting_agent_id TEXT NOT NULL,
    capability      TEXT NOT NULL,
    input_payload   JSONB NOT NULL,
    status          TEXT NOT NULL DEFAULT 'submitted',
    output_payload  JSONB,
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_a2a_tasks_workspace
    ON a2a_tasks (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_a2a_tasks_status
    ON a2a_tasks (status, created_at DESC);

CREATE TABLE IF NOT EXISTS a2a_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id         TEXT NOT NULL REFERENCES a2a_tasks(id),
    direction       TEXT NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_a2a_messages_task
    ON a2a_messages (task_id, created_at);

COMMIT;
