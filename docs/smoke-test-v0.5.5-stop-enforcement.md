# Phase v0.5.5 Smoke Test Runbook — STOP Enforcement + Confirmation (founder-only)

> **Scope:** bundle A1 (polling-after-STOP suppression) + B2 (one deterministic STOP confirmation reply) from v0.5.4 SMOKE_REPORT §12 followups. 24h idempotency window; best-effort audit no retry on confirmation failure. Founder-only smoke — **no Friend B/C involved** (three-friend cap holds; Friend B was the last GUARANTEED smoke).
>
> **Scaffolding-vs-runtime ordering:** this runbook lands as part of the SCAFFOLDING commit. Running the steps below before the runtime implementation commit lands WILL produce a `VERDICT: PENDING` (not FAIL) because the four new audit kinds (`fomo.sendblue.stop_confirmation_sent`, `fomo.sendblue.stop_confirmation_failed`, `fomo.alert.suppressed_stop_active`, `fomo.poll.skipped_stop_active`) are not yet registered. That is expected. Re-run the full sequence AFTER the runtime commit lands; only then is `VERDICT: PASS` possible.

---

## §0. Prerequisites

Run **once** before the smoke starts.

- [ ] `main` is at the SCAFFOLDING + RUNTIME commit chain (the runtime commit MUST be on the branch before §6 tests can produce PASS; running the §6 tests on scaffolding-only will produce PENDING).
- [ ] No friend is involved in this phase. Do NOT brief a Friend C; do NOT mint a friend invite; do NOT add a friend's Gmail to Google Cloud Console.
- [ ] Founder iPhone is the device that will send STOP/START iMessages.
- [ ] Founder iPhone is the device that will receive STOP confirmation iMessages.
- [ ] ngrok is healthy and forwarding to localhost:8080 (same static subdomain as v0.5.4 — `FOMO_FRIEND_BETA_BASE_URL` unchanged).
- [ ] SendBlue Sandbox tier still active for the SENDBLUE_FROM_NUMBER (same prereq as v0.5.2/v0.5.4 — without this, the STOP confirmation outbound will be silently gated).
- [ ] Latest `main` pulled, branch `phase-v0.5.5-stop-enforcement` checked out.

---

## §1. Capture the §0 baseline snapshot (load-bearing for C7 cross-tenant)

Run **before** any env edits or server boot. The C7 evidence check diffs the post-smoke `stop_active` rows against this baseline.

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.5-baseline-stop-active.txt
```

Confirm the output contains the v0.5.4 final state (4 rows expected: founder, Morris, gm3258, and Sheila's residual UUID `8fbead5c-…` which retains a stop_active row even though her users row was deleted in v0.5.4 aftercare).

---

## §2. Set the v0.5.5 env vars

Open `apps/fomo/.env.3b3.local` and add to the bottom:

```
FOMO_V0_5_5_BASELINE_CONFIRMED=true
FOMO_V0_5_5_WINDOW_HOURS=24
```

`FOMO_V0_5_5_BASELINE_CONFIRMED=true` is your assertion that you ran §1 above. Setting it without running §1 will produce a misleading C7 PASS that doesn't actually compare against a snapshot.

`FOMO_V0_5_5_WINDOW_HOURS=24` is the smoke window for the evidence-script queries. Default 24; widen only if the smoke runs across multiple days.

Existing v0.5.4 env vars (`FOMO_FRIEND_BETA_BASE_URL`, founder phone, polling caps, etc.) stay unchanged.

---

## §3. Preflight

```bash
set -a; source apps/fomo/.env.3b3.local; set +a
pnpm --filter @brevio/fomo run preflight:v0.5.5
```

**At scaffolding time** (runtime commit not yet landed), expect:
- `✓ Preflight passed.`
- 4 WARN lines: one for each of the four v0.5.5-expected audit kinds (`fomo.sendblue.stop_confirmation_sent`, `_failed`, `fomo.alert.suppressed_stop_active`, `fomo.poll.skipped_stop_active`) reporting `PENDING runtime commit`.
- A NOTE at the bottom reminding you the smoke-evidence will be PENDING until runtime.

**At runtime time** (runtime commit landed), the 4 WARN lines disappear automatically.

If preflight reports an ERROR (not WARN), fix the named env var and re-run before proceeding. Errors include any missing v0.5.3 hardening kind, missing keys, `BREVIO_DEV_MODE=true`, `FOMO_AUTO_SEND_ENABLED=true`, etc.

---

## §4. Boot the dev server

In Terminal 1:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"   # must end with sslmode=require
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.5.log
```

The `DATABASE_URL` sanity check guards against the stale Render-staging shell-export footgun (memory: `feedback_stale-database-url-shell-export`). If it doesn't end with `sslmode=require`, re-source the env file in that terminal before booting.

Wait for `fomo.server.listening` on port 8080.

---

## §5. ngrok

In Terminal 2:

```bash
ngrok http --url=https://unshivering-interaulic-beatriz.ngrok-free.dev 8080
```

