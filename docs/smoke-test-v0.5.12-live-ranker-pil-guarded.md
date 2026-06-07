# Phase v0.5.12 Smoke Test Runbook — Live Ranker Reads PIL in Guarded Mode

> Founder-only smoke. **No friend involvement** (three-friend cap holds per `[[three-friend-beta-cap]]`).
> v0.5.12 is the FIRST phase where the live ranker actually reads PIL signals at rank time. v0.5.11 made PIL substrate + shadow eval live; v0.5.12 turns it into bounded, eval-gated, kill-switchable production behavior.
>
> **Phase under Core Dimension Check discipline.** See [`memory/project_v05-12-scope.md`](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-12-scope.md).

---

## 0. Prerequisites

- v0.5.11 merged on `main` with VERDICT: PASS (merge `762439c3`).
- `BREVIO_SENDER_HASH_KEY` set (≥32 bytes). LOAD-BEARING — the live ranker hashes sender_email at read time using this key; drift here = silent zero-PIL-context blindness.
- `OPENAI_API_KEY` set. LOAD-BEARING — the two-call hybrid runs TWO ranker calls per PIL-influenced rank; the offline `pil-live.eval.ts` also calls OpenAI.
- Founder phone registered with SendBlue (carry-forward).
- Working ngrok tunnel pointing at the local dev server (or accept signed-curl substitute per `[[v05-10-pass]]` pattern).
- Stale `DATABASE_URL` in shell: re-source `apps/fomo/.env.3b3.local` in every smoke terminal (`[[stale-database-url-shell-export]]`).
- v0.5.11 substrate populated: at least one canonical-HMAC PIL row in `memory_signals` from prior v0.5.11 smoke or synthetic seed. **LOAD-BEARING** — the live ranker has nothing to read if absent. The two `sender_suppressed` rows + three `sender_importance` rows from v0.5.11 smoke (carry-forward on `main`) satisfy this.

## 1. §1 baseline snapshot (run BEFORE booting dev server)

```bash
set -a && source apps/fomo/.env.3b3.local && set +a

# Capture baselines into /tmp for the post-smoke diff
psql "$DATABASE_URL" -P pager=off -c "
SELECT 'rank_results (total)' AS scope, COUNT(*)::text AS n FROM rank_results
UNION ALL SELECT 'rank_results last 24h', COUNT(*)::text FROM rank_results WHERE created_at > now() - interval '24 hours'
UNION ALL SELECT 'brevio.rank.pil_applied audits (expect 0)', COUNT(*)::text FROM audit_log WHERE action='brevio.rank.pil_applied'
UNION ALL SELECT 'brevio.signal.aggregated audits (v0.5.11 carry-forward)', COUNT(*)::text FROM audit_log WHERE action='brevio.signal.aggregated'
UNION ALL SELECT 'sender_importance rows', COUNT(*)::text FROM memory_signals WHERE kind='sender_importance'
UNION ALL SELECT 'sender_suppressed rows (canonical HMAC)', COUNT(*)::text FROM memory_signals WHERE kind='sender_suppressed' AND scope_key ~ '^[a-f0-9]{32}\$'
UNION ALL SELECT 'sender_suppressed rows (legacy message:id placeholder)', COUNT(*)::text FROM memory_signals WHERE kind='sender_suppressed' AND scope_key LIKE 'message:%'
UNION ALL SELECT 'alerts WHERE sender_email_hash IS NOT NULL', COUNT(*)::text FROM alerts WHERE sender_email_hash IS NOT NULL;
" | tee /tmp/v0.5.12-baseline.txt

# Record the smoke start timestamp
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.12-smoke-start-ts.txt
```

**After capture**, export `FOMO_V0_5_12_BASELINE_CONFIRMED=true` and re-run preflight.

## 2. Env setup

