# Phase 3G.1 Smoke Test Report — Production Hardening

> Filled after running every step in
> [`smoke-test-3g1-production-hardening.md`](smoke-test-3g1-production-hardening.md).
> Commit as `docs/SMOKE_REPORT_3G1.md` once `VERDICT: PASS`. **v0.5
> friend beta gate cannot begin until this report lands on `main`.**

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3g1-production-hardening`
**Commit SHA at run time:** _<git rev-parse HEAD>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3G.md` on `main` with `VERDICT: PASS` (v0.1 done)
- [ ] Branch `phase3g1-production-hardening` checked out with the four 3G.1 items implemented

---

## 2. Static gate

| Check | Command | Expected | Got |
|---|---|---|---|
| Preflight | `pnpm run preflight:3g1` | `✓ Preflight passed.` | ☐ |
| Build | `pnpm run build` | clean (no TS errors) | ☐ |
| Lint | `pnpm run lint` | clean (no eslint output) | ☐ |
| Tests | `pnpm run test` | `pass 900+` | ☐ |
| Evidence | `pnpm run smoke-evidence:3g1` | `VERDICT: PASS` | ☐ |

Paste the final line of each command (verbatim) below:

```
preflight: …
build: …
lint: …
test: ℹ pass <NNN>
evidence: VERDICT: PASS
```

---

## 3. Per-item PASS evidence (LOAD-BEARING)

Each item must cite the specific regression test that proves the fix.

### Item #1 — Neon migration verification at boot

- Incident covered: 2026-05-28 01:06 UTC; first 3 `/sendblue/inbound` POSTs returned HTTP 500 because `0004_inbound_replies` was PGlite-applied but not Neon-applied
- Regression tests:
  - `src/db/migration-verifier.test.ts` → `incident reproduction (2026-05-28 inbound_replies missing on Neon)` (2 tests)
  - `src/db/migration-verifier.test.ts` → `incident reproduction (multiple missing tables — generalized fault)` (2 tests)
  - `src/db/migration-verifier.test.ts` → `verifyMigrations does not write any row to any required table` (READ-ONLY invariant)
- Migration command: `pnpm --filter @brevio/fomo run migrate:neon` (wired in `apps/fomo/package.json`)
- Policy: fail-loud everywhere; no auto-apply; no env bypass

### Item #2 — SendBlue OPTED_OUT decoding + local `stop_active` sync

- Incident covered: 2026-05-29 01:12 UTC; alert `d9728e57…` failed with `client_error: HTTP 400` and no usable detail
- Regression tests:
  - `src/adapters/sendblue/client.test.ts` → `400 response-body decoder (3G.1 item #2)` (8 tests)
  - `src/workers/outbound-sender.test.ts` → `OPTED_OUT drift detection (Phase 3G.1 item #2)` (5 tests)
- New audit action: `fomo.send.opt_out_drift_detected` (allowlisted in `FOMO_AUDIT_ACTIONS`)
- New memory-signal source: `opt_out_drift_carrier` (allowlisted in `MEMORY_SIGNAL_SOURCES`)
- Privacy invariant proven: raw response body (`content`, `error_detail`, `accountEmail`, `to_number`, `from_number`) NEVER persisted to any audit row

### Item #3 — needs_reauth visibility

- Incident covered: 2026-05-28 UTC; polling silently skipped founder for 18+ hours; only discovered via psql
- Regression tests:
  - `src/workers/gmail-poll.test.ts` → `needs_reauth visibility (Phase 3G.1 item #3)` (4 tests)
  - `src/workers/needs-reauth-boot-check.test.ts` → `findUsersNeedingReauth (Phase 3G.1 item #3)` (6 tests)
- Cycle attr: `users_needs_reauth` distinct from `users_skipped`
- Boot WARN: `fomo.poll.needs_reauth_at_boot` per user with `needs_reauth=true`
- Founder directive enforced: uses the SAME active-user set the polling worker uses (cursorStore.listUserIds()), not a broader oauth_tokens scan

### Item #10 — memory_signals snapshot at boot

- Incident covered: 2026-05-29 01:00 UTC; stop_active=true from 2026-05-28 survived silently into next day; lost ~10 min discovering via psql
- Regression tests:
  - `src/workers/memory-signals-boot-snapshot.test.ts` → `snapshotMemorySignalsForBoot (Phase 3G.1 item #10)` (8 tests)
- Boot event: `fomo.memory_signals.snapshot_at_boot` with one entry per active signal
- Each entry surfaces: `kind`, `scope_key`, `age_seconds`, `source`, `confidence`, `active_flag`
- Privacy invariant proven: raw `detail` body NEVER logged (test asserts canary string not present)

---

## 4. Optional founder observations (live fault-injection from §2 of the runbook)

| Item | Did the new behavior show up as expected? | Notes |
|---|---|---|
| #10 boot snapshot | _<yes / no>_ | _<paste relevant log line>_ |
| #3 needs_reauth WARN | _<yes / no>_ | _<paste relevant log line>_ |
| #2 OPTED_OUT (live SendBlue) | _<yes / skipped — proven by unit tests only / no>_ | _<paste audit row if exercised live>_ |
| #1 migration fail-loud | _<yes via unit tests / live tested — paste behavior>_ | _<note>_ |

---

## 5. Clean-stop confirmation (carry-over from 3G)

- [ ] `curl -X POST /sendblue/inbound` → HTTP 404 with FOMO_SENDBLUE_INBOUND_ENABLED=false
- [ ] `curl -X POST /slack/interactivity` → HTTP 404 with FOMO_SLACK_REVIEW_ENABLED=false

---

## 6. Verdict

☐ **PASS** — every static check green, every per-item regression test cited
above is in `pass <NNN>` count, no leaks, clean stop confirmed. **v0.5 friend
beta gate may open.**

☐ **FAIL** — list failures below; v0.5 blocked until a re-run reaches PASS.

Failures / followups:

- _…_

---

## 7. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_

---

## 8. What 3G.1 PASS does NOT promise

- Items #4 / #5 / #6 / #7 / #8 / #9 / #11 from the hardening catalog
  (post-v0.5 ops sprint candidates)
- Friend beta privacy copy / signed friend webhooks (v0.5)
- Snooze resurface scheduler (v0.3+)
- Auto-send (its own gate after v0.5)
- Multi-tenant scale, operational dashboards, queue retries
