# Phase v0.5.10 Smoke Test — Reply-parser feedback intents (connect natural replies → v0.5.9 substrate)

> Founder-only smoke. **NOT greenfield** — the v0.1.0 reply-parser ships (Phase 3F.1); the v0.5.9 Brevio-wide Feedback substrate ships ([[v05-9-pass]]). v0.5.10 connects them via a new `feedback-routing.ts` policy module AND adds 2 NEW positive-signal intents (`this_mattered`, `more_like_this`) to the classifier.
>
> Path A default: founder texts a natural reply (e.g. "ignore this sender") to a real Brevio iMessage alert thread. If SendBlue inbound is tier-blocked, fall back to signed-curl substitute per [[v05-2-pass]] pattern.

---

## §0 What changes in v0.5.10

| Before (v0.5.9) | After (v0.5.10) |
|---|---|
| Reply-parser v0.1.0 classifies 6 intents (`snooze, ignore, ignore_sender, why, false_positive, unclear`) | Reply-parser v0.2.0 classifies 8 intents (adds `this_mattered, more_like_this`) |
| `sendblue-inbound.ts` has scattered `switch (c.intent)` block writing feedback_events with legacy kinds, NO `source_surface`, NO `applyFeedback` invocation | NEW `feedback-routing.ts` policy module is the single testable place for "intent → feedback_event_input + applyFeedback" decisions. Route handler calls one function. |
| `ignore_sender` natural reply writes feedback_event but does NOT fire the v0.5.9 `sender_feedback_ignored` consumer | `ignore_sender` natural reply → routing module → feedback_event + `applyFeedback` → `sender_feedback_ignored` upsert + `brevio.feedback.applied` audit. END-TO-END. |
| No positive-signal natural-reply intents | `this_mattered` and `more_like_this` ship; both write feedback_events with positive mapping (verb=approved, dimension=importance/pattern, role=user, value=confirmed_important/more_like_this). NO memory_signal write (Q4.A lock). |
| Existing 0.7 global confidence threshold; classifier sees all replies | Same 0.7 threshold + NEW ≤3-word safe rule + NEW explicit-feedback-phrase allowlist absorbed into `parseReplyDeterministic` so the LLM never sees canonical short feedback phrases. |
| `feedback.written` audit detail carries v0.5.9 fields (source_surface, verb, dimension, role, legacy_kind, feedback_event_id) | Same fields PLUS new 4: `intent_source`, `inbound_reply_id`, `parser_intent`, `parser_confidence`. Hard privacy rule: NO raw reply text / subject / body / snippet / headers / sender_email. |
| `slack-interactivity.ts` + `ops:feedback-inject` emit `feedback.written` without `intent_source` | Both add `intent_source` for symmetry (`slack_interactivity` / `ops_inject`). 1-line additive change to each. |

**Out of scope** (founder-locked — see [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-10-scope.md) §"Scope OUT"):
- PIL ranking consumption of any feedback-derived signal
- New `applyFeedback` consumer arms beyond v0.5.9's `(email_alert, ignored, sender)`
- HMR feedback-prompt surface / acknowledgment iMessage (future-phase candidate: HMR Feedback Acknowledgment)
- Auto-send / new tools / new agentic surfaces / Friend C / production scale
- Renaming v0.1.0 intents
- Per-intent confidence calibration
- Storing reply text in any column / detail field
- STOP/START as preference feedback
- 3E.1 reversal

---

## §1 Baseline snapshot (Terminal 1, run once)

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity-check DATABASE_URL (per memory stale-database-url-shell-export):
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"
# Must end with sslmode=require.

# Baseline 1: feedback_events row count (will verify no row loss + new
# inserts during smoke).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS total_rows FROM feedback_events;
" | tee /tmp/v0.5.10-baseline-feedback-events-count.txt

# Baseline 2: existing sender_feedback_ignored signal rows for founder
# (Test 1 will create a NEW one or update an existing one).
psql "$DATABASE_URL" -P pager=off -c "
SELECT scope_key, (detail->>'ignored_count')::int AS ignored_count, updated_at
FROM memory_signals
WHERE user_id = 'founder' AND kind = 'sender_feedback_ignored'
ORDER BY updated_at DESC LIMIT 10;
" | tee /tmp/v0.5.10-baseline-sender-feedback-ignored.txt

