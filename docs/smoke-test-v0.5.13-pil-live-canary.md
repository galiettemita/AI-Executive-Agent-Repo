# Phase v0.5.13 Smoke Test Runbook — Founder-only PIL Live Canary / Controlled Activation

> Founder-only canary. **No friend involvement** (three-friend cap per `[[three-friend-beta-cap]]`).
> v0.5.13 is the first phase where the v0.5.12 live PIL ranker path actually runs against real founder traffic in production, behind a per-user allowlist with reversible env-only rollback.
>
> **Phase under Core Dimension Check discipline.** See [`memory/project_v05-13-scope.md`](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-13-scope.md).
>
> **Founder correction #4: 24h observation window is NOT a merge blocker.** Canary proof can be shorter. This runbook produces the LOAD-BEARING evidence in a bounded window; the report MAY recommend additional observation time but the PR does NOT block on it.

---

## 0. Prerequisites

- v0.5.12 merged on `main` with VERDICT: PASS (merge `9d7a2e1b`).
- `BREVIO_SENDER_HASH_KEY` set (carry-forward; LOAD-BEARING for read-time HMAC).
- `OPENAI_API_KEY` set (carry-forward; the v0.5.12 two-call hybrid runs TWO ranker calls per PIL-influenced rank).
- Founder phone registered with SendBlue (carry-forward).
- `FOMO_FOUNDER_USER_ID` set to the actual founder user_id (LOAD-BEARING; the allowlist must contain this exact string).
- Stale `DATABASE_URL` in shell: re-source `apps/fomo/.env.3b3.local` in every smoke terminal (`[[stale-database-url-shell-export]]`).
- v0.5.12 substrate present: at least one canonical-HMAC PIL row in `memory_signals` for founder. **LOAD-BEARING** — the live ranker has nothing to read if absent. The carry-forward `sender_importance` + `sender_suppressed` rows from v0.5.12 smoke satisfy this.

## 1. §1 baseline snapshot (run BEFORE booting dev server)

```bash
set -a && source apps/fomo/.env.3b3.local && set +a

# Capture baselines into /tmp for the post-canary diff
psql "$DATABASE_URL" -P pager=off -c "
SELECT 'rank_results (total)' AS scope, COUNT(*)::text AS n FROM rank_results
UNION ALL SELECT 'rank_results last 24h', COUNT(*)::text FROM rank_results WHERE created_at > now() - interval '24 hours'
UNION ALL SELECT 'brevio.rank.pil_applied audits (total)', COUNT(*)::text FROM audit_log WHERE action='brevio.rank.pil_applied'
UNION ALL SELECT 'brevio.rank.pil_applied audits last 24h', COUNT(*)::text FROM audit_log WHERE action='brevio.rank.pil_applied' AND occurred_at > now() - interval '24 hours'
UNION ALL SELECT 'brevio.signal.aggregated audits (v0.5.11 carry-forward)', COUNT(*)::text FROM audit_log WHERE action='brevio.signal.aggregated'
UNION ALL SELECT 'sender_importance rows (founder)', COUNT(*)::text FROM memory_signals WHERE user_id='founder' AND kind='sender_importance'
UNION ALL SELECT 'sender_suppressed rows (founder, canonical HMAC)', COUNT(*)::text FROM memory_signals WHERE user_id='founder' AND kind='sender_suppressed' AND scope_key ~ '^[a-f0-9]{32}\$'
UNION ALL SELECT 'sender_suppressed rows (founder, legacy message:id placeholder)', COUNT(*)::text FROM memory_signals WHERE user_id='founder' AND kind='sender_suppressed' AND scope_key LIKE 'message:%'
UNION ALL SELECT 'alerts WHERE sender_email_hash IS NOT NULL', COUNT(*)::text FROM alerts WHERE sender_email_hash IS NOT NULL;
" | tee /tmp/v0.5.13-baseline.txt

# Record the smoke start timestamp (used to scope the canary window)
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.13-smoke-start-ts.txt
```

**After capture**, export `FOMO_V0_5_13_BASELINE_CONFIRMED=true` and re-run preflight.

## 2. Env setup

```
# v0.5.13 new
FOMO_PIL_LIVE_USER_ALLOWLIST=                # leave EMPTY for Phase A; set to <founder_user_id> for Phase C
FOMO_V0_5_13_BASELINE_CONFIRMED=true
FOMO_V0_5_13_WINDOW_HOURS=24                 # scope window for smoke-evidence (default 24)

# v0.5.12 carry-forward
FOMO_PIL_LIVE_ENABLED=false                  # default off; flipped on for Phase C/D
FOMO_PIL_SCORE_CAP=0.15                      # carry-forward Q2.A
FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false      # hard boundary: OFF this phase

# v0.5.11 substrate carry-forward
FOMO_PIL_K_THRESHOLD=3
FOMO_PIL_SCORE_DELTA=0.1
FOMO_PIL_RECENCY_FULL_DAYS=90
FOMO_PIL_RECENCY_DECAY_DAYS=90
```

