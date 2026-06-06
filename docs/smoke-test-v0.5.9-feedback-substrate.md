# Phase v0.5.9 Smoke Test — Feedback + Learn/Grow Loop substrate (Brevio-wide)

> Founder-only smoke. **Brevio-wide substrate**, NOT FOMO/email-only. FOMO/`email_alert` is the FIRST active caller; 12 future surfaces are declared but inactive per [scope memory: v05-9-scope](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-9-scope.md).
>
> No friend involved. Three-friend cap holds.
>
> **Path A is the default:** founder-only ops-inject of a synthetic `(source_surface=email_alert, kind=ignored, dimension=sender)` event proves the end-to-end feedback → memory_signal pipe. Slack-interactivity regression secondary.

---

## §0 What changes in v0.5.9 (so you know what to look for)

| Before (v0.5.8) | After (v0.5.9) |
|---|---|
| `feedback_events` table is email-shaped (`sender_email` column + 11 email-shaped kinds) | Same table, additive `source_surface text NOT NULL DEFAULT 'email_alert'` column + `(user_id, source_surface)` index (Q1.A-modified) |
| No Brevio-wide surface discriminator | `BREVIO_FEEDBACK_SURFACES` (13 declared) + `BREVIO_FEEDBACK_ACTIVE_SURFACES = ['email_alert']` allowlist (Q2.A) |
| 11 hardcoded email-shaped event kinds | 6 generic kinds (`approved, rejected, snoozed, ignored, asked_why, corrected`) + `opened` if current caller; 10 legacy kinds map via `mapLegacyFeedbackKind`; `stop` NOT mapped (consent stays separate from preference per founder lock) (Q3.A-modified) |
| `feedback.written` audit detail carries `{user_id, alert_id, sender_email, kind, detail}` only | Same audit kind; detail extended additive: `source_surface`, `verb`, `dimension`, `role`, `legacy_kind` (Q6.A) |
| No consumer side wired (feedback captured, never applied) | NEW: `applyFeedback(event)` consumer fires inline; for `(source_surface=email_alert, kind=ignored, dimension=sender)` upserts `memory_signals(kind='sender_feedback_ignored', scope_key=<HMAC-hashed>)` (Q5.B) |
| No audit for consumer side | NEW audit kind `brevio.feedback.applied` per memory_signal upsert (Q6.C) |
| Reply parser handles only STOP/START | UNCHANGED (Q4.C — reply-parser feedback deferred to a future phase) |
| No HMR feedback-prompt surface | UNCHANGED (deferred; v0.5.7 template + voice unchanged) |

**Out of scope** (founder-locked — see [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-9-scope.md) §"Scope OUT"):
- PIL ranking behavior — `sender_feedback_ignored` is WRITE-ONLY this phase; ranker does NOT consume
- Reply-parser feedback intents — Q4.C deferred
- HMR feedback-prompt surface — own future phase
- Activating any surface beyond `email_alert`
- Renaming `feedback_events` / `sender_email` / creating `brevio_feedback_events`
- `source_ref_type` / `source_ref_id` columns (DEFER per founder lock; do NOT add unless trivial during runtime)
- New tools / new modalities / autonomy tiers / Friend C / production scale
- 3E.1 reversal (no LLM body-generation introduced)

**Privacy guardrail (founder-locked at approval time):** `memory_signals(kind='sender_feedback_ignored').scope_key` MUST be `HMAC-SHA-256(BREVIO_SENDER_HASH_KEY, user_id+':'+normalize(email))` hex-truncated to 32 chars. NO raw sender_email in `brevio.feedback.applied` detail or `sender_feedback_ignored` detail. The legacy `feedback_events.sender_email` column STAYS as v0.5.x state (not expanded, not reduced).

---

## §1 Baseline snapshot (Terminal 1, run once)

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity-check DATABASE_URL (see memory stale-database-url-shell-export):
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"
# Must end with sslmode=require.

# Baseline 1: feedback_events row count (per source_surface if column already
# applied; otherwise just total).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS total_rows FROM feedback_events;
" | tee /tmp/v0.5.9-baseline-feedback-events-count.txt

# Baseline 2: sender_feedback_ignored row count — load-bearing.
# Pre-smoke this MUST be 0 (the kind did not exist before v0.5.9).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS rows
FROM memory_signals
WHERE kind = 'sender_feedback_ignored';
" | tee /tmp/v0.5.9-baseline-sender-feedback-ignored-count.txt

