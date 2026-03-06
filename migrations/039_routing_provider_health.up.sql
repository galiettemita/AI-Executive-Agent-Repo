BEGIN;

CREATE TABLE IF NOT EXISTS public.routing_provider_health (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  provider VARCHAR(32) NOT NULL,
  model_id VARCHAR(64) NOT NULL,
  health_status VARCHAR(16) NOT NULL DEFAULT 'healthy'
    CHECK (health_status IN ('healthy','degraded','down','unknown')),
  latency_p50_ms INTEGER,
  latency_p99_ms INTEGER,
  error_rate NUMERIC(5,4) NOT NULL DEFAULT 0,
  sample_count INTEGER NOT NULL DEFAULT 0,
  window_start TIMESTAMPTZ NOT NULL,
  window_end TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_routing_health_provider ON public.routing_provider_health(provider, model_id);
CREATE INDEX IF NOT EXISTS idx_routing_health_window ON public.routing_provider_health(window_start DESC);
CREATE INDEX IF NOT EXISTS idx_routing_health_status ON public.routing_provider_health(health_status);

ALTER TABLE public.routing_provider_health ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS routing_health_service_only ON public.routing_provider_health;
CREATE POLICY routing_health_service_only ON public.routing_provider_health
  USING (public.is_service_or_admin())
  WITH CHECK (public.is_service_or_admin());

COMMIT;
