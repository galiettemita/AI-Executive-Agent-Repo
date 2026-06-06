# Phase v0.5.8 Smoke Test — Gmail INBOX Event Reliability Hardening

> Founder-only smoke. **First phase scoped under the new Core Dimension Check discipline** (per [`docs/brevio-core-agent-dimensions.md`](brevio-core-agent-dimensions.md) + [`docs/brevio-product-philosophy.md`](brevio-product-philosophy.md)). It is a **hardening** phase, NOT a product surface change.
>
> No friend involved. Three-friend cap holds.
>
> **Path A is the default** per the locked scope ([memory: v05-8-scope](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-8-scope.md) §"Smoke runbook Path A default"). The load-bearing proof is a Gmail-to-self synthetic important email that v0.5.7 **never** surfaces but v0.5.8 surfaces within ≤3 poll cycles. External-email regression is secondary.

---

## §0 — What changes in v0.5.8 (so you know what to look for)

| Before (v0.5.7) | After (v0.5.8) |
|---|---|
| Gmail history.list uses `historyTypes='messageAdded'` only | `historyTypes='messageAdded,labelAdded'` (Q1.A — one call, comma list) |
| Gmail-to-self self-sends NEVER surface (Sent path skips messageAdded) | Surface within ≤3 poll cycles via `labelAdded:INBOX` (Q2.A INBOX literal filter) |
| Per-cycle dedupe is implicit (Gmail only fires messageAdded once) | Explicit per-cycle `Set<message_id>` dedupe; first-seen wins (Q3.A) |
| `gmail.poll.cycle` audit detail has `messages_observed` only | Adds 4 new counters: `messages_observed_via_messageAdded_only`, `messages_observed_via_labelAdded_only`, `messages_observed_via_both`, `messages_dedupe_drops` (Q6.A) |
| No per-message structural observability | NEW audit kind `fomo.gmail.poll.event_observed` per (cycle, message_id) AFTER dedupe — STRUCTURAL fields ONLY: `event_types_seen`, `inbox_label_present`, `is_dedupe_drop`, `message_id` (Q6.A) |
| Malformed Gmail events silently disappear | NEW audit kind `fomo.gmail.poll.event_skipped` with `reason='malformed_labelAdded'` (Q5 fallback; best-effort, NO retry) |

**Out of scope (founder-locked — see [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-8-scope.md) §"Scope boundaries"):**
- HMR / renderer / ranker prompt — v0.5.7 PASS state preserved
- 3E.1 reversal (no LLM in body composition) — permanently preserved
- Personalized Importance Learning substrate — own phase
- Feedback + Learn/Grow Loop substrate — strategic next-phase candidate
- SendBlue tier work (F1) — own future phase
- Friend C onboarding — three-friend cap; expansion is its own decision
- New schema / table / migration — Q4.A locks NO new persistence
- Raw email content in any new audit detail — Q6 lock: structural enums + booleans + `message_id` only

---

## §1 — Baseline snapshot (Terminal 1, run once)

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity-check DATABASE_URL didn't get clobbered by your zshrc (see memory
# stale-database-url-shell-export):
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"
# Must end with sslmode=require. If clobbered, re-source.

# Snapshot recent gmail.poll.cycle rows — load-bearing for C13. Before
# v0.5.8 these rows do NOT carry the new counter keys. After v0.5.8 they
# do. The diff is the proof.
psql "$DATABASE_URL" -P pager=off -c "
SELECT occurred_at,
       (detail->>'messages_observed')::int AS messages_observed,
       (detail->>'messages_observed_via_messageAdded_only')::int AS m_added_only,
       (detail->>'messages_observed_via_labelAdded_only')::int  AS l_added_only,
       (detail->>'messages_observed_via_both')::int             AS both,
       (detail->>'messages_dedupe_drops')::int                  AS dedupe_drops
FROM audit_log
WHERE action = 'gmail.poll.cycle'
  AND occurred_at > now() - interval '24 hours'
ORDER BY occurred_at DESC
LIMIT 30;
" | tee /tmp/v0.5.8-baseline-gmail-poll-cycle.txt

# Snapshot recent rank_results count for the founder — load-bearing for C10
# (we'll diff the count AFTER §6 Test 1).
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) FROM rank_results
WHERE user_id='founder' AND created_at > now() - interval '24 hours';
" | tee /tmp/v0.5.8-baseline-rank-results-count.txt

# Snapshot stop_active rows (load-bearing for C14 cross-tenant):
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.8-baseline-stop-active.txt

