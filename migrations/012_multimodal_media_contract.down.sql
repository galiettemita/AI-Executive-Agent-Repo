BEGIN;

DROP INDEX IF EXISTS public.messages_media_assets_gin_idx;
DROP INDEX IF EXISTS public.media_assets_workspace_created_idx;
DROP TABLE IF EXISTS public.media_assets;

ALTER TABLE public.messages
  DROP COLUMN IF EXISTS media_assets,
  DROP COLUMN IF EXISTS content_parts;

COMMIT;
