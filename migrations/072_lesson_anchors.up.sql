-- Lesson anchoring for continual learning (P3-11).
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS is_anchor BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS anchor_weight FLOAT NOT NULL DEFAULT 1.0;
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS reuse_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS confidence FLOAT NOT NULL DEFAULT 0.5;
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS workspace_adoption_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE learned_lessons ADD COLUMN IF NOT EXISTS anchored_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_lessons_is_anchor ON learned_lessons(is_anchor) WHERE is_anchor = TRUE;
CREATE INDEX IF NOT EXISTS idx_lessons_reuse_count ON learned_lessons(reuse_count DESC);

-- Lesson usage tracking for forgetting detection.
CREATE TABLE IF NOT EXISTS lesson_usages (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id    UUID NOT NULL,
    workspace_id UUID NOT NULL,
    request_id   UUID,
    used_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_lesson_usages_lesson_id ON lesson_usages(lesson_id);
CREATE INDEX IF NOT EXISTS idx_lesson_usages_used_at ON lesson_usages(used_at);
CREATE INDEX IF NOT EXISTS idx_lesson_usages_workspace_id ON lesson_usages(workspace_id);

-- Lesson reuse baselines for forgetting detection.
CREATE TABLE IF NOT EXISTS lesson_reuse_baselines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id           UUID NOT NULL,
    workspace_id        UUID NOT NULL,
    weekly_reuse_count  INTEGER NOT NULL,
    recorded_week       DATE NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_baselines_lesson_week ON lesson_reuse_baselines(lesson_id, recorded_week);
