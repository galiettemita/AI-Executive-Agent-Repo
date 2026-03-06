BEGIN;

CREATE TABLE IF NOT EXISTS public.routing_models (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  model_id VARCHAR(64) NOT NULL,
  provider VARCHAR(32) NOT NULL CHECK (provider IN ('anthropic','openai','google','mistral','cohere','local')),
  display_name VARCHAR(128) NOT NULL,
  max_tokens INTEGER NOT NULL,
  input_cost_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0,
  output_cost_per_1k NUMERIC(10,6) NOT NULL DEFAULT 0,
  capabilities TEXT[] NOT NULL DEFAULT '{}',
  context_window INTEGER NOT NULL DEFAULT 4096,
  supports_vision BOOLEAN NOT NULL DEFAULT false,
  supports_tools BOOLEAN NOT NULL DEFAULT false,
  supports_streaming BOOLEAN NOT NULL DEFAULT true,
  latency_p50_ms INTEGER,
  latency_p99_ms INTEGER,
  availability_percent NUMERIC(5,2) NOT NULL DEFAULT 99.9,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_routing_models_model_id ON public.routing_models(model_id);
CREATE INDEX IF NOT EXISTS idx_routing_models_provider ON public.routing_models(provider);

ALTER TABLE public.routing_models ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS routing_models_read ON public.routing_models;
CREATE POLICY routing_models_read ON public.routing_models
  FOR SELECT
  USING (true);

DROP POLICY IF EXISTS routing_models_write ON public.routing_models;
CREATE POLICY routing_models_write ON public.routing_models
  FOR ALL
  USING (public.is_service_or_admin())
  WITH CHECK (public.is_service_or_admin());

COMMIT;