# Note the timestamp NOW — you'll use it later as the smoke-start cutoff:
date -u +"%Y-%m-%dT%H:%M:%SZ"
```

Confirm all three snapshot files have content. Copy the timestamp output — paste into queries below as `<SMOKE_START_TS>`.

**Pre-smoke setup note:** If founder is currently in `stop_active = true` from a prior smoke, DELETE the row BEFORE the baseline (same as v0.5.6/7 runbook). The v0.5.3 OPTED_OUT drift detector may re-record it after the first send if SendBlue blocks delivery — that's correct hardening behavior, not a v0.5.8 violation (the C14 diff is for NON-founder rows).

---

## §2 — Add v0.5.8 env vars

Append to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_8_BASELINE_CONFIRMED=true
FOMO_V0_5_8_WINDOW_HOURS=24
```

Re-source in Terminal 1: `set -a; source apps/fomo/.env.3b3.local; set +a`.

---

## §3 — Preflight + code-level sanity (Test 0)

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.8
```

Expect: `✓ Preflight passed.` with 2 WARNs at SCAFFOLDING-time:
- `FOMO_AUDIT_ACTIONS: 'fomo.gmail.poll.event_observed' PENDING runtime commit`
- `FOMO_AUDIT_ACTIONS: 'fomo.gmail.poll.event_skipped' PENDING runtime commit`

Zero WARNs after the runtime commit lands.

If any ERROR fires, fix the named env var and re-run before proceeding.

**Code-level sanity (C3 — load-bearing):**

```bash
grep -n "historyTypes:" apps/fomo/src/adapters/gmail/client.ts
```

Expect a line containing both `'messageAdded'` AND `'labelAdded'` (single comma-joined string per Q1.A). Until the runtime commit lands, this prints the v0.5.7 line (`historyTypes: 'messageAdded'` only) — record that as the baseline.

**Unit-test sanity (C4–C9):**

```bash
pnpm --filter @brevio/fomo test src/adapters/gmail/gmail-client.test.ts
pnpm --filter @brevio/fomo test src/workers/gmail-poll.test.ts
```

Expect 6 new tests (per Q3/Q5 locks) green:
- C4: external messageAdded path → dispatch (no regression)
- C5: Gmail-to-self labelAdded:INBOX-only → dispatch
- C6: routed/forwarded labelAdded:INBOX → dispatch
- C7: same message in BOTH event types same cycle → ONE dispatch (Q3.A dedupe)
- C8: labelAdded with NON-INBOX label → no dispatch
- C9: malformed labelAdded (missing `addedLabels`) → `event_skipped` audit + no dispatch

---

## §4 — Boot dev server (Terminal 1)

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.8.log
```

Wait for `fomo.server.listening` on port 8080. Leave running.

---

## §5 — ngrok (Terminal 2, OPTIONAL)

v0.5.8 does NOT exercise the SendBlue inbound webhook path. ngrok is not required for v0.5.8 PASS. Skip unless you also want to opportunistically verify outbound delivery during Test 1.

```bash
ngrok http --domain=unshivering-interaulic-beatriz.ngrok-free.dev 8080
```

---

## §6 — Tests

### Test 1 — Path A (load-bearing): Gmail-to-self synthetic important email

**Goal:** prove the labelAdded:INBOX-only path produces a rank within ≤3 poll cycles. v0.5.7 baseline = **NEVER** surfaces this. v0.5.8 baseline = ≤3 cycles.

In Terminal 3:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Capture the smoke-start timestamp:
SMOKE_START_TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "SMOKE_START_TS=$SMOKE_START_TS"
```

In the Gmail web UI, send yourself an email **FROM your founder Gmail TO your founder Gmail** with a unique subject:

> Subject: `[v0.5.8-smoke] Q4 board deck — sign-off needed tomorrow`
>
> Body: 3–5 sentences with time-sensitive content (e.g., counselor/employer/school/contract style).

**Critical:** this MUST be Gmail-to-self (Gmail web UI → Compose → To: your own founder Gmail address). Sending from a different account is Test 2's path.

Wait ~60s (poll interval 10s; ≤3 cycles = ≤30s typical, ≤60s worst-case).

Watch Terminal 1 for:
- `fomo.rank.completed` (ranker classified it)
- `fomo.slack.posted` (Slack card appeared in your founder DM)

**Query the new audit kind first (C12):**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail, occurred_at
FROM audit_log
WHERE action = 'fomo.gmail.poll.event_observed'
  AND actor_user_id = 'founder'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC
LIMIT 10;
"
```