# Baseline 3: brevio.feedback.applied audit count — load-bearing.
# Pre-smoke this MUST be 0 (the kind did not exist before v0.5.9).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS rows
FROM audit_log
WHERE action = 'brevio.feedback.applied';
" | tee /tmp/v0.5.9-baseline-applied-count.txt

# Baseline 4: existing stop_active rows (carry-forward — must stay identical
# across smoke per v0.5.5/8 cross-tenant pattern).
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.9-baseline-stop-active.txt

# SMOKE_START_TS — keep this; paste into queries below.
date -u +"%Y-%m-%dT%H:%M:%SZ" | tee /tmp/v0.5.9-smoke-start-ts.txt
```

Confirm all four baseline files have content. Baseline 2 + 3 MUST show 0 rows. Copy the timestamp output — paste into queries below as `<SMOKE_START_TS>`.

---

## §2 Apply migration 0007 (one-time per environment)

The migration was authored in the scaffolding commit but is NOT auto-applied. Apply once, AFTER the runtime commit lands AND tests are green:

```bash
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0007_feedback_events_source_surface.sql
```

Verify:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema='public' AND table_name='feedback_events' AND column_name='source_surface';
"
```

Expected: one row, `data_type=text`, `is_nullable=NO`, `column_default='email_alert'::text`.

Also verify all existing rows were backfilled:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT source_surface, COUNT(*)
FROM feedback_events
GROUP BY source_surface;
"
```

Expected: every row shows `source_surface=email_alert`.

---

## §3 Add v0.5.9 env vars

Append to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_9_BASELINE_CONFIRMED=true
FOMO_V0_5_9_WINDOW_HOURS=24
BREVIO_SENDER_HASH_KEY=<generate via: openssl rand -base64 32>
```

The `BREVIO_SENDER_HASH_KEY` is a NEW required env var per the founder privacy guardrail. Generate it FRESH; do NOT reuse `BREVIO_TOKEN_KEK` / `BREVIO_PHONE_HASH_KEY` / any other key (separate hash domain).

Re-source: `set -a; source apps/fomo/.env.3b3.local; set +a`.

---

## §4 Preflight + code-level sanity (Test 0)

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.9
```

Expect: `✓ Preflight passed.` After scaffolding-only commit lands, the preflight emits 4–5 WARNs (PENDING runtime commit + reminders). After runtime commit lands, all WARNs flip to silent except the `MIGRATION_0007` reminder (which is operator-action, not auto-applied).

If any ERROR fires, fix the named env var and re-run.

**Code-level sanity (post-runtime commit only):**

```bash
# C3 check: BREVIO_FEEDBACK_SURFACES contains 13 entries; ACTIVE contains only 'email_alert'
grep -n "BREVIO_FEEDBACK_SURFACES\|BREVIO_FEEDBACK_ACTIVE_SURFACES" apps/fomo/src/memory/feedback-events.ts

# C4 check: BREVIO_FEEDBACK_EVENT_KINDS + mapLegacyFeedbackKind exported
grep -n "BREVIO_FEEDBACK_EVENT_KINDS\|mapLegacyFeedbackKind" apps/fomo/src/memory/feedback-events.ts
```

**Unit-test sanity:**

```bash
pnpm --filter @brevio/fomo test src/memory/feedback-events.test.ts
pnpm --filter @brevio/fomo test src/kernel/integration-harness.test.ts
```

Expect ALL green. New tests cover:
- Active-surface accept (`email_alert` → SUCCESS)
- Active-surface reject (`calendar_reminder` → rejected with `inactive_surface` audit; NO row written) — LOAD-BEARING "not trapped in email" proof
- Unknown-surface reject (`not_a_real_surface` → rejected with `unknown_surface` audit)
- 10 legacy-kind mapping tests (one per mappable legacy kind; `stop` preserved as-is)
- Consumer pipe: feedback `(email_alert, ignored, sender)` → memory_signal upsert; `ignored_count` increments; `brevio.feedback.applied` audit fires
- Reversibility: DELETE memory_signal row → next feedback creates fresh
- Cross-tenant: write for user A does NOT touch user B's memory_signals
- Privacy canary: zero raw email substrings in new audit/memory_signal detail

---

## §5 Boot dev server (Terminal 1)

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.9.log
```