Carry-forward env unchanged from v0.5.12.

**The runbook walks 6 sub-phases (A–F) cycling through the 4-case truth table + soft/hard rollback.** Each sub-phase has its own boot command + env. The kill-switch + allowlist timeline is the LOAD-BEARING evidence; record every flip ISO timestamp into `/tmp/v0.5.13-<flip>-ts.txt`.

## 3. Preflight

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.13
```

Expected: PASS with WARNs only on PENDING runtime artifacts (`KillSwitches.pil_live_user_allowlist` field; worker-level allowlist gate). All ERRORs must be cleared.

If `FOMO_PIL_LIVE_ENABLED=true` AND `FOMO_PIL_LIVE_USER_ALLOWLIST` is empty/unset → preflight **ERRORS** (founder correction #2). This is intentional. Either set the allowlist or keep the global OFF for the boot under test.

## 4. Boot

```bash
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.13.log
```

Wait for `fomo.server.listening`. Verify boot log shows:
- v0.5.11 substrate kinds in `memory_signals.snapshot_at_boot`
- `pil_live_enabled: false` in the resolved kill switches (Phase A start)

In a separate terminal, start ngrok if needed.

---

## 5. Phase A — Kill switch OFF, allowlist any (C1 case (a) + carry-forward of v0.5.12 C1)

**Goal:** prove the kill-switch-off contract still holds (bit-identical v0.5.11) regardless of the allowlist state.

### 5.1 Confirm boot state

```bash
echo "$FOMO_PIL_LIVE_ENABLED"        # expect: false (or unset)
echo "$FOMO_PIL_LIVE_USER_ALLOWLIST" # any value or empty — doesn't matter
```

### 5.2 Drive a founder rank

Wait for the polling worker to surface ≥1 fresh ranked alert for the founder user_id. Alternatively, send yourself a test email.

### 5.3 Verify Phase A contract

```bash
SMOKE_START_TS=$(cat /tmp/v0.5.13-smoke-start-ts.txt)
FOUNDER=$(echo "$FOMO_FOUNDER_USER_ID")
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, prompt_version, label, ROUND(score::numeric, 3) AS score, created_at
FROM rank_results
WHERE user_id='$FOUNDER' AND created_at > '$SMOKE_START_TS'
ORDER BY created_at DESC LIMIT 3;
"
# Expect: prompt_version='ranker-v0.2.0'

# 0 PIL audits for the founder in Phase A
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id='$FOUNDER'
  AND occurred_at > '$SMOKE_START_TS';
"
# Expect: 0
```

**Record** Phase A timestamps + rank_result_id(s) in §16 of the SMOKE_REPORT.

---

## 6. Phase B — Kill switch ON, allowlist EMPTY (C1 case (b))

**Goal:** prove the LOAD-BEARING preflight ERROR + runtime fail-closed contract.

### 6.1 Try to boot with global=true + empty allowlist

```bash
export FOMO_PIL_LIVE_ENABLED=true
export FOMO_PIL_LIVE_USER_ALLOWLIST=
pnpm --filter @brevio/fomo run preflight:v0.5.13
```

Expected: **preflight exits non-zero** with `[ERROR] FOMO_PIL_LIVE_USER_ALLOWLIST: LOAD-BEARING v0.5.13 founder rule: FOMO_PIL_LIVE_ENABLED=true requires FOMO_PIL_LIVE_USER_ALLOWLIST to be non-empty ...`.

### 6.2 (Optional) Bypass preflight, observe runtime fail-closed

If the founder wants to also exercise the runtime fail-closed path:

```bash
# Bypass preflight; boot anyway with global=true + empty allowlist
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.13-phaseB.log &
```

Expected: server boots; boot log emits a single `fomo.pil_live.allowlist_empty` WARN; subsequent ranks all run the baseline-only path (no PIL audits). Drive one founder rank to confirm.

Document the WARN log line + the absence of PIL audits in §16.

Stop this server before proceeding.

---

## 7. Phase C — Kill switch ON, allowlist=founder (C1 case (c) + C2 LOAD-BEARING)

**Goal:** prove the founder-only canary works end-to-end.

### 7.1 Boot with global=true + allowlist=founder

```bash
# Re-source env, set FOMO_PIL_LIVE_USER_ALLOWLIST to the actual founder user_id
set -a && source apps/fomo/.env.3b3.local && set +a
export FOMO_PIL_LIVE_ENABLED=true
export FOMO_PIL_LIVE_USER_ALLOWLIST="$FOMO_FOUNDER_USER_ID"
echo "Allowlist set to: '$FOMO_PIL_LIVE_USER_ALLOWLIST'"

