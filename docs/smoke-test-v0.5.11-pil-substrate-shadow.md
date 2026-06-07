# Phase v0.5.11 Smoke Test Runbook — PIL Substrate + Shadow Ranker Context/Eval

> Founder-only smoke. **No friend involvement** (three-friend cap holds per `[[three-friend-beta-cap]]`).
> Live ranker behavior MUST remain unchanged. Production ranker call site passes `pil_context: null` unconditionally throughout this phase.

**Phase under Core Dimension Check discipline.** See [`memory/project_v05-11-scope.md`](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-11-scope.md).

---

## 0. Prerequisites

- v0.5.10 merged on `main` with VERDICT: PASS (commit `cb1356c6`).
- `BREVIO_SENDER_HASH_KEY` set (≥32 bytes). LOAD-BEARING this phase — used for both the new `alerts.sender_email_hash` column AND the new memory_signal scope_key AND the shadow-context lookup.
- `OPENAI_API_KEY` set. Both the ranker AND the shadow eval harness call OpenAI.
- Founder phone registered with SendBlue (carry-forward from v0.5.10).
- Working ngrok tunnel pointing at the local dev server (or accept signed-curl substitute fallback, per `[[v05-10-pass]]` documented note).
- Stale `DATABASE_URL` in shell: re-source `apps/fomo/.env.3b3.local` in every smoke terminal (`[[stale-database-url-shell-export]]`).

## 1. §1 baseline snapshot (run BEFORE booting dev server)

```bash
set -a && source apps/fomo/.env.3b3.local && set +a

# Capture baselines into /tmp for the post-smoke diff
psql "$DATABASE_URL" -P pager=off -c "
SELECT 'feedback_events' AS scope, COUNT(*) FROM feedback_events
UNION ALL SELECT 'sender_feedback_ignored', COUNT(*) FROM memory_signals WHERE kind='sender_feedback_ignored'
UNION ALL SELECT 'sender_importance (expect 0)', COUNT(*) FROM memory_signals WHERE kind='sender_importance'
UNION ALL SELECT 'sender_suppressed (expect 0)', COUNT(*) FROM memory_signals WHERE kind='sender_suppressed'
UNION ALL SELECT 'inbound_replies', COUNT(*) FROM inbound_replies
UNION ALL SELECT 'brevio.signal.aggregated audits (expect 0)', COUNT(*) FROM audit_log WHERE action='brevio.signal.aggregated'
UNION ALL SELECT 'alerts.sender_email_hash null-count (pre-smoke)', COUNT(*) FROM alerts WHERE sender_email_hash IS NULL;
" | tee /tmp/v0.5.11-baseline.txt

# Record the smoke start timestamp
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.11-smoke-start-ts.txt
```

**After capture**, export `FOMO_V0_5_11_BASELINE_CONFIRMED=true` and re-run preflight.

## 2. Env setup

Tunable env vars (Q5.C). Default values are SAFE; only override if smoke needs to test boundary behavior.

```
# Default conservative values (preflight enforces bounds)
FOMO_PIL_K_THRESHOLD=3
FOMO_PIL_SCORE_DELTA=0.1
FOMO_PIL_RECENCY_FULL_DAYS=90
FOMO_PIL_RECENCY_DECAY_DAYS=90

# Smoke gating
FOMO_V0_5_11_BASELINE_CONFIRMED=true
FOMO_V0_5_11_WINDOW_HOURS=24   # default; override only for boundary tests
```

Carry-forward env unchanged from v0.5.10.

## 3. Migration 0008 — apply alerts.sender_email_hash to live Neon

Runtime commit ships `apps/fomo/src/db/migrations/0008_alerts_sender_email_hash.sql`. **MUST be applied to live Neon BEFORE booting dev server.**

```bash
# Verify migration applied
psql "$DATABASE_URL" -P pager=off -c "
SELECT column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_name = 'alerts' AND column_name = 'sender_email_hash';
"
# Expected: sender_email_hash | text | YES
```

