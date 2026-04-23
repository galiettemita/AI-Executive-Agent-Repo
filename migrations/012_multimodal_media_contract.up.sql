BEGIN;

ALTER TABLE public.messages
  ADD COLUMN IF NOT EXISTS content_parts JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS media_assets JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS public.media_assets (
  asset_id TEXT PRIMARY KEY,
  workspace_id UUID,
  source_uri TEXT,
  storage_uri TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
  sha256 CHAR(64),
  filename TEXT,
  duration_ms INTEGER CHECK (duration_ms IS NULL OR duration_ms >= 0),
  width INTEGER CHECK (width IS NULL OR width > 0),
  height INTEGER CHECK (height IS NULL OR height > 0),
  page_count INTEGER CHECK (page_count IS NULL OR page_count > 0),
  codec TEXT,
  provenance TEXT NOT NULL DEFAULT 'user_message',
  safety_labels TEXT[] NOT NULL DEFAULT '{}',
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS media_assets_workspace_created_idx
  ON public.media_assets (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS messages_media_assets_gin_idx
  ON public.messages USING GIN (media_assets);

COMMIT;