# Baseline 3: inbound_replies count (Test 1 will add 1 new row).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS total_rows FROM inbound_replies;
" | tee /tmp/v0.5.10-baseline-inbound-replies-count.txt

# Baseline 4: stop_active rows (cross-tenant carry-forward; non-founder
# rows must be byte-identical post-smoke per C12).
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.10-baseline-stop-active.txt

# Baseline 5: recent feedback.written audit count (none should yet carry
# the new intent_source field — that's a v0.5.10 runtime artifact).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS rows_with_intent_source
FROM audit_log
WHERE action = 'feedback.written'
  AND detail ? 'intent_source';
"

# SMOKE_START_TS — keep this; paste into queries below.
date -u +"%Y-%m-%dT%H:%M:%SZ" | tee /tmp/v0.5.10-smoke-start-ts.txt
```

Confirm all five baseline files have content.

---

## §2 Add v0.5.10 env vars

Append to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_10_BASELINE_CONFIRMED=true
FOMO_V0_5_10_WINDOW_HOURS=24
```

**No new env var is required this phase.** The `BREVIO_SENDER_HASH_KEY` from v0.5.9 is reused by the `ignore_sender` → `sender_feedback_ignored` consumer pipe.

Re-source: `set -a; source apps/fomo/.env.3b3.local; set +a`.

---

## §3 Preflight + code-level sanity + unit tests

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.10
```

Expect: `✓ Preflight passed.` After scaffolding-only commit: 4–5 WARNs (PENDING runtime + operator reminders). After runtime commit: all WARNs silent except the migration-style operator reminders.

**Code-level sanity (post-runtime):**

```bash
# C3 check: PROMPT_VERSION bumped to v0.2.0
grep -n "PROMPT_VERSION" apps/fomo/src/reply-parser/prompt.ts

# Q3.C allowlist absorbed into deterministic.ts
grep -n "this mattered\|more like this\|not important\|ignore this sender" apps/fomo/src/reply-parser/deterministic.ts

# Routing module exports
grep -n "export.*routeReplyFeedback" apps/fomo/src/reply-parser/feedback-routing.ts
```

**Unit-test sanity (load-bearing for C4, C5, C10, C11):**

```bash
pnpm --filter @brevio/fomo test src/reply-parser/index.test.ts
pnpm --filter @brevio/fomo test src/reply-parser/deterministic.test.ts
pnpm --filter @brevio/fomo test src/reply-parser/prompt.test.ts
pnpm --filter @brevio/fomo test src/reply-parser/feedback-routing.test.ts
pnpm --filter @brevio/fomo test src/routes/sendblue-inbound.test.ts
```

Expected: all green. New tests cover:

- Each allowlist phrase → deterministic match with confidence=1.0 (C4)
- ≤3-word non-allowlist with high LLM confidence → forced unclear (C5)
- All 8 intents → expected feedback_event shape (C6, C7, C8, C9)
- `unclear` → routing module returns `unclear_no_op` and writes nothing (C10)
- Idempotency: duplicate provider_message_id → ONE write (C11)
- Cross-tenant: write for user A does NOT touch user B (C12)
- Privacy canary on assembled audit detail (C13)

---

## §4 Boot dev server (Terminal 1)

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.10.log
```

Wait for `fomo.server.listening` on port 8080. Leave running.

---

## §5 ngrok (Terminal 2, RECOMMENDED for natural reply flow)

v0.5.10 exercises the SendBlue inbound webhook path. Real iMessage replies need a public HTTPS endpoint.

```bash
ngrok http --domain=unshivering-interaulic-beatriz.ngrok-free.dev 8080
```

If SendBlue inbound webhooks don't fire (per [[sendblue-plan-gates]] tier-block on the founder phone), the §6 Test 1 fallback uses signed-curl substitution.

---

## §6 Tests

### Test 1 — Path A (LOAD-BEARING): natural "ignore this sender" reply → full chain

**Goal:** prove the end-to-end pipe works for the canonical natural-reply use case. Founder texts "ignore this sender" to a real Brevio iMessage thread; the deterministic-allowlist match fires; the routing module writes the feedback_event AND invokes `applyFeedback`; the `sender_feedback_ignored` memory_signal upserts; `brevio.feedback.applied` audit fires.

