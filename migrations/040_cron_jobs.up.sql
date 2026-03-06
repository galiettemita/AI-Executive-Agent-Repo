BEGIN;

CREATE TABLE IF NOT EXISTS public.cron_jobs (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  job_name VARCHAR(256) NOT NULL,
  description TEXT,
  cron_expression VARCHAR(128) NOT NULL,
  timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
  skill_id VARCHAR(64),
  action_type VARCHAR(32) NOT NULL CHECK (action_type IN ('skill','webhook','message','agent','workflow')),
  action_config JSONB NOT NULL DEFAULT '{}',
  status VARCHAR(16) NOT NULL DEFAULT 'active'
    CHECK (status IN ('active','paused','disabled','expired')),
  max_retries INTEGER NOT NULL DEFAULT 3,
  retry_delay_ms INTEGER NOT NULL DEFAULT 5000,
  timeout_ms INTEGER NOT NULL DEFAULT 120000,
  next_run_at TIMESTAMPTZ,
  last_run_at TIMESTAMPTZ,
  last_run_status VARCHAR(16) CHECK (last_run_status IN ('success','failed','timed_out','skipped')),
  run_count INTEGER NOT NULL DEFAULT 0,
  failure_count INTEGER NOT NULL DEFAULT 0,
  starts_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_jobs_user ON public.cron_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_cron_jobs_status ON public.cron_jobs(status);
CREATE INDEX IF NOT EXISTS idx_cron_jobs_next_run ON public.cron_jobs(next_run_at) WHERE status = 'active';
CREATE UNIQUE INDEX IF NOT EXISTS idx_cron_jobs_user_name ON public.cron_jobs(user_id, job_name);

ALTER TABLE public.cron_jobs ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cron_jobs_user_or_service ON public.cron_jobs;
CREATE POLICY cron_jobs_user_or_service ON public.cron_jobs
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