# Record the canary-open timestamp (LOAD-BEARING for smoke-evidence scoping)
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.13-canary-open-ts.txt

pnpm --filter @brevio/fomo dev 2>&1 | tee -a /tmp/fomo-v0.5.13.log
```

Wait for `fomo.server.listening`. Boot log should show:
- `pil_live_enabled: true` in resolved kill switches
- `pil_live_user_allowlist: ["<founder_user_id>"]` (or equivalent)

### 7.2 Drive ≥1 founder rank with a matching canonical PIL row

Either:
- **Natural-rank**: wait for Gmail polling to surface an email from a sender whose HMAC matches a canonical-HMAC `sender_importance` or `sender_suppressed` row for founder.
- **Synthetic-seed substitute (runbook §7.2 sanctioned)**: extend the v0.5.12 `_smoke-v0.5.12-seed-path-c.ts` pattern to v0.5.13 — same target scope (`d1a6c6c65e9c5d7a363198a0af91c1b7` BB1-equivalent works), but the seed script confirms it runs through the worker-level allowlist gate.

### 7.3 Verify Phase C audit + PIL block + scope_key match

```bash
CANARY_OPEN_TS=$(cat /tmp/v0.5.13-canary-open-ts.txt)
FOUNDER=$(echo "$FOMO_FOUNDER_USER_ID")

# Fresh PIL audit row
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, actor_user_id, occurred_at, jsonb_pretty(detail) AS detail FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id='$FOUNDER'
  AND occurred_at >= '$CANARY_OPEN_TS'
ORDER BY occurred_at DESC LIMIT 3;
"
# Expect: all 9 fields populated

