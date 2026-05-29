# Phase 3G.1 — Production Hardening (founder-only smoke)

> Four real-incident-backed visibility/safety fixes that bridge between
> v0.1 demo (PR #35 merged 2026-05-29) and v0.5 friend beta. Scope
> locked via 6-question gate 2026-05-29.

**Scope locked (founder-locked, no creep allowed):**
- **#1 Neon migration verification at boot** — fail-loud, named list of missing tables, NO auto-apply anywhere
- **#2 SendBlue OPTED_OUT decoding + local `stop_active` sync** — parse response body into named safe fields, never raw dump; on OPTED_OUT write stop_active=true + emit `fomo.send.opt_out_drift_detected`
- **#3 needs_reauth visibility** — boot WARN + `users_needs_reauth` count on `fomo.poll.cycle`
- **#10 memory_signals snapshot at boot** — log every active signal for the founder with kind/age/source/confidence; NEVER raw detail

Hard boundaries reaffirmed: no new tools, no new capabilities, no v0.5 friend beta work, no privacy copy, no friend webhooks, no auto-send, no snooze resurface scheduler, no OAuth landing page, no stable-hostname infra, no broad ops sprint, no items #4 / #5 / #6 / #7 / #8 / #9 / #11 from the catalog.

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_3G.md` on `main` (v0.1 PASS)
- [ ] On branch `phase3g1-production-hardening` with the four items implemented + tested

Sync local main, then check out the branch:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
git fetch origin && git checkout phase3g1-production-hardening
set -a; source apps/fomo/.env.3b3.local; set +a
```

---

## 1. Static checks (regression tests + evidence script)

The PASS criterion is: **all four regression tests are green and the static check passes.**

```bash
pnpm --filter @brevio/fomo run preflight:3g1
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run lint
pnpm --filter @brevio/fomo run test
pnpm --filter @brevio/fomo run smoke-evidence:3g1
```

Expected:
- preflight: `✓ Preflight passed.`
- build: clean (no TS errors)
- lint: clean (no eslint output)
- test: `pass 900+` (the four items add ~39 tests)
- evidence: `VERDICT: PASS` with all five static checks green

If any of the above fail, stop and paste the output.

---

## 2. Optional founder fault-injection — see the new behavior in a live dev run

Each item has a regression test that's already green. These scenarios are for
the founder to **observe the new behavior in a real dev server** — that's the
"clear evidence/check path proving the fix" criterion in the 6-question gate.

PGlite-only / dev-only. **Do NOT drop tables on the live Neon database.**

### 2a. Item #10 — memory_signals snapshot at boot

The most visible fix. Pre-seed a `stop_active=true` signal in Neon (or whatever
DB your `DATABASE_URL` points at — if it's still on the production Neon, the
signal is probably already there from yesterday's 3G run):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT kind, detail, source, confidence, updated_at
FROM memory_signals
WHERE user_id='founder';"
```

Boot the dev server briefly:

```bash
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g1-snapshot.log &
sleep 8
```

Look for the new event:

```bash
grep -E "fomo.memory_signals.snapshot_at_boot" /tmp/fomo-3g1-snapshot.log
```

Expected: one INFO line per active signal with `kind`, `scope_key`, `age_seconds`,
`source`, `confidence`, `active_flag`. **The line MUST NOT include the raw
detail body.** Visually verify there are no extra keys beyond the six listed.

Stop the server:

```bash
lsof -ti :8080 | xargs kill -9 2>/dev/null
```

### 2b. Item #3 — needs_reauth visibility

Mark the founder token needs_reauth=true (this is the same state 3F.2 left us
in yesterday, so you might already be there):

```bash
psql "$DATABASE_URL" -P pager=off -c \
  "UPDATE oauth_tokens SET needs_reauth=true WHERE user_id='founder';"
```

Boot:

```bash
FOMO_GMAIL_POLLING_ENABLED=true pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g1-reauth.log &
sleep 15  # 1-2 poll cycles
lsof -ti :8080 | xargs kill -9 2>/dev/null
```

Verify:

```bash
grep -E "fomo.poll.needs_reauth_at_boot|users_needs_reauth" /tmp/fomo-3g1-reauth.log
```

Expected:
- One `fomo.poll.needs_reauth_at_boot` WARN line with `user_id: founder` and the
  re-auth runbook pointer.
- At least one `fomo.poll.cycle` INFO line with `users_needs_reauth: 1` (distinct
  from the generic `users_skipped`).

If you want polling to work again, re-auth via the procedure in
`feedback_brevio-oauth-google-reauth-procedure` memory (5-step session-mint +
curl + browser flow).

### 2c. Item #2 — SendBlue OPTED_OUT decoding

The cleanest live verification is to run the regression test directly:

```bash
pnpm --filter @brevio/fomo run test 2>&1 | grep -E "OPTED_OUT drift|opt_out_drift_detected"
```

Expected: all six `runOutboundOnce — OPTED_OUT drift detection` tests pass.

If you want to see it against the live SendBlue API:
1. Text STOP from your phone to the SendBlue number to create a carrier-level opt-out.
2. Manually clear `memory_signals.stop_active` via SQL (the same drift that hit
   the 3G smoke).
3. Re-trigger an approved alert (the runbook in `docs/smoke-test-3g-full-demo.md`
   §4-5 covers how).
4. Confirm in the audit log that `fomo.send.opt_out_drift_detected` fires AND
   `memory_signals.stop_active` is re-written with `source='opt_out_drift_carrier'`.
5. Text START to recover.

### 2d. Item #1 — migration verifier (fault-injection in PGlite, not Neon)

The regression test suite (`src/db/migration-verifier.test.ts`) exercises this
against PGlite with a deliberately-skipped migration. The live-Neon path is
verified passively every time `pnpm dev` boots (the verifier runs at startup).

To see the fail-loud path on real Neon **without** dropping a table:

1. Imagine a hypothetical migration `0005_<new_table>.sql` is committed but not yet applied.
2. The verifier would refuse to boot, naming `<new_table>` + `0005_<new_table>.sql`.
3. The operator runs `pnpm --filter @brevio/fomo run migrate:neon` to apply it.
4. The verifier passes; the server boots.

You can confirm the migration script is wired:

```bash
grep -A 1 'migrate:neon' apps/fomo/package.json
```

---

## 3. Clean-stop (carry-over from 3G — still required)

3G.1 doesn't change the kill-switch surface, but the clean-stop check is still
the gate's final invariant.

```bash
FOMO_GMAIL_POLLING_ENABLED=false FOMO_RANKER_ENABLED=false \
FOMO_SLACK_REVIEW_ENABLED=false FOMO_SEND_ENABLED=false \
FOMO_SENDBLUE_INBOUND_ENABLED=false \
pnpm --filter @brevio/fomo dev > /tmp/fomo-3g1-cleanstop.log 2>&1 &
sleep 8
curl -s -o /dev/null -w "/sendblue/inbound HTTP %{http_code}\n" -X POST http://localhost:8080/sendblue/inbound
curl -s -o /dev/null -w "/slack/interactivity HTTP %{http_code}\n" -X POST http://localhost:8080/slack/interactivity
lsof -ti :8080 | xargs kill -9 2>/dev/null
```

Both curls → HTTP 404 (same as 3G).

---

## 4. Fill in `docs/SMOKE_REPORT_3G1.md`

Use `docs/SMOKE_REPORT_TEMPLATE_3G1.md`. Required:
- Per-item PASS bullet citing the regression test that proves it
- Build / lint / test counts
- Optional: founder observations from §2 fault-injection scenarios
- Verdict: PASS

Commit. Open PR. Merge to main. **v0.5 friend beta gate opens.**

---

## What's intentionally NOT in 3G.1

- Snooze resurface scheduler — v0.3+
- Friend beta privacy copy + signed friend webhooks — v0.5
- Auto-send — its own gate after v0.5 stability
- Items #4 / #5 / #6 / #7 / #8 / #9 / #11 from the hardening catalog — deferred to a post-v0.5 ops sprint
- Any new tools, any new capabilities
