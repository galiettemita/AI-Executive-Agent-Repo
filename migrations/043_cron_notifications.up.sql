BEGIN;

CREATE TABLE IF NOT EXISTS public.cron_notifications (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  job_id UUID NOT NULL REFERENCES public.cron_jobs(id) ON DELETE CASCADE,
  notify_on TEXT[] NOT NULL DEFAULT '{failed}',
  channel VARCHAR(16) NOT NULL CHECK (channel IN ('email','sms','push','webhook','in_app')),
  destination VARCHAR(256) NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_cron_notifications_user ON public.cron_notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_cron_notifications_job ON public.cron_notifications(job_id);

ALTER TABLE public.cron_notifications ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS cron_notifications_user_or_service ON public.cron_notifications;
CREATE POLICY cron_notifications_user_or_service ON public.cron_notifications
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