**Step 1 — Generate a Brevio alert:** founder sends themselves an important email via Gmail-to-self. Wait for polling to rank + Slack-post + outbound-send via SendBlue. Founder receives the iMessage on their phone.

**Step 2 — Founder replies:** in the iMessage thread, founder texts: `ignore this sender`

**Step 3 — Wait ~10 seconds** for SendBlue inbound webhook to fire (or for the curl substitute below if needed).

**Step 4 — Query (load-bearing):**

```bash
SMOKE_START_TS=$(cat /tmp/v0.5.10-smoke-start-ts.txt)

# C14: feedback_events row + 10-field audit detail
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, source_surface, kind, sender_email, detail->>'dimension' AS dimension
FROM feedback_events
WHERE user_id='founder' AND occurred_at > '$SMOKE_START_TS'
ORDER BY id ASC LIMIT 5;
"

psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action='feedback.written' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'parser_intent'='ignore_sender'
ORDER BY occurred_at ASC LIMIT 3;
"

# C6/C14: brevio.feedback.applied audit row
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action='brevio.feedback.applied' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC LIMIT 3;
"

# C6: sender_feedback_ignored memory_signal upsert (NEW or UPDATED)
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, scope_key, jsonb_pretty(detail) AS detail, confidence, updated_at
FROM memory_signals
WHERE user_id='founder' AND kind='sender_feedback_ignored'
  AND updated_at > '$SMOKE_START_TS'
ORDER BY updated_at DESC LIMIT 3;
"
```

**Expected:**
- ≥1 `feedback_events` row with `source_surface='email_alert'`, `kind='ignored'` (generic verb direct from routing module), `sender_email=<founder's own gmail>` (legacy column preserved per v0.5.9), `detail.dimension='sender'`
- ≥1 `feedback.written` audit row with detail carrying ALL 10 locked fields: `intent_source='reply_parser_deterministic'` (deterministic allowlist hit), `inbound_reply_id=<bigint>`, `parser_intent='ignore_sender'`, `parser_confidence=1.0`, `source_surface='email_alert'`, `verb='ignored'`, `dimension='sender'`, `role='user'`, `feedback_event_id=<bigint>` (legacy_kind absent — direct generic verb)
- ≥1 `brevio.feedback.applied` audit row with `memory_signal_kind='sender_feedback_ignored'`, `memory_signal_action ∈ {'created', 'updated'}`, `memory_signal_scope_key_hash=<32 hex>` (NEVER the raw sender_email)
- `sender_feedback_ignored` memory_signal row upserted (if baseline had a row for this sender_email hash, action=updated and ignored_count incremented; if not, action=created and ignored_count=1)

**Fallback if SendBlue inbound webhook does NOT fire** (per [[v05-2-pass]] / [[sendblue-plan-gates]]):

```bash
# Signed-curl substitute. Mints a synthetic SendBlue webhook payload with
# the founder phone + a synthetic provider_message_id + the reply text
# "ignore this sender". The SENDBLUE_INBOUND_SIGNING_SECRET signs the body
# so the route's HMAC gate passes.
PROVIDER_MSG_ID="smoke-v0.5.10-test1-$(date +%s)"
BODY=$(jq -nc --arg msgid "$PROVIDER_MSG_ID" --arg phone "$FOMO_FOUNDER_PHONE_NUMBER" \
  '{accountEmail: "test@example-smoke.test", content: "ignore this sender", from_number: $phone, message_handle: $msgid, date_received: now | todateiso8601}')
SIG=$(printf '%s' "$BODY" | openssl dgst -sha256 -hmac "$SENDBLUE_WEBHOOK_SECRET" -hex | awk '{print $2}')
curl -X POST "https://$FOMO_FRIEND_BETA_BASE_URL_HOST/sendblue/inbound" \
  -H "sb-signing-secret: $SIG" \
  -H 'content-type: application/json' \
  --data "$BODY"
```

Then re-run the queries above. Substitution produces the same audit + memory_signal shape; only `intent_source` may differ if the synthetic phrasing is recognized by the allowlist (still `reply_parser_deterministic`).

**Pass criteria for Test 1:**
- [ ] `feedback.written` audit detail carries all 10 locked fields (C1) ✓
- [ ] `feedback_events.source_surface='email_alert'` (C2) ✓
- [ ] `ignore_sender` intent → `applyFeedback` → `sender_feedback_ignored` upsert (C6) ✓
- [ ] `brevio.feedback.applied` audit fires (C6) ✓
- [ ] Memory_signal scope_key is HMAC-hashed (NEVER plain email) (carry-forward v0.5.9) ✓
- [ ] Audit + memory_signal detail contains NO raw reply text / sender_email substring (C13) ✓

