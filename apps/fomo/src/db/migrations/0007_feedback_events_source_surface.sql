-- Phase v0.5.9 — Feedback + Learn/Grow Loop substrate (Brevio-wide).
--
-- Generalizes the existing `feedback_events` table from email-shaped to
-- Brevio-wide by adding a `source_surface` discriminator. v0.5.9 activates
-- ONE surface (`email_alert`); 12 future surfaces are declared in code via
-- `BREVIO_FEEDBACK_SURFACES` but rejected at the write gate until each one
-- runs its own 6Q gate to be added to `BREVIO_FEEDBACK_ACTIVE_SURFACES`.
--
-- Per Q1.A-modified founder lock (memory: project_v05-9-scope):
--   ✓ Additive ALTER TABLE only — no rename, no destructive change
--   ✓ NOT NULL DEFAULT 'email_alert' — Postgres backfills all existing rows
--     atomically with the column add; no separate UPDATE migration needed
--   ✓ Keep `alert_id`, `sender_email`, table name `feedback_events`
--   ✓ NO rename of `sender_email` in v0.5.9 (defer to whenever a non-email
--     surface ships and needs it)
--   ✓ NO rename of `feedback_events` table in v0.5.9 (defer)
--   ✓ NO `brevio_feedback_events` table in v0.5.9 (defer)
--
-- Founder-flagged DEFERRED columns (NOT in v0.5.9 per [[real-or-absent-no-half-wired]]
-- "do not let schema cleanup expand the phase" — uncomment ONLY when a future
-- non-email surface actually needs them and that surface runs its own 6Q gate):
--
--   -- ALTER TABLE "feedback_events"
--   --   ADD COLUMN "source_ref_type" text NULL,
--   --   ADD COLUMN "source_ref_id"   text NULL;
--   -- COMMENT ON COLUMN "feedback_events"."source_ref_type" IS
--   --   'Future-surface abstract ref pointer type (e.g. ''calendar_event'', ''browser_snapshot''). NULL for v0.5.x email_alert path which uses alert_id.';
--   -- COMMENT ON COLUMN "feedback_events"."source_ref_id" IS
--   --   'Future-surface abstract ref pointer id. NULL for v0.5.x email_alert path.';
--
-- Safety:
--   * Reversible (column add can be reverted via ALTER TABLE DROP COLUMN —
--     the scaffolding commit ships the migration SQL but the runtime commit
--     does NOT auto-apply; founder applies via psql during smoke setup AFTER
--     runtime + tests are green)
--   * No data loss possible (additive, with defaulted backfill)
--   * No index downtime (CREATE INDEX is non-blocking on small tables; will
--     be CREATE INDEX CONCURRENTLY if/when the table grows past N=1M rows;
--     for v0.5.x scale a normal CREATE INDEX is correct)
--
-- Scaffolding-vs-runtime boundary (founder-locked):
--   This migration FILE lands in the scaffolding commit so the runbook can
--   reference it. The runtime commit:
--     1. Registers this migration in `apps/fomo/src/db/migration-verifier.ts`
--        (REGISTERED_COLUMNS table-column entry for feedback_events.source_surface)
--     2. Updates `apps/fomo/src/db/schema.ts` to add the column to the
--        drizzle table definition
--     3. Adds the write-time active-surface gate in
--        `apps/fomo/src/memory/feedback-events.ts` (FeedbackStore.write)
--   The migration is APPLIED by the founder running:
--     psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql
--   AFTER the runtime commit lands AND tests are green, NOT during scaffolding.
--   The preflight script warns if the column is missing from the live DB.

ALTER TABLE "feedback_events"
  ADD COLUMN "source_surface" text NOT NULL DEFAULT 'email_alert';

CREATE INDEX "feedback_events_source_surface_idx"
  ON "feedback_events" ("user_id", "source_surface");

COMMENT ON COLUMN "feedback_events"."source_surface" IS
  'Brevio-wide surface discriminator. Must be a value in BREVIO_FEEDBACK_SURFACES (runtime-declared 13-surface enum). Write-time gate restricts to BREVIO_FEEDBACK_ACTIVE_SURFACES (v0.5.9 = [''email_alert'']). DEFAULT ''email_alert'' backfills existing v0.5.x rows.';

COMMENT ON INDEX "feedback_events_source_surface_idx" IS
  'Per-user, per-surface lookup index. Powers PIL consumer queries (count feedback events by surface in time window) and the cross-tenant isolation check.';
