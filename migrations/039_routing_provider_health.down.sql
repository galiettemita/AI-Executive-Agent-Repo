BEGIN;

DROP POLICY IF EXISTS routing_health_service_only ON public.routing_provider_health;
DROP INDEX IF EXISTS idx_routing_health_status;
DROP INDEX IF EXISTS idx_routing_health_window;
DROP INDEX IF EXISTS idx_routing_health_provider;
DROP TABLE IF EXISTS public.routing_provider_health;

COMMIT;