Same static URL as v0.5.4. Required because the founder's STOP/START iMessages arrive via SendBlue inbound webhook, which needs a public HTTPS endpoint.

Confirm the `Forwarding` line shows `https://unshivering-interaulic-beatriz.ngrok-free.dev -> http://localhost:8080`.

---

## §6. Test sequence (founder-only, all 5 tests run from founder iPhone)

### Test 1: Founder STOP → confirmation received

1. From the founder iPhone, text **STOP** to `SENDBLUE_FROM_NUMBER` (+1 (214) 354-7196 in v0.5.4).
2. Within ~10 seconds:
   - Dev log should show `fomo.sendblue.inbound.received` (existing v0.1 event) + `fomo.sendblue.stop_recorded` (existing v0.1) + **NEW** `fomo.sendblue.stop_confirmation_sent` (v0.5.5 runtime).
   - The founder iPhone should receive an iMessage from SENDBLUE_FROM_NUMBER. Wording must contain "**You're unsubscribed**" and "**Text START to turn Brevio back on**" (or equivalent canonical phrases).
3. Run a quick DB check:
   ```bash
   set -a; source apps/fomo/.env.3b3.local; set +a
   psql "$DATABASE_URL" -P pager=off -c "
   SELECT user_id, jsonb_pretty(detail) AS detail, updated_at
   FROM memory_signals
   WHERE kind = 'stop_active' AND user_id = '${FOMO_FOUNDER_USER_ID:-founder}'
   ORDER BY updated_at DESC LIMIT 1;
   "
   ```
   The founder row should now have `active=true` AND a new field `stop_confirmation_sent_at=<ISO timestamp>` in the detail JSON (the runtime commit stores this on the existing `stop_active` signal — no new memory_signal kind).

### Test 2: Idempotency — duplicate STOP within 24h does NOT re-send

1. From the founder iPhone, text **STOP** again to `SENDBLUE_FROM_NUMBER` (any phrasing — "STOP", "stop", "unsubscribe").
2. Expect:
   - Dev log shows `fomo.sendblue.inbound.received` + `fomo.sendblue.stop_recorded`.
   - Dev log does NOT show a second `fomo.sendblue.stop_confirmation_sent` (idempotency guard fired).
   - Founder iPhone does NOT receive a second confirmation iMessage.
3. The runtime should log a structured INFO line like `fomo.sendblue.stop_confirmation_skipped` with `reason=within_idempotency_window` and the `seconds_until_next_eligible` countdown.

### Test 3: Founder START → alerts re-enabled

1. From the founder iPhone, text **START** to `SENDBLUE_FROM_NUMBER`.
2. Within ~10 seconds:
   - Dev log shows `fomo.sendblue.inbound.received` + `fomo.sendblue.start_recorded` (existing v0.1 event).
   - `stop_active` for founder updates to `active=false`.
3. Send yourself a FOMO-worthy email (founder Gmail → founder Gmail) with a real-sounding subject ("Re: Sequoia partner sync — Thursday 3pm work?").
4. Wait one poll cycle (~10 s). A founder Slack card appears with full v0.1 founder shape (Snippet + full footer).
5. Approve in Slack. Within ~10 s the founder iPhone receives the real FOMO iMessage. **Note the alert was created normally — the start_recorded re-enabled the pipeline.**

### Test 4: Induced confirmation failure → best-effort audit, no retry

This test induces a SendBlue 5xx for the STOP confirmation send specifically.

1. Set up: temporarily change `SENDBLUE_API_KEY_ID` to an invalid value in `apps/fomo/.env.3b3.local` (just append `-broken`). Restart the dev server.
   - **Important**: this will also break the next FOMO alert send and any other outbound. Run Test 4 in isolation; restore the key immediately after.
2. From the founder iPhone, text **STOP** again. (≥ 24 h after Test 1's STOP, OR clear the `stop_confirmation_sent_at` field manually so the idempotency guard doesn't pre-empt this test.)
3. Expect:
   - Dev log shows `fomo.sendblue.inbound.received` + `fomo.sendblue.stop_recorded` + `fomo.sendblue.stop_confirmation_failed` (with sanitized error_code/error_message in detail; ≤ 200 chars).
   - Dev log does NOT show `fomo.sendblue.stop_confirmation_sent`.
   - **Crucially**: no `_sent` row follows the `_failed` row for the same actor — the runtime did NOT retry.
4. Restore `SENDBLUE_API_KEY_ID` to its real value. Restart the dev server. Send yourself any test outbound to verify SendBlue is healthy again.

### Test 5: Cross-tenant isolation — other users untouched

Founder-driven; no other user interaction needed. Snapshot post-state and diff:

```bash
set -a; source apps/fomo/.env.3b3.local; set +a
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.5-post-stop-active.txt

diff /tmp/v0.5.5-baseline-stop-active.txt /tmp/v0.5.5-post-stop-active.txt | tee /tmp/v0.5.5-stop-active.diff
```

