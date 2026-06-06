# Phase v0.5.6 Smoke Test Runbook — iMessage Tone + Summary Length (founder-only)

> **Scope:** Address Friend B's "robotic + truncated iMessage" feedback from v0.5.4 §10. **HYBRID approach (founder Q3 corrected lock 2026-06-05):** preserve the 3E.1 no-LLM-body-generation directive — `renderFounderText` stays deterministic — and use the existing carve-out for `rank.reason` only. Two surfaces: (a) deterministic shell rewrite (drops `FOMO · IMPORTANT (0.92)` header, sentence-shaped composition, sentence-boundary truncation); (b) ranker `reason` field prompt + structured output schema + deterministic fallback. Length policy: target 220–280 / hard cap 320 / absolute 340 / 1–2 short sentences / no mid-sentence cut / no arbitrary ellipsis. Founder-only smoke — **no friend involved** (three-friend cap holds).
>
> **Scaffolding-vs-runtime ordering:** this runbook lands as part of the SCAFFOLDING commit. Running the steps below before the runtime implementation commit lands WILL produce a `VERDICT: PENDING` (not FAIL) because two runtime artifacts are pending: (1) the audit kind `fomo.alert.drafter_schema_failed` is not yet registered; (2) `FOUNDER_TEXT_TEMPLATE_VERSION` is still `founder-text-v0.1.0`. That is expected. Re-run the full sequence AFTER the runtime commit lands; only then is `VERDICT: PASS` possible.

---

## §0. Prerequisites

Run **once** before the smoke starts.

