BEGIN;

CREATE SCHEMA IF NOT EXISTS temporal;

CREATE TABLE IF NOT EXISTS temporal.workflow_snapshots (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  workflow_id VARCHAR(120) NOT NULL,
  run_id VARCHAR(120) NOT NULL,
  queue VARCHAR(120) NOT NULL,
  state VARCHAR(30) NOT NULL,
  snapshot JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workflow_id, run_id)
);

CREATE TABLE IF NOT EXISTS temporal.compensation_logs (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  workflow_id VARCHAR(120) NOT NULL,
  run_id VARCHAR(120) NOT NULL,
  activity_name VARCHAR(120) NOT NULL,
  status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'DONE', 'FAILED')),
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMIT;