Expected: at least one row whose `detail.event_types_seen` JSON array contains `"labelAdded"`. For the Gmail-to-self message specifically, `event_types_seen` MAY contain only `["labelAdded"]` (no messageAdded — Gmail's known quirk that v0.5.8 fixes).

**Query the cycle-level counters (C13):**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT occurred_at,
       (detail->>'messages_observed')::int AS messages_observed,
       (detail->>'messages_observed_via_messageAdded_only')::int AS m_added_only,
       (detail->>'messages_observed_via_labelAdded_only')::int  AS l_added_only,
       (detail->>'messages_observed_via_both')::int             AS both,
       (detail->>'messages_dedupe_drops')::int                  AS dedupe_drops
FROM audit_log
WHERE action = 'gmail.poll.cycle'
  AND occurred_at > '$SMOKE_START_TS'
ORDER BY occurred_at ASC;
"
```

Expected:
- `l_added_only` ≥ 1 across the smoke window (the KEY METRIC — this is the message v0.5.7 would have missed)
- `both == dedupe_drops` (Q3.A invariant — every "both" pair produces exactly one drop)
- `messages_observed` continues to count post-dedupe unique observations (documented carry-forward invariant)

**Query rank_results to confirm ≤3 cycles:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, message_id, label, round(score::numeric, 3) AS score, length(reason) AS reason_len, created_at
FROM rank_results
WHERE user_id='founder' AND created_at > '$SMOKE_START_TS'
ORDER BY created_at ASC LIMIT 5;
"
```

Expected: at least one new row whose `created_at` is ≤30s (typical) or ≤60s (worst-case 3 cycles at 10s interval each, allowing slack) after `$SMOKE_START_TS`.

**Approve the Slack card** (opportunistic — outbound delivery is NOT load-bearing for v0.5.8 PASS). Verify the outbound audit chain (`fomo.send.attempted` → `fomo.send.succeeded` OR `fomo.send.failed` OR `fomo.send.opt_out_drift_detected` if SendBlue blocks) is consistent with the v0.5.7 HMR shape (sanity for C14).

**Pass criteria for Test 1:**
- [ ] `fomo.rank.completed` row arrived ≤3 cycles after Gmail-to-self send (C10) ✓
- [ ] ≥1 `fomo.gmail.poll.event_observed` row whose `event_types_seen` contains `"labelAdded"` (C12) ✓
- [ ] `event_types_seen` array contains no values outside `messageAdded`/`labelAdded` (C12 enum-safety) ✓
- [ ] Cycle counter `messages_observed_via_labelAdded_only` ≥ 1 in window (C13) ✓
- [ ] `event_observed` detail contains NO subject / sender / body / raw label name (C12 sanitized scan) ✓
- [ ] Slack card body still renders via `human-message-v0.3.0` template (HMR un-regressed — C14) ✓

### Test 2 — External regression (Path B optional): icloud → gmail external email

**Goal:** prove the existing `messageAdded` path still produces dispatch (no v0.5.8 regression on external mail).

Send a 5-sentence important-looking email from an iCloud (or any non-Gmail) account TO your founder Gmail address:

> Subject: `[v0.5.8-smoke-ext] follow-up needed by end of week`

Wait ~5–10 minutes (Gmail history-event batching for external mail can be slow — this is exactly the lag v0.5.8 also helps with, but doesn't fully eliminate).

Re-run the C13 cycle-counter query above. Now expect:
- `m_added_only` ≥ 1 (the external email's messageAdded path)
- `l_added_only` ≥ 1 still (carry-over from Test 1)
- `both` may be ≥ 1 (if Gmail batched both events for the external mail in one cycle)
- `dedupe_drops == both` (invariant)

**Pass criteria for Test 2:**
- [ ] External email also ranks (no regression on messageAdded path — C11) ✓
- [ ] `m_added_only` ≥ 1 in window ✓

### Test 3 — HMR regression: v0.5.7 still PASSES on this branch

**Goal:** prove v0.5.8 does NOT touch the renderer.

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7
```

Expected: **VERDICT: PASS** (or the documented blocked-external shapes from the v0.5.7 SMOKE_REPORT, identical shape). Operator confirms identical shape in the §8 report.

If `smoke-evidence:v0.5.7` reports a NEW failure that did not exist on the prior PR #46 PASS, **v0.5.8 has regressed HMR — STOP and fix before merging.**

### Test 4 — Cross-tenant isolation

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.8-post-stop-active.txt

# Non-founder diff — these rows MUST be byte-identical to baseline:
diff <(grep -v "founder " /tmp/v0.5.8-baseline-stop-active.txt) <(grep -v "founder " /tmp/v0.5.8-post-stop-active.txt)
```

**Pass criterion:** the non-founder diff is empty. (Founder row may differ if v0.5.3 drift detector re-recorded it after a SendBlue OPTED_OUT bounce — that's correct cross-phase behavior, not a v0.5.8 violation.)

Also check no non-founder sends / no non-founder event_observed rows:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, COUNT(*) AS rows
FROM audit_log
WHERE action IN ('fomo.send.attempted', 'fomo.gmail.poll.event_observed')
  AND occurred_at > '$SMOKE_START_TS'
GROUP BY actor_user_id
ORDER BY actor_user_id;
"
```

Only `actor_user_id='founder'` should appear.

---

## §7 — Run all 8 evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
FOMO_V0_5_2_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
FOMO_V0_5_4_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.5
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.6
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.8
```

**Known expected non-PASS shapes (NOT v0.5.8 regressions per C14 — same shapes prior SMOKE_REPORTs documented):**

- v0.5.3 may FAIL on Item #1 (no `/onboard/callback` in window — v0.5.8 is founder-only, expected)
- v0.5.4 may FAIL on C13/C14 (window-slide false positives)
- v0.5.5 will FAIL C2/C3/C11 (SendBlue OPTED_OUT blocked-external — F1 own future phase)
- v0.5.6 + v0.5.7 should PASS unchanged

v0.5.8 should report **VERDICT: PASS** if Tests 1–4 succeeded AND no HMR regression.

---

## §8 — Fill `SMOKE_REPORT_v0.5.8.md`

```bash
cp docs/SMOKE_REPORT_TEMPLATE_v0.5.8.md docs/SMOKE_REPORT_v0.5.8.md
```

Open and fill:
- §1 prerequisites (tick what's done)
- §3 PASS criteria table with evidence
- §4–§11 paste each evidence-script output
- §12 paste sample event_observed JSON detail + sample cycle counter row
- §13 founder observations
- §14 verdict: PASS / FAIL / PENDING
- §15 sign-off + date

---

## §9 — Aftercare

- [ ] Kill Terminal 1 dev server + Terminal 2 ngrok (if used)
- [ ] If Test 1 left a fresh `stop_active` row for founder from outbound failure, no action (v0.5.3 drift detector behavior — not a v0.5.8 concern)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via Test 3
- [ ] No new schema / migration introduced (Q4.A invariant)
- [ ] No LLM call accidentally introduced anywhere (3E.1 invariant — v0.5.8 is poller-layer only)

---

## §10 — Commit the report

```bash
git checkout -b docs-smoke-report-v0.5.8
git add docs/SMOKE_REPORT_v0.5.8.md
git commit -m "docs: SMOKE_REPORT_v0.5.8 VERDICT: <PASS/FAIL>"
git push -u origin docs-smoke-report-v0.5.8
gh pr create --title "docs: v0.5.8 SMOKE_REPORT VERDICT: <verdict>" --body "..."
```

Or commit directly to main if preferred.

---

## What v0.5.8 PASS does NOT promise

Per the [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-8-scope.md) §"What v0.5.8 PASS does NOT auto-unlock":

- ❌ v0.5.9 Feedback + Learn/Grow Loop substrate — its own 6Q gate
- ❌ F1 SendBlue tier fix — own future phase
- ❌ Friend C onboarding — three-friend cap; expansion is its own decision
- ❌ PIL substrate — own phase
- ❌ Auto-send — own gate per FOMO_PLAN v0.8
- ❌ 3E.1 reversal — permanently preserved
- ❌ Any HMR-surface expansion (calendar / drafts / tasks / etc.) — each own 6Q gate
- ❌ OAuth auto-refresh-after-expiry hardening ([hardening-backlog](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_hardening-backlog.md) candidate entry #2 — own future gate)
- ❌ The four v0.5.7 §11 bonus findings — each own gate
- ❌ Short-body length policy resolution — own future gate per v0.5.6 PASS finding
- ❌ Runbook drift-detector amendment — own future gate per v0.5.6 PASS finding

**Next phase is decided AT THE NEXT 6-question gate** with the binding three principle-gate questions + Core Dimension Check + per-phase Q1–Q6. Strategic candidate locked: **v0.5.9 Feedback + Learn/Grow Loop substrate** per founder direction.
