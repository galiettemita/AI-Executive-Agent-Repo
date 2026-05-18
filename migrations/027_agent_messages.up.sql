BEGIN;

CREATE TABLE IF NOT EXISTS public.agent_messages (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  execution_id UUID NOT NULL REFERENCES public.agent_executions(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  role VARCHAR(16) NOT NULL CHECK (role IN ('system','user','assistant','tool')),
  content TEXT NOT NULL,
  tool_call_id VARCHAR(128),
  tool_name VARCHAR(128),
  tool_input_json JSONB,
  tool_output_json JSONB,
  tokens_used INTEGER NOT NULL DEFAULT 0,
  iteration INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_messages_execution ON public.agent_messages(execution_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_user ON public.agent_messages(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_messages_created ON public.agent_messages(created_at);

ALTER TABLE public.agent_messages ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_messages_user_or_service ON public.agent_messages;
CREATE POLICY agent_messages_user_or_service ON public.agent_messages
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
