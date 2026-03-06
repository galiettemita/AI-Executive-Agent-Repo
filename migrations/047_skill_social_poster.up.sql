BEGIN;

CREATE TABLE IF NOT EXISTS public.skill_social_posts (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  platform VARCHAR(32) NOT NULL CHECK (platform IN ('twitter','linkedin','instagram','facebook','tiktok','threads','bluesky')),
  content_text TEXT,
  media_urls TEXT[] NOT NULL DEFAULT '{}',
  hashtags TEXT[] NOT NULL DEFAULT '{}',
  status VARCHAR(16) NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','scheduled','posted','failed','deleted')),
  scheduled_at TIMESTAMPTZ,
  posted_at TIMESTAMPTZ,
  platform_post_id VARCHAR(256),
  platform_url TEXT,
  engagement_json JSONB DEFAULT '{}',
  campaign_id UUID REFERENCES public.marketing_campaigns(id) ON DELETE SET NULL,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_skill_social_user ON public.skill_social_posts(user_id);
CREATE INDEX IF NOT EXISTS idx_skill_social_platform ON public.skill_social_posts(platform);
CREATE INDEX IF NOT EXISTS idx_skill_social_status ON public.skill_social_posts(status);
CREATE INDEX IF NOT EXISTS idx_skill_social_campaign ON public.skill_social_posts(campaign_id);
CREATE INDEX IF NOT EXISTS idx_skill_social_scheduled ON public.skill_social_posts(scheduled_at) WHERE status = 'scheduled';

ALTER TABLE public.skill_social_posts ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS skill_social_user_or_service ON public.skill_social_posts;
CREATE POLICY skill_social_user_or_service ON public.skill_social_posts
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
