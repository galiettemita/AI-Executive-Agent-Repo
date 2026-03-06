BEGIN;

CREATE TABLE IF NOT EXISTS public.marketing_templates (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  template_name VARCHAR(256) NOT NULL,
  template_type VARCHAR(16) NOT NULL CHECK (template_type IN ('email','sms','social','push','webhook')),
  subject VARCHAR(512),
  body_html TEXT,
  body_text TEXT,
  variables_json JSONB DEFAULT '[]',
  is_ai_generated BOOLEAN NOT NULL DEFAULT false,
  ai_prompt TEXT,
  version INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_marketing_templates_user ON public.marketing_templates(user_id);
CREATE INDEX IF NOT EXISTS idx_marketing_templates_type ON public.marketing_templates(template_type);

ALTER TABLE public.marketing_templates ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS marketing_templates_user_or_service ON public.marketing_templates;
CREATE POLICY marketing_templates_user_or_service ON public.marketing_templates
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
