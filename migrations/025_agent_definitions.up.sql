BEGIN;

CREATE TABLE IF NOT EXISTS public.agent_definitions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  agent_name VARCHAR(128) NOT NULL,
  agent_type VARCHAR(32) NOT NULL CHECK (agent_type IN ('supervisor','worker','specialist','coordinator','evaluator')),
  description TEXT,
  system_prompt TEXT NOT NULL,
  model_id VARCHAR(64) NOT NULL DEFAULT 'claude-sonnet-4-20250514',
  max_tokens INTEGER NOT NULL DEFAULT 4096,
  temperature NUMERIC(3,2) NOT NULL DEFAULT 0.7,
  tools_json JSONB NOT NULL DEFAULT '[]',
  skills TEXT[] NOT NULL DEFAULT '{}',
  max_iterations INTEGER NOT NULL DEFAULT 10,
  timeout_ms INTEGER NOT NULL DEFAULT 300000,
  is_system BOOLEAN NOT NULL DEFAULT false,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agent_definitions_user ON public.agent_definitions(user_id);
CREATE INDEX IF NOT EXISTS idx_agent_definitions_type ON public.agent_definitions(agent_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_definitions_user_name ON public.agent_definitions(user_id, agent_name);

ALTER TABLE public.agent_definitions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_definitions_user_or_service ON public.agent_definitions;
CREATE POLICY agent_definitions_user_or_service ON public.agent_definitions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
