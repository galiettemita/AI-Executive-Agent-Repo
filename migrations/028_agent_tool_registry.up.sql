BEGIN;

CREATE TABLE IF NOT EXISTS public.agent_tool_registry (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  tool_name VARCHAR(128) NOT NULL,
  description TEXT NOT NULL,
  input_schema JSONB NOT NULL DEFAULT '{}',
  output_schema JSONB NOT NULL DEFAULT '{}',
  handler_type VARCHAR(32) NOT NULL CHECK (handler_type IN ('builtin','skill','api','function','mcp')),
  handler_config JSONB NOT NULL DEFAULT '{}',
  required_scopes TEXT[] NOT NULL DEFAULT '{}',
  rate_limit_per_minute INTEGER,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_tool_registry_name ON public.agent_tool_registry(tool_name);

ALTER TABLE public.agent_tool_registry ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_tool_registry_read ON public.agent_tool_registry;
CREATE POLICY agent_tool_registry_read ON public.agent_tool_registry
  FOR SELECT
  USING (true);

DROP POLICY IF EXISTS agent_tool_registry_write ON public.agent_tool_registry;
CREATE POLICY agent_tool_registry_write ON public.agent_tool_registry
  FOR ALL
  USING (public.is_service_or_admin())
  WITH CHECK (public.is_service_or_admin());

COMMIT;