Wait for `fomo.server.listening` on port 8080. Leave running.

---

## §6 Tests

### Test 1 — Path A (LOAD-BEARING): ops-inject feedback → memory_signal pipe

**Goal:** prove the end-to-end pipe works. ops-inject a synthetic `(source_surface=email_alert, kind=ignored, dimension=sender)` event; assert the `feedback_events` row + `brevio.feedback.applied` audit + `memory_signals(sender_feedback_ignored)` upsert all fire, with structural-only privacy fields.

In Terminal 3:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Capture smoke-start timestamp:
SMOKE_START_TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "SMOKE_START_TS=$SMOKE_START_TS"

# Run ops-inject. Script lands with runtime commit. CLI shape (locked):
pnpm --filter @brevio/fomo run ops:feedback-inject -- \
  --user-id founder \
  --kind ignored \
  --source-surface email_alert \
  --dimension sender \
  --sender 'noisy-newsletter@example.com'
```

> **Operator security gate (founder-locked):** the ops-inject script REFUSES to run when `NODE_ENV=production` unless `FOMO_OPS_DEV_OVERRIDE=true` is set. No public route. No admin endpoint. CLI-only.

Wait ~5 seconds. Then query:

**Query 1 — feedback_events row** (C5 + C8):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, occurred_at, source_surface, kind, sender_email, detail
FROM feedback_events
WHERE user_id = 'founder' AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC
LIMIT 5;
"
```

Expected: ≥1 row with `source_surface='email_alert'`, generic `kind='ignored'`, `sender_email='noisy-newsletter@example.com'` (legacy column populated per v0.5.x convention), `detail.dimension='sender'`, `detail.role='user'`.

**Query 2 — feedback.written audit detail extension** (C8):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action = 'feedback.written'
  AND actor_user_id = 'founder'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC
LIMIT 3;
"
```

Expected: detail includes `source_surface='email_alert'`, `verb='ignored'`, `dimension='sender'`, `role='user'`. No subject, no body, no snippet.

**Query 3 — sender_feedback_ignored memory_signal upsert** (C9):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, scope_key, jsonb_pretty(detail) AS detail, confidence, source, updated_at
FROM memory_signals
WHERE user_id = 'founder' AND kind = 'sender_feedback_ignored'
  AND updated_at > '$SMOKE_START_TS';
"
```

Expected: 1 row. `scope_key` is a 32-char hex string (HMAC-SHA-256 truncated — NOT the plain `noisy-newsletter@example.com`). `detail.ignored_count=1`, `detail.source_surface='email_alert'`, `detail.first_ignored_at` set, `detail.last_ignored_at` set, `detail.source_feedback_event_ids=[<id>]`. `confidence≈0.6`. `source='feedback_loop_v0.5.9'`.

**Query 4 — brevio.feedback.applied audit row** (C12):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail, occurred_at
FROM audit_log
WHERE action = 'brevio.feedback.applied'
  AND actor_user_id = 'founder'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC;
"
```

Expected: 1 row. Detail includes `feedback_event_id` (matching the row from Query 1), `source_surface='email_alert'`, `verb='ignored'`, `dimension='sender'`, `memory_signal_kind='sender_feedback_ignored'`, `memory_signal_action='created'` (since pre-smoke baseline was 0), `confidence≈0.6`. No raw email anywhere.

**Reversibility sub-step (C10):**

```bash
# DELETE the signal row.
psql "$DATABASE_URL" -P pager=off -c "
DELETE FROM memory_signals
WHERE user_id = 'founder' AND kind = 'sender_feedback_ignored'
RETURNING user_id, scope_key, updated_at;
"

# Re-run ops-inject with the SAME synthetic sender:
pnpm --filter @brevio/fomo run ops:feedback-inject -- \
  --user-id founder \
  --kind ignored \
  --source-surface email_alert \
  --dimension sender \
  --sender 'noisy-newsletter@example.com'