**Expected diff:** only the founder's row changed (updated_at + detail). Morris (`25c1a707-…`), gm3258 (`4606e1e7-…`), and Sheila's residual UUID (`8fbead5c-…`) rows must be byte-identical to baseline. If any non-founder row is in the diff, STOP — that is the cross-tenant regression v0.5.5 was designed to catch.

---

## §7. Run all 5 evidence scripts

After the §6 test sequence completes:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.5
```

**All five must print `VERDICT: PASS`.**

If `smoke-evidence:v0.5.5` prints `VERDICT: PENDING`, the runtime implementation commit has not yet landed — that is the expected state at scaffolding time. Once the runtime commit lands on this branch, re-run.

If `smoke-evidence:v0.5.5` prints `VERDICT: FAIL`, read the per-criterion detail and fix before considering merge.

If `smoke-evidence:v0.5.2` fails because of the 24 h window slide, set `FOMO_V0_5_2_WINDOW_HOURS=48` (or higher) and re-run that one (known followup from v0.5.4).

---

## §8. Cross-tenant diff (the v0.5.5 load-bearing check, formal)

Already captured in §6 Test 5; this section just re-states the assertion for the SMOKE_REPORT §7.

**Expected shape:**
- ONE row modified: founder's `stop_active` row, with `updated_at` inside the smoke window AND `detail.stop_confirmation_sent_at` populated (or absent if Test 4's failure mode was the last STOP).
- ZERO row count change (founder's row already existed; v0.5.5 updates it in place rather than adding a new one).
- ZERO modifications to Morris's row (`25c1a707-…`).
- ZERO modifications to gm3258's row (`4606e1e7-…`).
- ZERO modifications to Sheila's residual row (`8fbead5c-…`).

---

## §9. Fill in the SMOKE_REPORT

1. Open `docs/SMOKE_REPORT_TEMPLATE_v0.5.5.md`.
2. Save a copy as `docs/SMOKE_REPORT_v0.5.5.md`.
3. Fill the header, all 12 PASS criterion checkboxes, evidence outputs (paste verbatim), cross-tenant diff (paste verbatim), operator visual checks, founder observations, verdict, sign-off, aftercare.

---

## §10. Aftercare

- [ ] If Test 4 left the env with `SENDBLUE_API_KEY_ID` broken, restore it.
- [ ] If the founder is left in a STOP'd state and you want alerts to resume, send START.
- [ ] No friend involvement → no Sheila-style deletion ops needed.
- [ ] Verify Morris is STILL UNTOUCHED one last time: `psql "$DATABASE_URL" -P pager=off -c "SELECT user_id, updated_at FROM memory_signals WHERE user_id = '25c1a707-811a-48a8-8ef7-fd1008057c89' AND kind = 'stop_active';"` — `updated_at` must still be `2026-06-01 22:04:04.182916+00`.

---

## §11. Commit + merge

After the SMOKE_REPORT is filled with `VERDICT: PASS`:

```bash
git add docs/SMOKE_REPORT_v0.5.5.md
git commit -m "phase v0.5.5: STOP enforcement smoke PASS — VERDICT: PASS"
git push
```

Then merge the v0.5.5 PR on GitHub (manual, founder-only — same pattern as v0.5.4 PR #40).

---

## §12. What v0.5.5 PASS does NOT promise

v0.5.5 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- Personalized Importance Learning (separate phase candidate; may be pulled forward per `docs/personalized-importance-learning.md`).
- Friend C onboarding (three-friend cap; Friend C is OPTIONAL, not auto-scheduled).
- Auto-send (still its own gate per FOMO_PLAN v0.8).
- iMessage tone rewrite + summary length fix (B1 — separate candidate from Sheila's §10 feedback).
- Google OAuth verification submission (B3 — multi-week external process).
- Any new email provider or model provider.

The next phase is decided AT THE NEXT 6-question gate.

---

## Known stumbling blocks

| Symptom | Cause | Fix |
|---|---|---|
| `smoke-evidence:v0.5.5` reports `VERDICT: PENDING` | Runtime commit not yet on this branch | Land the runtime commit; re-run |
| Preflight WARNs about 4 v0.5.5 audit kinds | Same — runtime commit pending | Expected at scaffolding time |
| Founder iPhone receives 2 STOP confirmations | Test 2's STOP was > 24 h after Test 1's, OR idempotency guard not implemented | Verify timing; if implementation, file as v0.5.5 runtime bug |
| Founder iPhone receives NO confirmation after Test 1 STOP | SendBlue Sandbox tier expired, OR runtime commit not landed, OR `stop_confirmation_sent` audit row missing — check the dev log | See §0 prereqs; if SendBlue, re-verify in dashboard |
| C7 cross-tenant diff shows non-founder row changed | Cross-tenant regression — STOP a STOP — investigate before any further action | This is the load-bearing failure mode v0.5.5 must catch |
| Boot fails with `SSL/TLS required` (SQLSTATE 28000) | Stale Render `DATABASE_URL` shadowing Neon URL | Re-source `.env.3b3.local` in that terminal (memory: `feedback_stale-database-url-shell-export`) |
