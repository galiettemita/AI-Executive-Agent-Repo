BEGIN;

CREATE TABLE IF NOT EXISTS public.routing_decisions (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  message_id UUID,
  skill_id VARCHAR(64),
  selected_model_id VARCHAR(64) NOT NULL,
  rule_id UUID REFERENCES public.routing_rules(id) ON DELETE SET NULL,
  complexity_score NUMERIC(3,2),
  estimated_tokens INTEGER,
  estimated_cost_cents NUMERIC(10,4),
  actual_tokens INTEGER,
  actual_cost_cents NUMERIC(10,4),
  actual_latency_ms INTEGER,
  was_fallback BOOLEAN NOT NULL DEFAULT false,
  decision_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_routing_decisions_user ON public.routing_decisions(user_id);
CREATE INDEX IF NOT EXISTS idx_routing_decisions_model ON public.routing_decisions(selected_model_id);
CREATE INDEX IF NOT EXISTS idx_routing_decisions_rule ON public.routing_decisions(rule_id);
CREATE INDEX IF NOT EXISTS idx_routing_decisions_created ON public.routing_decisions(created_at DESC);

ALTER TABLE public.routing_decisions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS routing_decisions_user_or_service ON public.routing_decisions;
CREATE POLICY routing_decisions_user_or_service ON public.routing_decisions
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
