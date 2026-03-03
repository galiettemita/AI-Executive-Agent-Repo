BEGIN;

CREATE TABLE IF NOT EXISTS public.edge_agents (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id),
  device_name VARCHAR(100) NOT NULL,
  client_cert_fingerprint CHAR(64) NOT NULL,
  last_seen_at TIMESTAMPTZ,
  status VARCHAR(20) NOT NULL DEFAULT 'offline' CHECK (status IN ('online','offline','stale')),
  os_version VARCHAR(20),
  agent_version VARCHAR(20),
  enabled_local_skills TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_edge_agents_user_id ON public.edge_agents(user_id);
CREATE INDEX IF NOT EXISTS idx_edge_agents_status ON public.edge_agents(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_edge_agents_user_fingerprint ON public.edge_agents(user_id, client_cert_fingerprint);

ALTER TABLE public.edge_agents ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS edge_agents_user_or_service ON public.edge_agents;
CREATE POLICY edge_agents_user_or_service ON public.edge_agents
USING (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
)
WITH CHECK (
  public.is_service_or_admin()
  OR user_id::text = current_setting('app.user_id', true)
);

DROP TRIGGER IF EXISTS trg_edge_agents_active_user ON public.edge_agents;
CREATE TRIGGER trg_edge_agents_active_user
BEFORE INSERT OR UPDATE ON public.edge_agents
FOR EACH ROW
EXECUTE FUNCTION public.assert_active_user();

COMMIT;
