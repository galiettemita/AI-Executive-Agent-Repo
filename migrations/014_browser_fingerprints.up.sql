BEGIN;

CREATE TABLE IF NOT EXISTS public.browser_fingerprints (
  id UUID PRIMARY KEY DEFAULT public.uuid_v7_now(),
  user_id UUID NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  fingerprint_name VARCHAR(128) NOT NULL,
  user_agent TEXT NOT NULL,
  viewport_width INTEGER NOT NULL DEFAULT 1920,
  viewport_height INTEGER NOT NULL DEFAULT 1080,
  platform VARCHAR(32) NOT NULL DEFAULT 'Win32',
  language VARCHAR(16) NOT NULL DEFAULT 'en-US',
  timezone VARCHAR(50) NOT NULL DEFAULT 'America/New_York',
  webgl_vendor VARCHAR(128),
  webgl_renderer VARCHAR(256),
  canvas_hash CHAR(64),
  plugins_json JSONB DEFAULT '[]',
  headers_json JSONB DEFAULT '{}',
  is_default BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_browser_fingerprints_user_id ON public.browser_fingerprints(user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_browser_fingerprints_user_name ON public.browser_fingerprints(user_id, fingerprint_name);

ALTER TABLE public.browser_fingerprints ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS browser_fingerprints_user_or_service ON public.browser_fingerprints;
CREATE POLICY browser_fingerprints_user_or_service ON public.browser_fingerprints
  USING (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  )
  WITH CHECK (
    public.is_service_or_admin()
    OR user_id::text = current_setting('app.user_id', true)
  );

COMMIT;