# Verify a FRESH row (ignored_count=1, NOT resumed from prior count):
psql "$DATABASE_URL" -P pager=off -c "
SELECT (detail->>'ignored_count')::int AS ignored_count
FROM memory_signals
WHERE user_id = 'founder' AND kind = 'sender_feedback_ignored';
"
```

Expected: `ignored_count=1` (not 2). Reversibility holds.

**Pass criteria for Test 1:**
- [ ] `feedback_events` row with `source_surface='email_alert'`, generic `kind='ignored'`, `detail.dimension='sender'` (C5) ✓
- [ ] `feedback.written` audit detail extension carries `source_surface`, `verb`, `dimension`, `role` (C8) ✓
- [ ] `memory_signals(sender_feedback_ignored)` upserted; `scope_key` is hex-hashed NOT plain email; `ignored_count=1` (C9) ✓
- [ ] `brevio.feedback.applied` audit row fires with structural-only detail (C12) ✓
- [ ] Reversibility: DELETE → next inject creates fresh row (C10) ✓
- [ ] Privacy canary: no raw email substring in new audit/memory_signal detail (C16) ✓

### Test 2 — Slack interactivity regression: existing approve path still works

**Goal:** prove the existing `feedback.written` callers (Slack approve/snooze) still work after the additive extension, and that legacy kinds map to the generic taxonomy on write.

Trigger an existing polled-email-ranked-alert chain (founder Gmail receives a message → poller ranks → Slack card posts to founder DM). Click **Approve** on the Slack card.

Query:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail, occurred_at
FROM audit_log
WHERE action = 'feedback.written'
  AND actor_user_id = 'founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'verb' = 'approved'
ORDER BY occurred_at ASC
LIMIT 3;
"
```

Expected: ≥1 row. Detail includes `source_surface='email_alert'`, `verb='approved'`, `role='founder'`, `legacy_kind='founder_approved'` (since the Slack interactivity caller likely still uses the legacy kind; the mapping helper applies on write).

**Pass criteria for Test 2:**
- [ ] Slack approval writes feedback_event with `source_surface='email_alert'` (C13) ✓
- [ ] Detail carries `verb='approved'`, `role='founder'`, `legacy_kind='founder_approved'` (C13) ✓
- [ ] Existing Slack interactivity flow not broken ✓

### Test 3 — HMR regression: v0.5.7 still PASSES

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7
```

Expected: VERDICT: PASS (or identical to the documented benign FAIL shape from v0.5.8 SMOKE_REPORT §10 — window-pollution C3 FAIL when multiple smokes run in the same 24h). NEW failure shape that didn't exist in PR #46 = v0.5.9 regression — STOP and fix.

### Test 4 — Cross-tenant isolation

```bash
# Non-founder feedback.written rows in smoke window: must be 0.
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, COUNT(*) AS rows
FROM audit_log
WHERE action = 'feedback.written'
  AND occurred_at > '$SMOKE_START_TS'
  AND actor_user_id <> 'founder'
GROUP BY actor_user_id;
"

# Non-founder sender_feedback_ignored rows in smoke window: must be 0.
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, COUNT(*) AS rows
FROM memory_signals
WHERE kind = 'sender_feedback_ignored'
  AND updated_at > '$SMOKE_START_TS'
  AND user_id <> 'founder'
GROUP BY user_id;
"

# stop_active rows for non-founders must be byte-identical to baseline.
psql "$DATABASE_URL" -P pager=off -t -A -F"|" -c "
SELECT user_id, kind, detail::text, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active' AND user_id <> 'founder'
ORDER BY user_id;
" | tee /tmp/v0.5.9-post-stop-active.txt

diff /tmp/v0.5.9-baseline-stop-active.txt /tmp/v0.5.9-post-stop-active.txt || echo "(diff non-empty — see above)"
```

**Pass criterion:** zero non-founder rows in BOTH queries; stop_active diff empty (excluding presentation artifacts).

### Test 5 — Active-surface live reject (LOAD-BEARING "not trapped in email" proof)

**Goal:** prove the runtime rejects a write attempt with `source_surface='calendar_reminder'` (declared in `BREVIO_FEEDBACK_SURFACES` but NOT in `BREVIO_FEEDBACK_ACTIVE_SURFACES`).

```bash
# Attempt to inject a declared-but-inactive surface. The script should
# exit non-zero with a clear "inactive_surface" message.
pnpm --filter @brevio/fomo run ops:feedback-inject -- \
  --user-id founder \
  --kind ignored \
  --source-surface calendar_reminder \
  --dimension event \
  --sender 'unused-for-calendar' \
  2>&1 | tee /tmp/v0.5.9-test5-output.txt

