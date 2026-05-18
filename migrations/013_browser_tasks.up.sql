BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_tasks (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  session_id UUID NOT NULL REFERENCES public.browser_sessions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  skill_id VARCHAR(64) NOT NULL,
  task_type VARCHAR(32) NOT NULL CHECK (task_type IN ('navigate','click','type','extract','screenshot','scroll','wait','script')),
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','completed','failed','cancelled')),
  target_selector TEXT,
  input_value TEXT,
  result_json JSONB DEFAULT '{}',
  screenshot_url TEXT,
  error_code VARCHAR(64),
  error_message TEXT,
  sequence_order INTEGER NOT NULL DEFAULT 0,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_tasks_session_id ON public.browser_tasks(session_id);
CREATE INDEX IF NOT EXISTS idx_browser_tasks_user_id ON public.browser_tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_browser_tasks_status ON public.browser_tasks(status);

ALTER TABLE public.browser_tasks ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_tasks_user_or_service ON public.browser_tasks;
CREATE POLICY browser_tasks_user_or_service ON public.browser_tasks
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
