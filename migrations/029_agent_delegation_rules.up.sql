BEGIN;

CREATE TABLE IF NOT EXISTS public.agent_delegation_rules (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  supervisor_agent_id UUID NOT NULL REFERENCES public.agent_definitions(id) ON DELETE CASCADE,
  worker_agent_id UUID NOT NULL REFERENCES public.agent_definitions(id) ON DELETE CASCADE,
  delegation_condition JSONB NOT NULL DEFAULT '{}',
  priority INTEGER NOT NULL DEFAULT 0,
  max_parallel INTEGER NOT NULL DEFAULT 1,
  timeout_ms INTEGER NOT NULL DEFAULT 120000,
  fallback_agent_id UUID REFERENCES public.agent_definitions(id) ON DELETE SET NULL,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT no_self_delegation CHECK (supervisor_agent_id != worker_agent_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_delegation_supervisor ON public.agent_delegation_rules(supervisor_agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_delegation_worker ON public.agent_delegation_rules(worker_agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_delegation_fallback ON public.agent_delegation_rules(fallback_agent_id);

ALTER TABLE public.agent_delegation_rules ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS agent_delegation_service_only ON public.agent_delegation_rules;
CREATE POLICY agent_delegation_service_only ON public.agent_delegation_rules
  USING (public.is_service_or_admin())
  WITH CHECK (public.is_service_or_admin());

COMMIT;
