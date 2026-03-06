BEGIN;

DROP POLICY IF EXISTS routing_decisions_user_or_service ON public.routing_decisions;
DROP INDEX IF EXISTS idx_routing_decisions_rule;
DROP INDEX IF EXISTS idx_routing_decisions_created;
DROP INDEX IF EXISTS idx_routing_decisions_model;
DROP INDEX IF EXISTS idx_routing_decisions_user;
DROP TABLE IF EXISTS public.routing_decisions;

COMMIT;