- [ ] `main` is at the SCAFFOLDING + RUNTIME commit chain on `phase-v0.5.6-imessage-tone`.
- [ ] No friend is involved. Do NOT brief a Friend C; do NOT mint a friend invite; do NOT add a friend's Gmail to Google Cloud Console.
- [ ] Founder iPhone is the device that will receive the manual real-iMessage taste check (§6 Test 3).
- [ ] **Founder is un-flagged from SendBlue OPTED_OUT** (one-time ops via SendBlue dashboard — does NOT subsume F1 broader tier work). Without this, §6 Test 3 cannot deliver an iMessage; you can still run Tests 1, 2, 4 (mock + audit + cross-tenant) and accept C10 as N/A.
- [ ] ngrok is healthy and forwarding to `localhost:8080` (same static subdomain as v0.5.4 / v0.5.5 — `FOMO_FRIEND_BETA_BASE_URL` unchanged). Required only for §6 Test 3; Tests 1/2/4 don't strictly need it.
- [ ] SendBlue Sandbox tier still active for `SENDBLUE_FROM_NUMBER` (same prereq as v0.5.5). Required only for §6 Test 3.
- [ ] Prior phase v0.5.5 PR (#43) is on `main` with its known FAIL-external-blocker verdict. v0.5.6 explicitly does NOT retry v0.5.5.

---

## §1. Capture the §0 baseline snapshot (load-bearing for C2/C3/C4 template-version checks)

Run **before** any env edits or server boot. The C3 + C4 evidence checks query `fomo.send.attempted` rows in a rolling window; the baseline captures the v0.5.5-era `template_version='founder-text-v0.1.0'` distribution so that any stale rows persisting into the v0.5.6 window are interpretable, not surprising.

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id,
       detail->>'template_version' AS template_version,
       (detail->>'content_chars')::int AS content_chars,
       occurred_at
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > now() - interval '7 days'
ORDER BY occurred_at DESC
LIMIT 20;
" | tee /tmp/v0.5.6-baseline-send-attempted.txt
```

You should see only `founder-text-v0.1.0` rows from prior phases. After the v0.5.6 runtime commit lands and the smoke runs, the same query (in §8) should show the new bumped version (e.g. `founder-text-v0.2.0`) on freshly-rendered rows — and zero stale `v0.1.0` rows produced after the smoke start time.

Also snapshot the cross-tenant `stop_active` rows so §6 Test 4 (cross-tenant) has a known-good baseline:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.6-baseline-stop-active.txt
```

---

## §2. Set the v0.5.6 env vars

Open `apps/fomo/.env.3b3.local` and add to the bottom:

```
FOMO_V0_5_6_BASELINE_CONFIRMED=true
FOMO_V0_5_6_WINDOW_HOURS=24
```

`FOMO_V0_5_6_BASELINE_CONFIRMED=true` is your assertion that you ran §1 above. Setting it without running §1 will produce a misleading PASS that doesn't compare against a real snapshot.

`FOMO_V0_5_6_WINDOW_HOURS=24` is the smoke window for evidence-script queries. Default 24; widen only if the smoke runs across multiple days.

Existing v0.5.4 / v0.5.5 env vars stay unchanged.

---

## §3. Preflight

```bash
set -a; source apps/fomo/.env.3b3.local; set +a
pnpm --filter @brevio/fomo run preflight:v0.5.6
```

**At scaffolding time** (runtime commit not yet landed), expect:
- `✓ Preflight passed.`
- 2 WARN lines:
  1. `fomo.alert.drafter_schema_failed` PENDING runtime commit
  2. `FOUNDER_TEXT_TEMPLATE_VERSION` PENDING runtime commit (still `founder-text-v0.1.0`)
- A NOTE at the bottom reminding you that smoke-evidence will be PENDING until runtime lands.

**At runtime time** (runtime commit landed), both WARN lines disappear automatically.

If preflight reports an ERROR (not WARN), fix the named env var and re-run before proceeding. Errors include any missing v0.5.3 or v0.5.5 carry-forward audit kind, missing keys, `BREVIO_DEV_MODE=true`, `FOMO_AUTO_SEND_ENABLED=true`, etc.

---

## §4. Boot the dev server

In Terminal 1:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"   # must end with sslmode=require
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.6.log
```

The `DATABASE_URL` sanity check guards against the stale Render-staging shell-export footgun (memory: `feedback_stale-database-url-shell-export`). If the tail doesn't end with `sslmode=require`, re-source the env file in that terminal.

Wait for `fomo.server.listening` on port 8080.

---

## §5. ngrok (required only for §6 Test 3)

In Terminal 2:

```bash
ngrok http --domain=unshivering-interaulic-beatriz.ngrok-free.dev 8080
```

Required for the manual real-iMessage taste check. Skip if you are running Tests 1/2/4 only.

---

## §6. The four tests

### Test 1 — Mock-SendBlue regression (the CI bar)

Goal: prove the deterministic-shell rewrite is in effect end-to-end against a mock SendBlue, without needing real iMessage delivery. This is the regression-gate that every future PR should be able to re-run without iPhone access.

```bash
# In Terminal 3 (separate shell):
set -a; source apps/fomo/.env.3b3.local; set +a

# Inject a synthetic important email into the founder's Gmail (or use the
# existing brevio-canary mailbox if 3F.2 fixtures are present). Use the
# existing v0.5.4 synthetic-email script if available; otherwise send a
# real email to the founder Gmail with a unique subject like
# "[v0.5.6-smoke] Q3 board deck final draft" so you can grep audit later.
```

After the polling worker picks it up (≤ 60s) and the ranker decides `important`:

1. Watch Terminal 1 for the Slack `fomo.alert.queued` audit row.
2. In the founder's Slack DM, click **Approve** on the new alert card.
3. Wait for `fomo.send.attempted` audit row (this is the load-bearing artifact).
4. Query the audit row:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail->>'template_version' AS template_version,
       (detail->>'content_chars')::int AS content_chars,
       occurred_at
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > '<smoke-start-timestamp>'
ORDER BY occurred_at DESC
LIMIT 5;
"
```

Pass criteria for Test 1:
- `template_version` is the bumped v0.5.6 version (NOT `founder-text-v0.1.0`)
- `content_chars` is in 220–320 range (target 220–280 ideal)
- No `fomo.alert.drafter_schema_failed` audit row from this test (ranker.reason was within schema)

If you set `FOMO_OUTBOUND_USE_MOCK_SENDBLUE=true` (recommended for Test 1), the actual SendBlue API is not called — the audit row is produced, the body is rendered, but no iMessage goes out. This is the CI-runnable regression bar.

### Test 2 — Schema-violation fallback (the Q6 lock proof)

Goal: prove that when ranker.reason violates the new structured output schema, the deterministic fallback string is substituted, the `fomo.alert.drafter_schema_failed` audit row is written, and NO retry happens.

Approach: induce a violation deterministically. Options (pick one):

a. Temporarily lower the schema cap to a value the ranker will always exceed (e.g. set a debug-only `FOMO_V0_5_6_REASON_HARD_CAP=20` env if the runtime exposes such a knob; otherwise use approach b).

b. Bench a unit test directly:

```bash
pnpm --filter @brevio/fomo test src/ranker/validator.v0.5.6.test.ts
```

If approach a is wired:

1. Restart the dev server with the lowered cap.
2. Inject a synthetic important email (same as Test 1).
3. Watch for `fomo.alert.drafter_schema_failed` audit row.
4. Verify the subsequent `fomo.send.attempted` row's body uses the deterministic fallback (the text the runtime substitutes when reason fails — likely something like "Marked important by Brevio.").
5. Verify zero retry — only one `fomo.send.attempted` per alert.

Pass criteria for Test 2:
- ≥1 `fomo.alert.drafter_schema_failed` audit row in window
- The downstream `fomo.send.attempted` still produced output (deterministic fallback fired)
- No second `fomo.send.attempted` for the same alert_id (Q6 no-retry holds)

### Test 3 — Manual real-iMessage taste check (the founder eye-test)

Goal: confirm the rendered iMessage actually feels like a helpful nudge on a real iPhone, not a bot. This is the taste check Friend B's feedback was about.

Prerequisite: founder un-flagged from SendBlue OPTED_OUT (see §0).

1. Verify `FOMO_OUTBOUND_USE_MOCK_SENDBLUE` is NOT set (or is `false`) so the real provider fires.
2. Inject a synthetic important email (same as Test 1).
3. Approve the Slack card.
4. Wait for the iMessage on the founder iPhone.
5. Founder evaluates:
   - [ ] No `FOMO · IMPORTANT (0.92)` telemetry header at the top
   - [ ] Sentence-shaped — reads like a person curated it, not newline-separated raw fields
   - [ ] No arbitrary `…` ellipsis (sentence-boundary truncation only)
   - [ ] Body contains the ranker's "why this matters" prose (not the email body snippet)
   - [ ] Total length feels right — scannable on lock screen, not a wall of text
   - [ ] Feels friendly, not robotic

Operator paste the exact received text into SMOKE_REPORT §10 for the historical record (manually redact any personal context before pasting if the synthetic email had any).

### Test 4 — Cross-tenant isolation

Goal: prove no other user's state was modified during the v0.5.6 smoke.

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.6-post-stop-active.txt

diff /tmp/v0.5.6-baseline-stop-active.txt /tmp/v0.5.6-post-stop-active.txt
```

Pass criteria: zero diff (v0.5.6 should NOT touch `stop_active` at all — that's v0.5.5 territory).

Also verify no non-founder `fomo.send.attempted` rows during the smoke window:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, COUNT(*) AS sends
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > '<smoke-start-timestamp>'
GROUP BY actor_user_id;
"
```

Only `actor_user_id='founder'` should appear.

---

## §7. Run all six smoke-evidence scripts

```bash
set -a; source apps/fomo/.env.3b3.local; set +a

pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.5
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.6
```

Known issues:
- **v0.5.2**: may need `FOMO_V0_5_2_WINDOW_HOURS=168` if the v0.5.2 audit rows are older than 24h. Per v0.5.2 runbook §7.
- **v0.5.4**: may FAIL on window-slide false positives (recorded in v0.5.5 PR #43 SMOKE_REPORT §7 as F2). Operator verifies this is the same known FAIL, not a v0.5.6-caused regression.
- **v0.5.5**: FAILed externally per PR #43 SMOKE_REPORT (SendBlue OPTED_OUT). Operator confirms the FAIL shape matches the PR #43 record — same C3/C8/C11 blocked-external lines — not a new regression from v0.5.6.
- **v0.5.6**: at scaffolding time → `VERDICT: PENDING` (expected); at runtime + smoke-run time → expect `VERDICT: PASS`.

---

## §8. Cross-tenant baseline diff

This is the load-bearing check for C7. Already covered in §6 Test 4.

```bash
diff /tmp/v0.5.6-baseline-stop-active.txt /tmp/v0.5.6-post-stop-active.txt
```

Expected: empty diff. v0.5.6 does not modify `stop_active`.

```bash
diff /tmp/v0.5.6-baseline-send-attempted.txt <(psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id,
       detail->>'template_version' AS template_version,
       (detail->>'content_chars')::int AS content_chars,
       occurred_at
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > now() - interval '7 days'
ORDER BY occurred_at DESC
LIMIT 20;
")
```

Expected: new rows on bumped `template_version` appear; baseline rows on `founder-text-v0.1.0` remain unchanged (no retroactive rewrites).

---

## §9. Fill in `docs/SMOKE_REPORT_v0.5.6.md`

Copy `docs/SMOKE_REPORT_TEMPLATE_v0.5.6.md` to `docs/SMOKE_REPORT_v0.5.6.md` and fill in:
- §3 PASS criteria table with evidence + ☑/☐/N/A
- §4–§9 evidence-script outputs
- §10 operator visual confirmations + the **pasted received iMessage text** from Test 3
- §11 founder observations
- §12 verdict (PASS / FAIL / PENDING — see scaffolding-vs-runtime note)
- §13 sign-off

Commit the filled report as `docs/SMOKE_REPORT_v0.5.6.md` once `VERDICT: PASS` on all six evidence scripts AND the §6 Test 3 manual taste check is confirmed.

**v0.5.6 PASS does NOT auto-unlock v1.0, Friend C, auto-send, or any other phase.** The next phase runs its own 6-question gate.

---

## §10. Aftercare

- [ ] If Test 2 was run with a temporarily lowered schema cap (`FOMO_V0_5_6_REASON_HARD_CAP=20` or similar), unset that env var
- [ ] If Test 2 left a broken ranker.reason in the database, the audit row is fine (historical); no rollback needed
- [ ] If you set `FOMO_OUTBOUND_USE_MOCK_SENDBLUE=true` for Tests 1/2, unset it
- [ ] If the founder re-flagged themselves OPTED_OUT during the smoke, send a real START to restore alert delivery
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.5 STOP enforcement still functional — sanity-check by re-running `pnpm smoke-evidence:v0.5.5` and confirming the FAIL shape is identical to PR #43's record (no new regressions)

---

## §11. What v0.5.6 PASS does NOT promise

v0.5.6 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **F1 SendBlue tier fix** — its own future-phase candidate
- **Personalized Importance Learning substrate** — separate large phase, may be pulled forward before v1.0 if false-positive trust risk warrants
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.6 explicitly PRESERVES 3E.1 via the hybrid scope
- **Per-user tone customization** — PIL-adjacent, future phase
- **Ranker rewrite** — only the `reason` field's prompt + schema changes; label/score outputs untouched
- **Google OAuth verification submission (B3)** — multi-week external process
- **A new email provider** — Gmail remains only active provider per FOMO_DESIGN §6.2
- **A new model provider** — OpenAI-first per FOMO_DESIGN §18
- **Dashboard / web UI**
- **Calendar / Drafting / MCP / browser automation** — L2+ surfaces

The next phase is decided AT THE NEXT 6-question gate.
