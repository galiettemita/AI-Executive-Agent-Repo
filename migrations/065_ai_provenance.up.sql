-- AI content provenance tracking for watermarked responses.
CREATE TABLE ai_content_provenance (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id     UUID NOT NULL,
    workspace_id   UUID NOT NULL,
    model_id       TEXT NOT NULL,
    timestamp      TIMESTAMPTZ NOT NULL,
    watermark_hash TEXT NOT NULL,
    content_hash   TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_ai_provenance_request_id ON ai_content_provenance(request_id);
CREATE INDEX idx_ai_provenance_workspace_id ON ai_content_provenance(workspace_id);
CREATE INDEX idx_ai_provenance_content_hash ON ai_content_provenance(content_hash);