# Fresh rank_results row uses PIL-block prompt
RANK_ID=$(psql "$DATABASE_URL" -tA -c "
SELECT (detail->>'rank_result_id')::bigint FROM audit_log
WHERE action='brevio.rank.pil_applied' AND actor_user_id='$FOUNDER' AND occurred_at >= '$CANARY_OPEN_TS'
ORDER BY occurred_at DESC LIMIT 1;
")
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, user_id, prompt_version, label, score FROM rank_results WHERE id=$RANK_ID;
"
# Expect: prompt_version='ranker-v0.3.0', user_id=<founder>
```

---

## 8. Phase D — Cross-user check (C3 + C6 LOAD-BEARING)

**Goal:** prove the allowlist gate keeps non-founder users on the baseline-only path.

If any non-founder users are being polled during the canary window (carry-forward: existing friend-beta users), verify:

```bash
CANARY_OPEN_TS=$(cat /tmp/v0.5.13-canary-open-ts.txt)
FOUNDER=$(echo "$FOMO_FOUNDER_USER_ID")

# C3 LOAD-BEARING: 0 non-founder PIL audits in canary window
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id <> '$FOUNDER'
  AND occurred_at >= '$CANARY_OPEN_TS';
"
# Expect: 0

# C6 LOAD-BEARING: 0 founder PIL audits cite a scope_key without a founder memory_signal row
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log a
WHERE a.action='brevio.rank.pil_applied'
  AND a.actor_user_id='$FOUNDER'
  AND a.occurred_at >= '$CANARY_OPEN_TS'
  AND NOT EXISTS (
    SELECT 1 FROM memory_signals m
    WHERE m.user_id='$FOUNDER'
      AND m.scope_key = a.detail->>'scope_key_hash'
      AND m.kind IN ('sender_importance','sender_suppressed')
  );
"
# Expect: 0
```

---

## 9. Phase E — Soft rollback rehearsal (C7 LOAD-BEARING)

**Goal:** prove the soft rollback (clear allowlist) reverts to baseline-only behavior immediately on server restart.

### 9.1 Flip soft rollback

```bash
# Stop dev server
lsof -ti:8080 | xargs kill 2>/dev/null; sleep 2

# Clear allowlist (keep global=true to isolate the rollback to JUST the allowlist)
export FOMO_PIL_LIVE_USER_ALLOWLIST=
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.13-soft-rollback-ts.txt

# Re-boot — should fail preflight (founder correction #2). Confirm, then bypass and observe runtime fail-closed.
pnpm --filter @brevio/fomo run preflight:v0.5.13
# Expect: ERROR. Document the exact error message.

# Then bypass preflight + boot (the production code must ALSO fail-closed when global=true + allowlist empty).
pnpm --filter @brevio/fomo dev 2>&1 | tee -a /tmp/fomo-v0.5.13.log
```

### 9.2 Drive ≥1 founder rank after soft rollback

Natural Gmail or synthetic-seed. Then verify:

```bash
SOFT_TS=$(cat /tmp/v0.5.13-soft-rollback-ts.txt)
FOUNDER=$(echo "$FOMO_FOUNDER_USER_ID")

# 0 new PIL audits since soft rollback flip
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id='$FOUNDER'
  AND occurred_at > '$SOFT_TS';
"
# Expect: 0

# Fresh ranks have prompt_version='ranker-v0.2.0'
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, prompt_version FROM rank_results
WHERE user_id='$FOUNDER' AND created_at > '$SOFT_TS'
ORDER BY created_at DESC LIMIT 3;
"
# Expect: all prompt_version='ranker-v0.2.0'
```

---

## 10. Phase F — Hard rollback rehearsal (C8 LOAD-BEARING)

**Goal:** prove the hard rollback (global kill switch off) reverts to v0.5.11 bit-identical behavior even faster.

### 10.1 Flip hard rollback

```bash
# Stop dev server
lsof -ti:8080 | xargs kill 2>/dev/null; sleep 2

# Hard rollback: global=false. Allowlist no longer matters.
export FOMO_PIL_LIVE_ENABLED=false
date -u +'%Y-%m-%dT%H:%M:%SZ' | tee /tmp/v0.5.13-hard-rollback-ts.txt

# Re-boot — preflight should now PASS (global off makes allowlist moot)
pnpm --filter @brevio/fomo run preflight:v0.5.13
# Expect: PASS

pnpm --filter @brevio/fomo dev 2>&1 | tee -a /tmp/fomo-v0.5.13.log
```

### 10.2 Drive ≥1 founder rank after hard rollback

Then verify:

```bash
HARD_TS=$(cat /tmp/v0.5.13-hard-rollback-ts.txt)
FOUNDER=$(echo "$FOMO_FOUNDER_USER_ID")

psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM audit_log
WHERE action='brevio.rank.pil_applied'
  AND actor_user_id='$FOUNDER'
  AND occurred_at > '$HARD_TS';
"
# Expect: 0

psql "$DATABASE_URL" -P pager=off -c "
SELECT id, prompt_version FROM rank_results
WHERE user_id='$FOUNDER' AND created_at > '$HARD_TS'
ORDER BY created_at DESC LIMIT 3;
"
# Expect: all prompt_version='ranker-v0.2.0'
```

---

## 11. Path G — v0.5.12 BB1-BB8 regression (C12)

```bash
pnpm --filter @brevio/fomo run -s eval:pil-live 2>&1 | tee /tmp/v0.5.13-pil-live-eval.log
```

**Run 3 times.** Expected: VERDICT PASS on all 3 runs, all 19 fixtures green every run. Any flake or fail on BB1-BB8 is a v0.5.13 regression of the v0.5.12 contract.

---

## 12. Carry-forward — v0.5.9 + v0.5.10 + v0.5.12 still PASS (C11)

```bash
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.9  2>&1 | tail -10
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.10 2>&1 | tail -10
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.11 2>&1 | tail -10
pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.12 2>&1 | tail -10
```

Expected: all four still PASS or match documented benign shapes from prior reports. **v0.5.11 substrate UNTOUCHED is the C10 carry-forward.**

---

## 13. Run smoke-evidence:v0.5.13

Set the operator-confirmed timestamp env vars before running:

```bash
export FOMO_V0_5_13_CANARY_OPEN_TS=$(cat /tmp/v0.5.13-canary-open-ts.txt)
export FOMO_V0_5_13_SOFT_ROLLBACK_TS=$(cat /tmp/v0.5.13-soft-rollback-ts.txt)
export FOMO_V0_5_13_HARD_ROLLBACK_TS=$(cat /tmp/v0.5.13-hard-rollback-ts.txt)

pnpm --filter @brevio/fomo run -s smoke-evidence:v0.5.13 2>&1 | tee /tmp/v0.5.13-evidence.log
```

Expected: `VERDICT: PASS` (with operator-confirmed PENDINGs for C1 unit-test confirmation, C11 carry-forward script runs, C12 eval re-run).

---

## 14. Clean stop (final production state)

```bash
# Stop dev server
lsof -ti:8080 | xargs kill 2>/dev/null

# FINAL PRODUCTION STATE (founder correction #3): FOMO_PIL_LIVE_ENABLED=false
# unless founder explicitly approves keeping ON after reviewing the canary report.
export FOMO_PIL_LIVE_ENABLED=false
unset FOMO_PIL_LIVE_USER_ALLOWLIST   # optional; doesn't matter when global=false

# Confirm clean shutdown.
# Ctrl-C ngrok if used.
```

**Founder discretion:** the report includes a §17 explicit decision. The default is OFF unless the canary results convince the founder to keep ON.

---

## 15. Fill in the report

Copy `docs/SMOKE_REPORT_TEMPLATE_v0.5.13.md` → `docs/SMOKE_REPORT_v0.5.13.md`. Fill in all 12 PASS criteria + Phase A/B/C/D/E/F timeline + operator-confirmed evidence + sample JSON for new PIL audit rows + soft+hard rollback evidence + §17 founder decision on production state. Commit on the same branch.

---

## 16. Operator notes / honest substitutions

If any path was substituted (e.g. natural-rank waiting blocked by Gmail inbox state; used synthetic-seed substitute for Phase C), document the substitution here and in the report.

If Phase D's cross-user check finds 0 non-founder users polled during the canary window (perhaps friend-beta accounts have `stop_active=true`), document that the C3 check is operator-confirmed via the architectural mechanism (HMAC user_id + allowlist gate) plus unit tests rather than an in-vivo observation.

The kill-switch + allowlist timeline is the LOAD-BEARING evidence for C7 and C8. Document precisely:

```
Phase A (global=off, list=any):     <ts_start>  → <ts_phaseB_start>
Phase B (global=on, list=empty):    <ts_phaseB_start>  → <ts_phaseC_start>
Phase C (global=on, list=founder):  <ts_phaseC_start (== CANARY_OPEN_TS)>  → <ts_softFlip>
Phase E (global=on, list=empty):    <ts_softFlip (== SOFT_ROLLBACK_TS)>  → <ts_hardFlip>
Phase F (global=off, list=any):     <ts_hardFlip (== HARD_ROLLBACK_TS)>  → <ts_end>

Rank_result_ids observed per phase:
  Phase A: [...]
  Phase C: [...]
  Phase E (post-soft):  [...]
  Phase F (post-hard):  [...]

brevio.rank.pil_applied audits by phase:
  Phase A: 0  ✓ (expected — global off)
  Phase B: 0  ✓ (expected — preflight ERRORS; runtime fail-closed if bypassed)
  Phase C: ≥ 1  ✓ (LOAD-BEARING C2)
  Phase E (post-soft): 0  ✓ (LOAD-BEARING C7)
  Phase F (post-hard): 0  ✓ (LOAD-BEARING C8)
```

Per `[[real-or-absent-no-half-wired]]`: if any LOAD-BEARING criterion (C2, C3, C6, C7, C8) fails, v0.5.13 must NOT ship. The point of the canary is reversible proof, not feature shipping.

Per `[[no-gate-creep-on-extra-smokes]]`: ONE canary cycle is sufficient PASS evidence when regression tests + smoke evidence + carry-forward + no-leak scan all check out. Don't retroactively demand "let's do another canary day to be safe."

---

## 17. Aftercare

- v0.5.7 HMR template unchanged (no renderer edits this phase).
- v0.5.9 substrate untouched: `BREVIO_FEEDBACK_SURFACES`=13, ACTIVE=`['email_alert']`.
- v0.5.10 reply-parser intent set unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents.
- v0.5.11 PIL substrate write path untouched: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire on natural-reply feedback.
- v0.5.12 live PIL ranker path untouched: `buildLivePilContext` + `rankEmailWithLivePil` + `brevio.rank.pil_applied` audit unchanged. v0.5.13 adds ONLY the worker-level allowlist gate in front of `buildLivePilContext`.
- 3E.1 invariant: no LLM in body composition; only `rank.reason` is model-generated. v0.5.13 does NOT change this.
- No new active surface beyond `email_alert`.
- No new memory_signal kinds beyond v0.5.11's two.
- Read-side filter enforced: scope_key ~ `^[a-f0-9]{32}$`; legacy `message:<id>` rows ignored; v0.5.12 `k_threshold` n_events floor still applied.
- Kill switch contract: `FOMO_PIL_LIVE_ENABLED=false` → bit-identical v0.5.11.
- Allowlist contract: when global=true + allowlist=`<founder>`, only the founder user_id gets the two-call hybrid; everyone else baseline-only.
- Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default.
- **Final production state: `FOMO_PIL_LIVE_ENABLED=false`** unless founder explicitly approves keeping ON in §17 of the report.
