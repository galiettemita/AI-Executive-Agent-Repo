-- Add compressed_token_count and turn_count to compression_artifacts.

ALTER TABLE compression_artifacts
    ADD COLUMN IF NOT EXISTS compressed_token_count INTEGER,
    ADD COLUMN IF NOT EXISTS turn_count            INTEGER;

CREATE INDEX IF NOT EXISTS idx_compression_artifacts_token_filter
    ON compression_artifacts (workspace_id, original_token_count, created_at DESC)
    WHERE original_token_count IS NOT NULL;
