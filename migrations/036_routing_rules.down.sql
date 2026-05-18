BEGIN;

DROP POLICY IF EXISTS routing_rules_service_only ON public.routing_rules;
DROP INDEX IF EXISTS idx_routing_rules_condition;
DROP INDEX IF EXISTS idx_routing_rules_priority;
DROP TABLE IF EXISTS public.routing_rules;

COMMIT;
