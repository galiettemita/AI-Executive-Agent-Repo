BEGIN;

DROP POLICY IF EXISTS routing_models_write ON public.routing_models;
DROP POLICY IF EXISTS routing_models_read ON public.routing_models;
DROP INDEX IF EXISTS idx_routing_models_provider;
DROP INDEX IF EXISTS idx_routing_models_model_id;
DROP TABLE IF EXISTS public.routing_models;

COMMIT;