### Test 2 — Positive intent "this mattered"

**Goal:** prove `this_mattered` writes a positive-signal feedback_event but does NOT fire any memory_signal upsert.

**Step 1 — Generate another Brevio alert** (or use the existing thread; SendBlue + Brevio inbox-route by from-phone, not thread id).

**Step 2 — Founder replies:** `this mattered`

**Step 3 — Query:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action='feedback.written' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'parser_intent'='this_mattered'
ORDER BY occurred_at ASC LIMIT 3;
"
```

**Expected:** detail carries `intent_source='reply_parser_deterministic'`, `parser_intent='this_mattered'`, `parser_confidence=1.0`, `verb='approved'`, `dimension='importance'`, `role='user'`, `value='confirmed_important'`.

**Also verify NO memory_signal write fired:**

```bash
# No NEW or UPDATED sender_feedback_ignored row should fire from this_mattered.
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS new_or_updated_signals
FROM memory_signals
WHERE user_id='founder' AND kind='sender_feedback_ignored'
  AND updated_at > (
    SELECT occurred_at FROM audit_log
    WHERE action='feedback.written' AND detail->>'parser_intent'='this_mattered'
      AND actor_user_id='founder' AND occurred_at > '$SMOKE_START_TS'
    ORDER BY occurred_at DESC LIMIT 1
  );
"

# No brevio.feedback.applied audit should fire for this intent.
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS applied_audits_after_this_mattered
FROM audit_log
WHERE action='brevio.feedback.applied' AND actor_user_id='founder'
  AND occurred_at > (
    SELECT occurred_at FROM audit_log
    WHERE action='feedback.written' AND detail->>'parser_intent'='this_mattered'
      AND actor_user_id='founder' AND occurred_at > '$SMOKE_START_TS'
    ORDER BY occurred_at DESC LIMIT 1
  );
"
```

**Expected:** both counts = 0 (consumer arm doesn't fire for `this_mattered`).

**Pass criteria for Test 2:**
- [ ] `feedback.written` detail carries positive-signal mapping (verb=approved, dimension=importance, value=confirmed_important) (C7) ✓
- [ ] NO `sender_feedback_ignored` upsert after this reply ✓
- [ ] NO `brevio.feedback.applied` audit after this reply ✓
- [ ] Privacy canary clean (C13) ✓

### Test 3 — ≤3-word safe rule fail-safe

**Goal:** prove a 2-word non-allowlist reply with potentially high LLM confidence is forced to `unclear`.

**Founder texts:** `got it` (2 words, NOT in the allowlist).

**Query:**

```bash
# Expected: NO feedback.written row with parser_intent set; the existing
# fomo.sendblue.reply_unclear audit fires instead.
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS feedback_rows_after_got_it
FROM audit_log
WHERE action='feedback.written' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'parser_intent' IS NOT NULL
  AND occurred_at > (SELECT NOW() - INTERVAL '5 minutes');
"

psql "$DATABASE_URL" -P pager=off -c "
SELECT detail->>'classified_intent' AS intent, occurred_at
FROM audit_log
WHERE action='fomo.sendblue.reply_parsed' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at DESC LIMIT 3;
"
```

**Expected:** zero NEW feedback.written rows post-"got it"; `fomo.sendblue.reply_parsed` (or `reply_unclear`) row with classified_intent=`unclear`.

**Pass criteria for Test 3 (C5):**
- [ ] NO feedback_event written for "got it" ✓
- [ ] Existing reply-unclear audit fires ✓

### Test 4 — STOP regression: no v0.5.10 feedback_event written

**Goal:** prove the deterministic STOP/START compliance path is unchanged; STOP/START NEVER produces a v0.5.10 feedback_event.

**Founder texts:** `STOP`

**Query:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS feedback_rows_with_parser_intent
FROM audit_log
WHERE action='feedback.written' AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND occurred_at > (SELECT NOW() - INTERVAL '5 minutes');
"

# Existing v0.5.5 stop_active flip should fire.
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail, updated_at FROM memory_signals
WHERE user_id='founder' AND kind='stop_active';
"
```

