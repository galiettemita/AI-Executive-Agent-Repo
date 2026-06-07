-- Phase v0.5.11 — PIL substrate + shadow ranker context/eval.
--
-- Adds `sender_email_hash` column to the `alerts` table. The column stores
-- HMAC-SHA-256(sender_email, BREVIO_SENDER_HASH_KEY) — the same hash family
-- the v0.5.9 `sender_feedback_ignored` memory_signal scope_key uses. v0.5.10
-- §15 bonus finding #1 surfaced the missing thread: at reply-parse time the
-- consumer arm cannot bind to a real sender because `alerts` had no privacy-
-- safe sender identifier. The v0.5.10 `applyIgnoreSender` path worked around
-- this with a `scope_key='message:<id>'` placeholder that v0.5.11 supersedes.
--
-- Per Q4.A founder lock (memory: project_v05-11-scope) and runtime guardrail
-- #2 (no raw sender_email in new memory_signal/audit detail):
--   ✓ Additive ALTER TABLE only — no rename, no destructive change.
--   ✓ NULL on existing rows (backfill is NOT done — broad historical re-
--     derivation needs its own explicit gate per founder guardrail #1).
--     New writes by the rank step populate it forward.
--   ✓ Index for the PIL aggregation lookup path
--     (memory_signals.scope_key = alerts.sender_email_hash JOIN shape).
--
-- Privacy invariant unchanged: `alerts` still holds ONLY operational
-- identifiers + the HMAC. The cleartext sender_email is NEVER persisted on
-- this table. The Slack card payload continues to go through
-- applyEgressForSlackCard at post time and is not persisted here.

ALTER TABLE "alerts" ADD COLUMN "sender_email_hash" text;
--> statement-breakpoint
CREATE INDEX "alerts_sender_email_hash_idx" ON "alerts" USING btree ("sender_email_hash");
