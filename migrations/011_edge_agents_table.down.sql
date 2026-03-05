BEGIN;

DROP TRIGGER IF EXISTS trg_edge_agents_active_user ON public.edge_agents;
DROP POLICY IF EXISTS edge_agents_user_or_service ON public.edge_agents;
DROP INDEX IF EXISTS idx_edge_agents_user_fingerprint;
DROP INDEX IF EXISTS idx_edge_agents_status;
DROP INDEX IF EXISTS idx_edge_agents_user_id;
DROP TABLE IF EXISTS public.edge_agents;

COMMIT;
