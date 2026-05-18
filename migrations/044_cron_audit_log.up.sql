BEGIN;

CREATE TABLE IF NOT EXISTS public.cron_audit_log (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  job_id UUID REFERENCES public.cron_jobs(id) ON DELETE SET NULL,
  action VARCHAR(32) NOT NULL CHECK (action IN ('created','updated','paused','resumed','deleted','executed','failed','expired')),
  actor VARCHAR(32) NOT NULL CHECK (actor IN ('user','system','scheduler','admin')),
  details_json JSONB DEFAULT '{}',
  ip_address VARCHAR(45),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_audit_user ON public.cron_audit_log(user_id);
CREATE INDEX IF NOT EXISTS idx_cron_audit_job ON public.cron_audit_log(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_audit_action ON public.cron_audit_log(action);
CREATE INDEX IF NOT EXISTS idx_cron_audit_created ON public.cron_audit_log(created_at DESC);

ALTER TABLE public.cron_audit_log ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cron_audit_user_or_service ON public.cron_audit_log;
CREATE POLICY cron_audit_user_or_service ON public.cron_audit_log
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