```
# v0.5.12 tunables
FOMO_PIL_LIVE_ENABLED=false     # default off; flipped on for Path C/D/E
FOMO_PIL_SCORE_CAP=0.15         # Q2.A; bounds [0.05, 0.25]
FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false  # Q4.A — OFF this phase

# Smoke gating
FOMO_V0_5_12_BASELINE_CONFIRMED=true
FOMO_V0_5_12_WINDOW_HOURS=24

# v0.5.11 substrate carry-forward (preserve to keep substrate write path alive)
FOMO_PIL_K_THRESHOLD=3
FOMO_PIL_SCORE_DELTA=0.1
FOMO_PIL_RECENCY_FULL_DAYS=90
FOMO_PIL_RECENCY_DECAY_DAYS=90
```

Carry-forward env unchanged from v0.5.11.

## 3. Preflight

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.12
```

Expected: PASS with WARNs only on PENDING runtime artifacts (`pil-live-context.ts` / extended `pil-context.ts` export, `pil-live.eval.ts`, `brevio.rank.pil_applied` audit kind). All ERRORs must be cleared.

## 4. Boot

```bash
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.12.log
```

Wait for `fomo.server.listening`. Verify boot log shows v0.5.11 substrate kinds in the `memory_signals.snapshot_at_boot` event (sender_importance / sender_suppressed canonical-HMAC rows should appear).

In a separate terminal, start ngrok if needed.

---

## 5. Path A — Kill switch OFF (C1 LOAD-BEARING)

**Goal:** prove the kill-switch contract — when `FOMO_PIL_LIVE_ENABLED=false`, live ranker behavior is bit-identical to v0.5.11. No two-call hybrid, no PIL prompt block, no audit.

### 5.1 Confirm kill switch off

```bash
echo "$FOMO_PIL_LIVE_ENABLED"  # expect: false (or unset)
```

If unset or `false`, proceed. If `true`, restart server with the env explicitly cleared.

### 5.2 Drive a rank

Wait for the polling worker to surface ≥1 fresh ranked alert in the smoke window. Verify via:

```bash
SMOKE_START_TS=$(cat /tmp/v0.5.12-smoke-start-ts.txt)
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, user_id, message_id, prompt_version, label, score, substring(reason, 1, 80) AS reason_head
FROM rank_results
WHERE user_id='founder' AND created_at > '$SMOKE_START_TS'
ORDER BY created_at DESC LIMIT 3;
"
```

### 5.3 Verify bit-identical contract

```bash
# Capture the rank_result_id of a fresh rank
RANK_ID=$(psql "$DATABASE_URL" -tA -c "
SELECT id FROM rank_results
WHERE user_id='founder' AND created_at > '$SMOKE_START_TS'
ORDER BY created_at DESC LIMIT 1;
")
echo "Path A rank_result_id: $RANK_ID"

# 5.3.1 — NO brevio.rank.pil_applied audit for this rank_result_id
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS audit_count
FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND (detail->>'rank_result_id')::bigint = $RANK_ID;
"
# Expect: audit_count = 0

# 5.3.2 — prompt_version = ranker-v0.2.0 (v0.5.11 baseline shape, no PIL block)
psql "$DATABASE_URL" -P pager=off -c "
SELECT prompt_version FROM rank_results WHERE id=$RANK_ID;
"
# Expect: 'ranker-v0.2.0'

# 5.3.3 — ONE cost_records entry per rank_result_id (single ranker call, not two)
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS call_count FROM cost_records
WHERE user_id='founder' AND created_at > '$SMOKE_START_TS' AND created_at <= (
  SELECT created_at FROM rank_results WHERE id=$RANK_ID
) + interval '5 seconds';
" || echo "  (cost_records may be cycle-aggregated; operator confirms via runtime cost log)"
```

**Operator records** the Path A rank_result_id + the kill-switch-off timestamp window in §16 of the SMOKE_REPORT.

---

## 6. Path B — Kill switch ON, NO matching PIL row (C5 inverse)

**Goal:** prove `brevio.rank.pil_applied` fires ONLY when PIL context is non-null.

### 6.1 Flip kill switch

```bash
# Restart server with FOMO_PIL_LIVE_ENABLED=true
# (kill existing pnpm dev, re-source env, set var, re-launch)
export FOMO_PIL_LIVE_ENABLED=true
pnpm --filter @brevio/fomo dev 2>&1 | tee -a /tmp/fomo-v0.5.12.log &
```

### 6.2 Drive a rank for a sender NOT in the v0.5.11 PIL substrate

Wait for the polling worker to produce an alert whose sender has NO canonical-HMAC PIL row in `memory_signals`. Verify after-the-fact:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT a.id, a.sender_email_hash,
       EXISTS (
         SELECT 1 FROM memory_signals m
         WHERE m.user_id='founder'
           AND m.scope_key = a.sender_email_hash
           AND m.kind IN ('sender_importance', 'sender_suppressed')
       ) AS has_canonical_pil
FROM alerts a
WHERE a.user_id='founder' AND a.created_at > '$SMOKE_START_TS' AND a.sender_email_hash IS NOT NULL
ORDER BY a.created_at DESC LIMIT 5;
"
```

Find an alert with `has_canonical_pil = false`.

### 6.3 Verify no audit fires

```bash
# rank_result for that alert
psql "$DATABASE_URL" -P pager=off -c "
SELECT r.id FROM rank_results r
JOIN alerts a ON a.rank_result_id = r.id
WHERE a.id='<alert_id_from_6.2>' LIMIT 1;
"
# Use that id:
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND (detail->>'rank_result_id')::bigint = <rank_id_from_above>;
"
# Expect: 0
```

---

## 7. Path C — Kill switch ON, canonical-HMAC PIL row exists (C2 + C4 + C5 LOAD-BEARING)

**Goal:** prove the two-call hybrid runs correctly, score cap is enforced, audit fires with all 9 fields.

### 7.1 Choose a canonical-HMAC PIL row

```bash
# Pick a sender_importance or sender_suppressed row with HMAC scope_key
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, user_id, kind, scope_key, jsonb_pretty(detail) AS detail
FROM memory_signals
WHERE user_id='founder' AND kind IN ('sender_importance', 'sender_suppressed')
  AND scope_key ~ '^[a-f0-9]{32}\$'
ORDER BY updated_at DESC LIMIT 5;
"
```

Note the scope_key (e.g. `7de42631e2716f4d8f77a597fb38aa7d`).

### 7.2 Drive a rank for that scope

Either:
- **Natural-rank**: wait for Gmail polling to surface an email from a sender whose HMAC matches.
- **Synthetic-seed substitute**: seed an alert with `sender_email_hash` equal to the chosen scope_key (same pattern as v0.5.11 smoke `_smoke-v0.5.11-seed-alert.mjs`). After seeding, kick the polling/rank cycle.

### 7.3 Verify the two-call hybrid + audit chain

```bash
SMOKE_START_TS=$(cat /tmp/v0.5.12-smoke-start-ts.txt)
SCOPE_KEY='<the canonical HMAC chosen in 7.1>'

# 7.3.1 — brevio.rank.pil_applied audit fired with all 9 fields
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id='founder'
  AND occurred_at > '$SMOKE_START_TS'
  AND detail->>'scope_key_hash' = '$SCOPE_KEY'
ORDER BY occurred_at DESC LIMIT 1;
"
# Expect: all 9 fields populated:
#   rank_result_id, pil_signal_kinds_present, score_before_pil_cap,
#   score_after_pil_cap, pil_score_delta, pil_score_delta_was_capped,
#   model_mentioned_pil_in_reason, source_surface, scope_key_hash

# 7.3.2 — rank_results.prompt_version = ranker-v0.3.0 (PIL block included)
RANK_ID=$(psql "$DATABASE_URL" -tA -c "
SELECT (detail->>'rank_result_id')::bigint
FROM audit_log
WHERE action='brevio.rank.pil_applied' AND detail->>'scope_key_hash'='$SCOPE_KEY'
ORDER BY occurred_at DESC LIMIT 1;
")
psql "$DATABASE_URL" -P pager=off -c "
SELECT prompt_version, label, score FROM rank_results WHERE id=$RANK_ID;
"
# Expect: prompt_version='ranker-v0.3.0'

# 7.3.3 — score_after_pil_cap = rank_results.score (final value matches)
psql "$DATABASE_URL" -P pager=off -c "
SELECT
  r.score AS rank_score,
  (a.detail->>'score_after_pil_cap')::float AS audit_after_cap,
  (a.detail->>'score_before_pil_cap')::float AS audit_before_cap,
  (a.detail->>'pil_score_delta')::float AS audit_delta,
  (a.detail->>'pil_score_delta_was_capped')::boolean AS was_capped
FROM rank_results r
JOIN audit_log a ON (a.detail->>'rank_result_id')::bigint = r.id
WHERE r.id=$RANK_ID AND a.action='brevio.rank.pil_applied';
"
# Expect: rank_score = audit_after_cap (within float epsilon)

# 7.3.4 — privacy canary on this row's detail
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail::text FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND (detail->>'rank_result_id')::bigint = $RANK_ID;
" | grep -iEc 'gmail\.com|icloud\.com|hotmail\.com|@yahoo\.com|noreply@|unsubscribe|Subject:|From:|To:|because the user|the user has marked' | xargs -I {} echo "canary hits: {}"
# Expect: canary hits: 0
```

---

## 8. Path D — Legacy `scope_key='message:<id>'` row only (C3 LOAD-BEARING)

**Goal:** prove the canonical-HMAC read-side filter ignores legacy placeholder rows.

### 8.1 Verify a placeholder row exists

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, kind, scope_key, jsonb_pretty(detail) AS detail
FROM memory_signals
WHERE kind='sender_suppressed' AND scope_key LIKE 'message:%'
ORDER BY updated_at DESC LIMIT 3;
"
```

At least one row should be present (v0.5.10/v0.5.11 carry-forward from `applyIgnoreSender`).

### 8.2 Construct a synthetic alert with matching sender BUT no canonical HMAC row

The placeholder row binds to a `message_id`, not a sender hash. For the read-side filter to be tested, seed a synthetic alert where the sender_email_hash matches NO canonical row. Then drive a rank.

```bash
# Pick a sender hash that has NO canonical-HMAC PIL row
psql "$DATABASE_URL" -P pager=off -c "
SELECT 'no canonical row' AS check, NOT EXISTS (
  SELECT 1 FROM memory_signals
  WHERE user_id='founder' AND scope_key='<your synthetic hash>'
    AND kind IN ('sender_importance', 'sender_suppressed')
    AND scope_key ~ '^[a-f0-9]{32}\$'
) AS no_canonical_for_hash;
"
```

If `no_canonical_for_hash = true`, the synthetic test setup is valid.

### 8.3 Verify NO audit fires for this rank

```bash
RANK_ID='<from the synthetic alert>'
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS pil_applied_count FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND (detail->>'rank_result_id')::bigint = $RANK_ID;
"
# Expect: 0
```

**This is the LOAD-BEARING C3 proof.** BB6 fixture in `pil-live.eval.ts` is the in-vitro mirror.

---

## 9. Path E — Cross-user contamination (C7 LOAD-BEARING)

**Goal:** prove the per-user HMAC scope means user A's signal cannot be read by user B's rank call.

```bash
# Seed a canonical-HMAC PIL row for a synthetic userA on a synthetic sender
psql "$DATABASE_URL" -P pager=off -c "
INSERT INTO memory_signals (user_id, kind, scope_key, source, confidence, detail, created_at, updated_at)
VALUES ('userA-smoke-v0512', 'sender_importance', '<userA HMAC for sender X>',
        'feedback_derived', 0.6,
        '{\"score\": 0.3, \"n_positive_events\": 3, \"n_negative_events\": 0, \"last_updated\": \"now\", \"source_surface\": \"email_alert\", \"source_feedback_event_ids\": []}',
        now(), now());
"

# Compute the SAME sender's hash under FOUNDER's id (different by HMAC construction)
# Verify NO founder row exists at userA's scope_key
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, scope_key FROM memory_signals
WHERE scope_key='<userA HMAC for sender X>';
"
# Expect: only the userA row
```

Now drive a founder rank that targets the SAME sender. Because the founder's HMAC for sender X is DIFFERENT from userA's HMAC for sender X, the founder's `buildLivePilContext` call should return `null` (no canonical row at founder's scope).

Verify NO `brevio.rank.pil_applied` audit fires for the founder's rank of that sender. The BB4 fixture in `pil-live.eval.ts` adversarially simulates a hash collision by inserting a row at the founder's actual scope under userA's user_id; the test asserts the read-side filter joins on user_id AND ignores it.

---

## 10. Path F — Offline eval (`eval:pil-live`)

```bash
pnpm --filter @brevio/fomo run -s eval:pil-live 2>&1 | tee /tmp/v0.5.12-pil-live-eval.log
```

Expected output: per-fixture lines reporting baseline vs shadow vs live scores + PASS/FAIL.

**11 carry-forward shadow fixtures (F1–F11)** + **8 LOAD-BEARING becomes-blind fixtures (BB1–BB8)**:

| ID | Setup | Required result |
|---|---|---|
| BB1 | sender_suppressed=true 3d old + email "URGENT: from CEO" | label=important, score ≥ 0.7, rank.reason mentions both prior + override |
| BB2 | sender_importance.score=-0.3 (3 false_positives, no suppression) + normal-strength email | Live score within ±FOMO_PIL_SCORE_CAP of baseline; not auto-dropped |
| BB3 | sender_importance.score=+0.3 but 200d old + weak signals | Decay → live ≈ baseline (|Δ| ≤ 0.05) |
| BB4 | Direct-DB-insert simulating cross-user hash collision | `buildLivePilContext('userB', H_A)` returns user B row only |
| BB5 | Single false_positive (1 event, score=-0.1) + mid-strength email | Live within noise floor (|Δ| ≤ 0.05) of baseline |
| BB6 | Legacy `message:<id>` row only; NO canonical HMAC row | `pil_context: null`; bit-identical to baseline; NO audit |
| BB7 | `FOMO_PIL_LIVE_ENABLED=false` + PIL rows present + strong-signal email | `pil_context: null`; baseline-only call; no audit |
| BB8 | sender_importance.score=+1.0 (theoretical max) | `pil_score_delta_was_capped=true`; |delta| ≤ `FOMO_PIL_SCORE_CAP` |

The eval calls OpenAI (live model). Expect ≥60s runtime for ≥19 fixtures across baseline + shadow + live calls. Final line: `VERDICT: PASS`.

**If any of BB1–BB8 fails, v0.5.12 does NOT ship.**

---

## 11. Live ranker invariant — kill-switch-off bit-identical (composes with §5)

```bash
# Verify production ranker call site BOTH paths
grep -rn "FOMO_PIL_LIVE_ENABLED\|buildLivePilContext" apps/fomo/src/workers/ apps/fomo/src/ranker/index.ts apps/fomo/src/ranker/rank-email.ts 2>/dev/null
# Expect: ≥1 conditional gate referencing FOMO_PIL_LIVE_ENABLED before any buildLivePilContext call

# Verify rank_results schema unchanged
psql "$DATABASE_URL" -P pager=off -c "\d rank_results"
# Expect: identical column list to v0.5.11 baseline (no new pil_* columns)
```

---

## 12. Carry-forward — v0.5.9 + v0.5.10 + v0.5.11 still PASS

```bash
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.9  2>&1 | tail -10
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.10 2>&1 | tail -10
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.11 2>&1 | tail -10
```

Expected: all three still PASS or match documented benign shapes from prior reports. **v0.5.11 substrate UNTOUCHED is the load-bearing C12 carry-forward.**

---

## 13. Run smoke-evidence:v0.5.12

```bash
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.12 2>&1 | tee /tmp/v0.5.12-evidence.log
```

Expected: `VERDICT: PASS` (with operator-confirmed PENDINGs for items the script cannot observe directly: BB1/BB3/BB5/BB8 eval results, Path D synthetic test, kill-switch timeline).

---

## 14. Clean stop

```bash
# Optional cleanup of synthetic smoke rows
# psql "$DATABASE_URL" -c "DELETE FROM memory_signals WHERE user_id='userA-smoke-v0512';"
# psql "$DATABASE_URL" -c "DELETE FROM alerts WHERE alert_id LIKE 'smoke-v0512-%';"

# Flip kill switch back OFF (production safety)
export FOMO_PIL_LIVE_ENABLED=false

# Ctrl-C the dev server. Confirm clean shutdown.
# Ctrl-C ngrok if used.
```

**Founder discretion:** leave `FOMO_PIL_LIVE_ENABLED=true` in production if smoke PASS is convincing, OR keep it off and re-enable later. The kill switch contract guarantees off=bit-identical-v0.5.11; flipping it later is a single env change.

---

## 15. Fill in the report

Copy `docs/SMOKE_REPORT_TEMPLATE_v0.5.12.md` → `docs/SMOKE_REPORT_v0.5.12.md`. Fill in all 14 PASS criteria + 8 BB fixture results + operator-confirmed evidence + privacy canary diff + sample JSON for new audit rows. Commit on the same branch.

---

## 16. Operator notes / honest substitutions

If any path was substituted (e.g. natural-rank waiting blocked by Gmail inbox state; used synthetic-seed substitute), document the substitution here and in the report.

If the eval reports model-nondeterministic flakiness on shift magnitude (but DIRECTION is consistent), that's acceptable per Q4.A (eval asserts direction not magnitude). Document flaky fixtures by ID; investigate if >2/19 are flaky.

If Path A (kill switch off) and Path C (kill switch on) ran in the same smoke window, document the kill-switch timeline precisely so C1 audit-row-count consistency check is interpretable:

```
Kill-switch OFF window:  <ts_start> → <ts_flip>
Kill-switch ON window:   <ts_flip>  → <ts_end>
Rank_result_ids observed in OFF window: [...]
Rank_result_ids observed in ON window:  [...]
```

Per `[[real-or-absent-no-half-wired]]`: if BB1 fails (suppressed sender does NOT surface on URGENT email), v0.5.12 must NOT ship. Brevio going binary-blind on a sender is a product failure mode the founder explicitly forbade.

---

## 17. Aftercare

- v0.5.7 HMR template unchanged (no renderer edits this phase).
- v0.5.9 substrate untouched: `BREVIO_FEEDBACK_SURFACES`=13, ACTIVE=`['email_alert']`.
- v0.5.10 reply-parser intent set unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents.
- v0.5.11 PIL substrate write path untouched: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire on natural-reply feedback.
- 3E.1 invariant: no LLM in body composition; only `rank.reason` is model-generated. v0.5.12 changes `rank.reason` content (model may mention PIL prior) but does NOT add an LLM call to `renderFounderText`.
- No new active surface beyond `email_alert`.
- No new memory_signal kinds.
- Read-side filter enforced: scope_key ~ `^[a-f0-9]{32}$`; legacy `message:<id>` rows ignored.
- Kill switch contract: `FOMO_PIL_LIVE_ENABLED=false` → bit-identical v0.5.11.
- Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default.
