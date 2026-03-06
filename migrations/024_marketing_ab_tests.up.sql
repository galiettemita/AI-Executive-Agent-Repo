BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_ab_tests (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  campaign_id UUID NOT NULL REFERENCES public.marketing_campaigns(id) ON DELETE CASCADE,
  test_name VARCHAR(256) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','running','completed','cancelled')),
  variants_json JSONB NOT NULL DEFAULT '[]',
  winning_variant VARCHAR(64),
  metric_type VARCHAR(32) NOT NULL DEFAULT 'open_rate'
    CHECK (metric_type IN ('open_rate','click_rate','conversion_rate','revenue','custom')),
  confidence_level NUMERIC(5,4) NOT NULL DEFAULT 0.95,
  sample_size INTEGER,
  start_at TIMESTAMPTZ,
  end_at TIMESTAMPTZ,
  results_json JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_ab_tests_user ON public.marketing_ab_tests(user_id);
CREATE INDEX IF NOT EXISTS idx_marketing_ab_tests_campaign ON public.marketing_ab_tests(campaign_id);

ALTER TABLE public.marketing_ab_tests ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_ab_tests_user_or_service ON public.marketing_ab_tests;
CREATE POLICY marketing_ab_tests_user_or_service ON public.marketing_ab_tests
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
