BEGIN;

CREATE TABLE IF NOT EXISTS public.agent_executions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  agent_id UUID NOT NULL REFERENCES public.agent_definitions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  parent_execution_id UUID REFERENCES public.agent_executions(id) ON DELETE SET NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','running','completed','failed','cancelled','timed_out')),
  input_json JSONB NOT NULL DEFAULT '{}',
  output_json JSONB DEFAULT '{}',
  iterations INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  total_cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  error_code VARCHAR(64),
  error_message TEXT,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_executions_agent ON public.agent_executions(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_user ON public.agent_executions(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_parent ON public.agent_executions(parent_execution_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_status ON public.agent_executions(status);
CREATE INDEX IF NOT EXISTS idx_agent_executions_created ON public.agent_executions(created_at DESC);

ALTER TABLE public.agent_executions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_executions_user_or_service ON public.agent_executions;
CREATE POLICY agent_executions_user_or_service ON public.agent_executions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
