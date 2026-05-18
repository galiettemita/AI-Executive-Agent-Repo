BEGIN;

CREATE TABLE IF NOT EXISTS public.routing_user_preferences (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  preferred_provider VARCHAR(32),
  preferred_model_id VARCHAR(64),
  max_cost_per_request_cents NUMERIC(10,4),
  max_latency_ms INTEGER,
  prefer_quality_over_speed BOOLEAN NOT NULL DEFAULT true,
  allow_fallback BOOLEAN NOT NULL DEFAULT true,
  blocked_providers TEXT[] NOT NULL DEFAULT '{}',
  blocked_models TEXT[] NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_routing_user_prefs_user ON public.routing_user_preferences(user_id);

ALTER TABLE public.routing_user_preferences ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS routing_user_prefs_user_or_service ON public.routing_user_preferences;
CREATE POLICY routing_user_prefs_user_or_service ON public.routing_user_preferences
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
