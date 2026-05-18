BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_form_filler_templates (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  template_name VARCHAR(128) NOT NULL,
  target_url TEXT NOT NULL,
  fields_json JSONB NOT NULL DEFAULT '[]',
  submit_selector VARCHAR(256),
  pre_fill_script TEXT,
  post_submit_action VARCHAR(32) DEFAULT 'wait'
    CHECK (post_submit_action IN ('wait','screenshot','extract','navigate')),
  success_selector VARCHAR(256),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_form_filler_user ON public.skill_form_filler_templates(user_id);

ALTER TABLE public.skill_form_filler_templates ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_form_filler_user_or_service ON public.skill_form_filler_templates;
CREATE POLICY skill_form_filler_user_or_service ON public.skill_form_filler_templates
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