# Confirm:
# 1. NO row written to feedback_events with source_surface=calendar_reminder
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM feedback_events
WHERE source_surface = 'calendar_reminder' AND occurred_at > '$SMOKE_START_TS';
"

# 2. feedback.written failure audit row with rejection_reason=inactive_surface
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action = 'feedback.written'
  AND result = 'failure'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'rejection_reason' = 'inactive_surface'
ORDER BY occurred_at ASC LIMIT 3;
"
```

**Pass criterion (C6 — LOAD-BEARING):**
- [ ] ops-inject exits non-zero
- [ ] Zero rows in `feedback_events` with `source_surface='calendar_reminder'`
- [ ] ≥1 `feedback.written` failure audit with `detail.rejection_reason='inactive_surface'` and `detail.attempted_source_surface='calendar_reminder'`

This is the proof that future surfaces are declared but inactive. The substrate is Brevio-wide; activation is gate-controlled.

---

## §7 Run all 9 evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
FOMO_V0_5_2_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
FOMO_V0_5_4_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.5
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.6
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.8
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.9
```

**Known expected non-PASS shapes (NOT v0.5.9 regressions per C14/C15 — same shapes prior SMOKE_REPORTs documented):**

- v0.5.3 may FAIL on Item #1 (no `/onboard/callback` in window — founder-only smoke, expected)
- v0.5.4 may FAIL on C13/C14 (window-slide false positives)
- v0.5.5 will FAIL C2/C3/C11 (SendBlue OPTED_OUT blocked-external — F1 own future phase)
- v0.5.7 may FAIL C3 if multiple smokes share a 24h window (window-pollution; see v0.5.8 SMOKE_REPORT §10)
- v0.5.8 may FAIL C14 if non-founder STOP-suppressed polling produced event_observed rows (v0.5.5 design preserved; see v0.5.8 SMOKE_REPORT §11)
- v0.5.6 should PASS unchanged

v0.5.9 should report VERDICT: PASS if Tests 1–5 succeeded and migration 0007 applied.

---

## §8 Fill `SMOKE_REPORT_v0.5.9.md`

```bash
cp docs/SMOKE_REPORT_TEMPLATE_v0.5.9.md docs/SMOKE_REPORT_v0.5.9.md
```

Open and fill:
- §1 prerequisites (tick what's done)
- §3 PASS criteria table with evidence
- §4–§12 paste each evidence-script output
- §13 founder observations
- §14 verdict: PASS / FAIL / PENDING
- §15 sign-off + date

---

## §9 Aftercare

- [ ] Kill Terminal 1 dev server
- [ ] If Test 1 left a fresh `stop_active` row for founder from any outbound failure during Test 2, no action (v0.5.3 drift detector behavior — not a v0.5.9 concern)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via Test 3
- [ ] Migration 0007 applied; reversible if needed (ALTER TABLE DROP COLUMN)
- [ ] No LLM call accidentally introduced (3E.1 invariant — v0.5.9 is substrate, not body-render)

---

## §10 Commit the report

```bash
git checkout phase-v0.5.9-feedback-learn-grow
git add docs/SMOKE_REPORT_v0.5.9.md
git commit -m "docs: SMOKE_REPORT_v0.5.9 VERDICT: <PASS/FAIL>"
git push origin phase-v0.5.9-feedback-learn-grow
```

---

## What v0.5.9 PASS does NOT promise

Per the [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-9-scope.md) §"What v0.5.9 PASS does NOT auto-unlock":

- ❌ **PIL substrate** — own future phase; `sender_feedback_ignored` is write-only this phase
- ❌ **Reply-parser feedback intents** — own future phase per Q4.C
- ❌ **HMR feedback-prompt surface** ("Was this the kind of thing you want me to catch?") — own future phase
- ❌ **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- ❌ **F1 SendBlue tier fix** — own future phase
- ❌ **Friend C onboarding** — three-friend cap
- ❌ **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- ❌ **3E.1 reversal** — permanently preserved

**Next phase is decided AT THE NEXT 6-question gate** with the binding three principle-gate questions + Core Dimension Check + per-phase Q1–Q6. Strategic candidates after v0.5.9 PASS: PIL substrate (reads `sender_feedback_ignored`), reply-parser feedback intents (Q4 deferred), HMR feedback-prompt surface (Q5.C deferred), F1 SendBlue tier fix, autonomy tier 1.