**Expected:** zero `feedback.written` rows with `parser_intent`; `stop_active` memory_signal flipped to `active=true`.

**After Test 4, send `START` to restore the founder's outbound state** (re-source env if needed).

**Pass criteria for Test 4:**
- [ ] STOP did NOT produce a v0.5.10 feedback_event ✓
- [ ] Existing deterministic compliance path fired (`stop_active=true`) ✓

### Test 5 — Cross-tenant

```bash
# Non-founder feedback.written + sender_feedback_ignored counts in window.
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, COUNT(*) AS rows
FROM audit_log
WHERE action IN ('feedback.written','brevio.feedback.applied')
  AND occurred_at > '$SMOKE_START_TS' AND actor_user_id IS NOT NULL AND actor_user_id <> 'founder'
GROUP BY actor_user_id;
"

# Non-founder stop_active byte-identical to baseline (carry-forward).
psql "$DATABASE_URL" -P pager=off -t -A -F"|" -c "
SELECT user_id, kind, detail::text, source, updated_at
FROM memory_signals
WHERE kind='stop_active' AND user_id <> 'founder'
ORDER BY user_id;
" | tee /tmp/v0.5.10-post-stop-active.txt

diff /tmp/v0.5.10-baseline-stop-active.txt /tmp/v0.5.10-post-stop-active.txt
```

**Pass criterion (C12):** zero non-founder rows; stop_active diff empty (excluding presentation artifacts).

---

## §7 Run all 10 evidence scripts

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
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.10
```

**Known expected non-PASS shapes (NOT v0.5.10 regressions per C16):**

- v0.5.3 may FAIL on Item #1 (no `/onboard/callback` in founder-only smoke)
- v0.5.4 may FAIL on C13 (Morris stop_active window-slide)
- v0.5.5 may FAIL C2/C3/C5/C11 (SendBlue OPTED_OUT blocked-external)
- v0.5.7 may FAIL C3 (same-day multi-smoke window pollution — see v0.5.8/v0.5.9 SMOKE_REPORT §10 for the documented benign pattern)
- v0.5.8 may FAIL C14 (v0.5.5 STOP-suppressed polling preserved-by-design)

v0.5.10 should report VERDICT: PASS if Tests 1–5 succeeded.

---

## §8 Fill `SMOKE_REPORT_v0.5.10.md`

```bash
cp docs/SMOKE_REPORT_TEMPLATE_v0.5.10.md docs/SMOKE_REPORT_v0.5.10.md
```

Open and fill §1 prerequisites, §3 PASS criteria with evidence, §4–§13 evidence-script outputs, §14 verdict, §15 sign-off.

---

## §9 Aftercare

- [ ] Kill Terminal 1 dev server + Terminal 2 ngrok
- [ ] If Test 4 (STOP) left founder in stop_active=true, the founder texted START before this step. Verify `stop_active=false` (or row deleted) so the outbound substrate is restored.
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0`
- [ ] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES (13) + ACTIVE (['email_alert']) + sender_feedback_ignored memory_signal kind intact
- [ ] No LLM call accidentally introduced into renderer (3E.1 invariant — v0.5.10 is reasoning, not body composition)

---

## §10 Commit the report

```bash
git checkout phase-v0.5.10-reply-parser-feedback
git add docs/SMOKE_REPORT_v0.5.10.md
git commit -m "docs: SMOKE_REPORT_v0.5.10 VERDICT: <PASS/FAIL>"
git push origin phase-v0.5.10-reply-parser-feedback
```

---

## What v0.5.10 PASS does NOT promise

Per the [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-10-scope.md) §"What v0.5.10 PASS does NOT auto-unlock":

- ❌ **PIL substrate** — own future phase
- ❌ **HMR Feedback Acknowledgment / Feedback Prompt Surface** — own future phase per Q5.A defer
- ❌ **Positive-signal memory_signal kinds** — own future phase per Q4.A defer
- ❌ **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- ❌ **F1 SendBlue tier fix**
- ❌ **Friend C onboarding** — three-friend cap
- ❌ **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- ❌ **3E.1 reversal**
- ❌ **Storing reply text in any column**

**Next phase is decided AT THE NEXT 6-question gate** with the binding three principle-gate questions + Core Dimension Check + per-phase Q1–Q6.
