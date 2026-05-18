BEGIN;

CREATE TABLE IF NOT EXISTS public.cron_executions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  job_id UUID NOT NULL REFERENCES public.cron_jobs(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','success','failed','timed_out','skipped','cancelled')),
  scheduled_at TIMESTAMPTZ NOT NULL,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  input_json JSONB DEFAULT '{}',
  output_json JSONB DEFAULT '{}',
  error_code VARCHAR(64),
  error_message TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  duration_ms INTEGER,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_executions_job ON public.cron_executions(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_executions_user ON public.cron_executions(user_id);
CREATE INDEX IF NOT EXISTS idx_cron_executions_status ON public.cron_executions(status);
CREATE INDEX IF NOT EXISTS idx_cron_executions_scheduled ON public.cron_executions(scheduled_at DESC);

ALTER TABLE public.cron_executions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cron_executions_user_or_service ON public.cron_executions;
CREATE POLICY cron_executions_user_or_service ON public.cron_executions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
