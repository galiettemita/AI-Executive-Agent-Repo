BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_content_generations (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  content_type VARCHAR(32) NOT NULL CHECK (content_type IN ('blog_post','social_post','email','ad_copy','landing_page','product_description','seo_meta','press_release')),
  prompt TEXT NOT NULL,
  model_id VARCHAR(64) NOT NULL,
  generated_content TEXT NOT NULL,
  title VARCHAR(512),
  tone VARCHAR(32) CHECK (tone IN ('professional','casual','persuasive','informative','humorous','urgent','friendly')),
  target_audience TEXT,
  keywords TEXT[] NOT NULL DEFAULT '{}',
  word_count INTEGER,
  seo_score NUMERIC(5,2),
  readability_score NUMERIC(5,2),
  status VARCHAR(16) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','approved','published','archived')),
  tokens_used INTEGER NOT NULL DEFAULT 0,
  cost_cents NUMERIC(10,4) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_content_user ON public.skill_content_generations(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_content_type ON public.skill_content_generations(content_type);
CREATE INDEX IF NOT EXISTS idx_skill_content_status ON public.skill_content_generations(status);

ALTER TABLE public.skill_content_generations ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_content_user_or_service ON public.skill_content_generations;
CREATE POLICY skill_content_user_or_service ON public.skill_content_generations
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