If migration not applied, the rank-step write will fail at boot. Apply via `pnpm --filter @brevio/fomo run db:migrate` (or the project's preferred path).

## 4. Preflight

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.11
```

Expected: PASS with WARNs only on the PENDING runtime artifacts (pil-aggregation.ts, pil-context.ts, pil-shadow.eval.ts, brevio.signal.aggregated, sender_importance/sender_suppressed). All ERRORs must be cleared before proceeding.

## 5. Boot

```bash
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.11.log
```

Wait for `fomo.server.listening`. Verify boot log shows `pil_substrate_enabled=true` (runtime commit will add this) AND that the ranker bootstrap surfaces `pil_context_live=false` (proving the live ranker path is bit-identical).

In a separate terminal, start ngrok if needed.

---

## 6. Path A — LOAD-BEARING ignore_sender natural reply

**Goal:** prove the producer + consumer arms of the new aggregation pipe both fire on a real natural reply against a v0.5.11-rank-time alert (which carries `sender_email_hash`).

### 6.1 Wait for a fresh ranked alert

Wait for the polling worker to surface ≥1 fresh alert (check Slack candidate review channel; check `audit_log` for `fomo.rank.completed` in the window). The alert must be a v0.5.11-rank-time alert — verify by querying:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, alert_state, sender_email_hash, created_at FROM alerts
WHERE user_id='founder' AND created_at > '$(cat /tmp/v0.5.11-smoke-start-ts.txt)'
ORDER BY created_at DESC LIMIT 5;
"
# Expected: at least one row with sender_email_hash IS NOT NULL
```

If no fresh alert is appearing, you can still proceed to Path B (synthetic via ops-inject). Document `Path A: skipped — no fresh alert in window` in §16 below.

### 6.2 Send the natural reply

**Primary:** founder replies `ignore this sender` (lowercase) to the alert thread on iPhone.

**Fallback (signed-curl substitute, accepted per `[[v05-10-pass]]` honest-substitute note):**

```bash
MSG_ID="sb-smoke-v0.5.11-pathA-$(date +%s)"
curl -sS -X POST "http://localhost:8080/sendblue/inbound" \
  -H "sb-signing-secret: $SENDBLUE_WEBHOOK_SECRET" \
  -H "content-type: application/json" \
  -d "{\"from_number\":\"$FOMO_FOUNDER_PHONE_NUMBER\",\"content\":\"ignore this sender\",\"message_handle\":\"$MSG_ID\"}"
```

Expected response: `{"ok":true,"intent":"ignore_sender","source":"classifier"}` (`source` is the cosmetic legacy field; the DB audit is the source of truth).

### 6.3 Verify the chain

```bash
SMOKE_START_TS=$(cat /tmp/v0.5.11-smoke-start-ts.txt)

# 6.3.1 — feedback.written audit (v0.5.10 carry-forward; sender_present should now be TRUE because alerts carry sender_email_hash)
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) FROM audit_log
WHERE action='feedback.written' AND actor_user_id='founder' AND occurred_at > '$SMOKE_START_TS'
  AND (detail->>'parser_intent')='ignore_sender'
ORDER BY occurred_at DESC LIMIT 1;
"
# Expect: detail.sender_present=true (was false in v0.5.10 smoke due to bonus finding #1)

# 6.3.2 — NEW: sender_suppressed memory_signal row created
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, scope_key, jsonb_pretty(detail) AS detail
FROM memory_signals
WHERE user_id='founder' AND kind='sender_suppressed' AND updated_at > '$SMOKE_START_TS';
"
# Expect: 1 row; scope_key matches the alert.sender_email_hash; detail.set_by='explicit_ignore_sender'

# 6.3.3 — v0.5.9 sender_feedback_ignored ALSO upserted (UNTOUCHED carry-forward)
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, scope_key, jsonb_pretty(detail) AS detail
FROM memory_signals
WHERE user_id='founder' AND kind='sender_feedback_ignored' AND updated_at > '$SMOKE_START_TS';
"
# Expect: 1 row; scope_key matches sender_suppressed.scope_key

# 6.3.4 — brevio.signal.aggregated audit fires with 15 locked fields
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) FROM audit_log
WHERE action='brevio.signal.aggregated' AND actor_user_id='founder' AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at DESC LIMIT 1;
"
# Expect: suppression_flipped=true, memory_signal_kind='sender_suppressed', memory_signal_action='created'

# 6.3.5 — privacy canary (no raw email anywhere)
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail::text FROM audit_log
WHERE action='brevio.signal.aggregated' AND actor_user_id='founder' AND occurred_at > '$SMOKE_START_TS';
" | grep -iE 'gmail\.com|icloud\.com|hotmail\.com|@yahoo\.com|noreply@|unsubscribe' && echo "CANARY HIT" || echo "canary clean"
```

### 6.4 Reversibility sub-step

```bash
# Capture the scope_key, delete the row, re-fire the aggregation
SCOPE=$(psql "$DATABASE_URL" -tA -c "SELECT scope_key FROM memory_signals WHERE user_id='founder' AND kind='sender_suppressed' ORDER BY updated_at DESC LIMIT 1;")
echo "scope_key: $SCOPE"
psql "$DATABASE_URL" -c "DELETE FROM memory_signals WHERE user_id='founder' AND kind='sender_suppressed' AND scope_key='$SCOPE';"

# Re-fire via ops:feedback-inject with the same sender hash (substitute sender_email)
pnpm --filter @brevio/fomo run -s ops:feedback-inject -- \
  --user-id founder --kind ignored --source-surface email_alert --dimension sender \
  --sender 'sender-bound-to-scope@example.com'

# Verify a NEW row created (memory_signal_action='created')
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail->>'memory_signal_action' FROM audit_log
WHERE action='brevio.signal.aggregated' AND actor_user_id='founder'
ORDER BY occurred_at DESC LIMIT 1;
"
# Expect: 'created' (NOT 'updated')
```

---

## 7. Path B — positive intents

### 7.1 "this mattered" via natural reply (or curl substitute)

```bash
MSG_ID="sb-smoke-v0.5.11-pathB-tm-$(date +%s)"
curl -sS -X POST "http://localhost:8080/sendblue/inbound" \
  -H "sb-signing-secret: $SENDBLUE_WEBHOOK_SECRET" \
  -H "content-type: application/json" \
  -d "{\"from_number\":\"$FOMO_FOUNDER_PHONE_NUMBER\",\"content\":\"this mattered\",\"message_handle\":\"$MSG_ID\"}"
```

Verify:
- `feedback_events` row with `kind=approved`, `dimension=importance`
- `sender_importance` memory_signal row with `score=+δ` (default 0.1) and `n_positive_events=1`
- `brevio.signal.aggregated` audit fires with `suppression_flipped=false`
- `sender_suppressed` row UNCHANGED (positive intent does not flip suppression)

### 7.2 "more like this"

Same shape; `score` should shift by `+2δ` (default 0.2). Verify on the same scope_key as Path B.7.1 — `n_positive_events=2`, `score` accumulates.

---

## 8. Path C — threshold-k flip via consecutive false_positive

### 8.1 Use a NEW synthetic sender (so we can observe the flip cleanly)

```bash
# Fire k=3 (default) consecutive false_positive events on a synthetic sender
for i in 1 2 3; do
  pnpm --filter @brevio/fomo run -s ops:feedback-inject -- \
    --user-id founder --kind corrected --source-surface email_alert --dimension ranker_label \
    --sender "smoke-fp-test-v0.5.11@example.com"
done

# Query: sender_importance.score should be approximately -3δ = -0.3
# sender_suppressed should NOW be true (n_negative_events >= k)
psql "$DATABASE_URL" -P pager=off -c "
SELECT kind, jsonb_pretty(detail) AS detail FROM memory_signals
WHERE user_id='founder' AND scope_key = (
  SELECT scope_key FROM memory_signals
  WHERE kind='sender_importance' AND user_id='founder' ORDER BY updated_at DESC LIMIT 1
)
ORDER BY kind;
"
# Expect:
#   sender_importance: score=-0.3, n_negative_events=3
#   sender_suppressed: set_by='threshold_negative_aggregation'

# Also verify the LAST brevio.signal.aggregated has suppression_flipped=true on the 3rd write
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail->>'suppression_flipped', detail->>'n_negative_events_after'
FROM audit_log
WHERE action='brevio.signal.aggregated' AND actor_user_id='founder'
ORDER BY occurred_at DESC LIMIT 3;
"
# Expect: 3rd row (newest) shows suppression_flipped=true, n_negative_events_after=3
```

### 8.2 One-correction-no-flip sanity (use a fresh synthetic sender)

```bash
pnpm --filter @brevio/fomo run -s ops:feedback-inject -- \
  --user-id founder --kind approved --source-surface email_alert --dimension importance \
  --sender "smoke-one-positive-test-v0.5.11@example.com"

# Verify: sender_importance.score = +0.1; sender_suppressed row DOES NOT EXIST for this scope_key
```

---

## 9. Path D — shadow eval harness

```bash
pnpm --filter @brevio/fomo run -s eval:pil-shadow 2>&1 | tee /tmp/v0.5.11-pil-shadow-eval.log
```

Expected output: per-fixture lines reporting baseline-score vs PIL-augmented-score + PASS/FAIL on shift direction. Final line: `VERDICT: PASS`.

The eval calls OpenAI; expect ≥30s runtime per ≥10 fixtures. **Live production ranker call site is NOT involved in this harness.**

---

## 10. Path E — cross-user contamination (LOAD-BEARING)

```bash
# Inject as a synthetic user A
pnpm --filter @brevio/fomo run -s ops:feedback-inject -- \
  --user-id userA-smoke-v0.5.11 --kind ignored --source-surface email_alert --dimension sender \
  --sender "cross-tenant-test@example.com"

# Capture the scope_key user A wrote
USER_A_SCOPE=$(psql "$DATABASE_URL" -tA -c "
  SELECT scope_key FROM memory_signals
  WHERE user_id='userA-smoke-v0.5.11' AND kind='sender_suppressed'
  ORDER BY updated_at DESC LIMIT 1;
")
echo "user A scope_key: $USER_A_SCOPE"

# Verify: user B (founder) has NO memory_signal with that scope_key
psql "$DATABASE_URL" -P pager=off -c "
SELECT kind, user_id FROM memory_signals
WHERE scope_key='$USER_A_SCOPE' AND user_id <> 'userA-smoke-v0.5.11';
"
# Expect: 0 rows

# Shadow eval cross-user fixture (must be ≥1 row in pil-shadow.eval.ts):
# user B's score for sender X must equal baseline regardless of user A's signal
# This is asserted inside the harness; review /tmp/v0.5.11-pil-shadow-eval.log
grep -i 'cross_user\|cross-user' /tmp/v0.5.11-pil-shadow-eval.log
```

---

## 11. Live ranker invariant — bit-identical verification

```bash
# Verify PROMPT_VERSION still ranker-v0.2.0
grep -rn "ranker-v0\." apps/fomo/src/ranker/prompt.ts
# Expect: PROMPT_VERSION = 'ranker-v0.2.0'

# Verify production ranker call site does NOT import buildPilContext
grep -rn "buildPilContext" apps/fomo/src/workers/ apps/fomo/src/ranker/rank-email.ts
# Expect: 0 matches in workers/, 0 matches in rank-email (eval imports only)

# Verify rank_results schema unchanged
psql "$DATABASE_URL" -P pager=off -c "\d rank_results"
# Expect: same columns as v0.5.10 baseline (no new pil_* columns)
```

---

## 12. Carry-forward — v0.5.9 + v0.5.10 still PASS

```bash
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.9 2>&1 | tail -15
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.10 2>&1 | tail -15
```

Expected: both still PASS or match the documented benign shapes from prior reports. v0.5.9 substrate UNTOUCHED is load-bearing.

---

## 13. Run smoke-evidence:v0.5.11

```bash
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.11 2>&1 | tee /tmp/v0.5.11-evidence.log
```

Expected: `VERDICT: PASS` (with operator-confirmed PENDINGs for the items the script can't observe directly: ≤3-word safe rule absence, eval-side cross-user, reversibility cycle).

---

## 14. Clean stop

```bash
# Optional cleanup of synthetic test rows (founder discretion)
# psql "$DATABASE_URL" -c "DELETE FROM memory_signals WHERE scope_key IN (...);"
# psql "$DATABASE_URL" -c "DELETE FROM feedback_events WHERE sender_email LIKE 'smoke-%@example.com';"

# Ctrl-C the dev server. Confirm shutdown clean.
# Ctrl-C ngrok if used.
```

---

## 15. Fill in the report

Copy `docs/SMOKE_REPORT_TEMPLATE_v0.5.11.md` → `docs/SMOKE_REPORT_v0.5.11.md`. Fill in all 18 PASS criteria + operator-confirmed evidence + privacy canary diff + sample JSON for new audit rows. Commit on the same branch.

---

## 16. Operator notes / honest substitutions

If any path was substituted (e.g. Path A natural reply blocked by SendBlue tier; used signed-curl), document the substitution here and in the report. **Do not oversell the substitute as full live proof** — same rule as v0.5.10 (`[[v05-10-pass]]` founder caveat).

If the shadow eval reports model-nondeterministic flakiness on shift magnitude (but direction is consistent), that's acceptable per Q1 (eval asserts DIRECTION not MAGNITUDE). Document flaky fixtures by ID; investigate if >2/10 are flaky.

If Path A skipped because no fresh ranked alert appeared during the smoke window, the LOAD-BEARING ignore_sender chain is demonstrated via Path B's ops-inject substitute pattern (same code path; documented honest substitution per `[[real-or-absent-no-half-wired]]`).

---

## 17. Aftercare

- v0.5.7 HMR template unchanged (no renderer edits this phase).
- v0.5.9 substrate untouched: `BREVIO_FEEDBACK_SURFACES`=13, ACTIVE=`['email_alert']`, `sender_feedback_ignored` rows untouched.
- v0.5.10 reply-parser intent set unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents.
- 3E.1 invariant: no LLM in body composition; only `rank.reason` is model-generated.
- No new active surface beyond `email_alert`.
- No live ranker behavior change.
- No outbound alert behavior change.
