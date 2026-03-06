BEGIN;

CREATE TABLE IF NOT EXISTS public.routing_rules (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  rule_name VARCHAR(128) NOT NULL,
  priority INTEGER NOT NULL DEFAULT 0,
  condition_type VARCHAR(32) NOT NULL CHECK (condition_type IN ('complexity','cost','latency','capability','tier','skill','fallback')),
  condition_json JSONB NOT NULL DEFAULT '{}',
  target_model_id VARCHAR(64) NOT NULL,
  fallback_model_id VARCHAR(64),
  max_cost_cents NUMERIC(10,4),
  max_latency_ms INTEGER,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_routing_rules_priority ON public.routing_rules(priority DESC);
CREATE INDEX IF NOT EXISTS idx_routing_rules_condition ON public.routing_rules(condition_type);

ALTER TABLE public.routing_rules ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS routing_rules_service_only ON public.routing_rules;
CREATE POLICY routing_rules_service_only ON public.routing_rules
  USING (public.is_service_or_admin())
  WITH CHECK (public.is_service_or_admin());

COMMIT;
