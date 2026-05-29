# Phase 3G.1 Smoke Test Report — Production Hardening

> Filled after running every step in
> [`smoke-test-3g1-production-hardening.md`](smoke-test-3g1-production-hardening.md).
> v0.5 friend beta gate opens when this report lands on `main`.

---

**Founder:** Galiette Mita
**Run date:** 2026-05-29 18:42 UTC
**Branch:** `phase3g1-production-hardening`
**Commit SHA at run time:** `4672f01169ff6a3681a51c3fbd098bd6a2cf7f9f`

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_3G.md` on `main` with `VERDICT: PASS` (v0.1 done; merged 2026-05-29 via PR #35 at `f988551f`)
- [x] Branch `phase3g1-production-hardening` checked out with the four 3G.1 items implemented

---

## 2. Static gate

| Check | Command | Expected | Got |
|---|---|---|---|
| Preflight (empty env) | `env -i pnpm run preflight:3g1` | fails-loud with 3 named errors | ✓ DATABASE_URL / BREVIO_TOKEN_KEK / FOMO_FOUNDER_USER_ID named |
| Build | `pnpm run build` | clean (no TS errors) | ✓ clean |
| Lint | `pnpm run lint` | clean (no eslint output) | ✓ clean |
| Tests | `pnpm run test` | `pass 900+` | ✓ **pass 900 / fail 0 / skipped 0** (262 suites, 3.88s) |
| Evidence | `pnpm run smoke-evidence:3g1` | `VERDICT: PASS` | ✓ `VERDICT: PASS` on 6 static checks |
| CI | GitHub Actions `build + test` on PR #36 | green on the head commit | ✓ SUCCESS at 2026-05-29 18:42 UTC on `4672f011` |

Final lines verbatim:

```
test:     ℹ tests 900 / ℹ suites 262 / ℹ pass 900 / ℹ fail 0 / ℹ skipped 0 / ℹ duration_ms 3877
evidence: VERDICT: PASS  (run `pnpm --filter @brevio/fomo run test` to confirm regression tests are green)
preflight: ✖ 3 required check(s) failed (DATABASE_URL / BREVIO_TOKEN_KEK / FOMO_FOUNDER_USER_ID)
```

---

## 3. Per-item PASS evidence (LOAD-BEARING)

Each item cites the specific regression test that proves the fix.

### Item #1 — Neon migration verification at boot

- **Incident covered:** 2026-05-28 01:06 UTC; first 3 `/sendblue/inbound` POSTs returned HTTP 500 because `0004_inbound_replies` was PGlite-applied but not Neon-applied. Same shape hit 3D.2 with `alerts` table.
- **Regression tests** (all in `src/db/migration-verifier.test.ts`):
  - `incident reproduction (2026-05-28 inbound_replies missing on Neon)` — 2 tests: `verifyMigrations returns ok=false with inbound_replies named as missing` + `verifyMigrationsOrThrow throws PendingMigrationsError with the named table + migration file`
  - `incident reproduction (multiple missing tables — generalized fault)` — 2 tests: every missing table named with its source migration; PendingMigrationsError message lists every missing table on its own line
  - `READ-ONLY invariant` — 1 test: `verifyMigrations does not write any row to any required table`
  - `REQUIRED_TABLES is frozen and lists every public-schema table the runtime depends on` (13 tables, cross-checked against `gated-pg.test.ts`)
- **Migration command:** `pnpm --filter @brevio/fomo run migrate:neon` (wired in `apps/fomo/package.json`)
- **Policy:** fail-loud everywhere; no auto-apply; no env bypass (founder-locked 2026-05-29)

### Item #2 — SendBlue OPTED_OUT decoding + local `stop_active` sync

- **Incident covered:** 2026-05-29 01:12 UTC; alert `d9728e57…` failed with `client_error: HTTP 400` and no usable detail. Root cause: carrier-level opt-out had drifted from local `stop_active`.
- **Regression tests:**
  - `src/adapters/sendblue/client.test.ts` → `400 response-body decoder (3G.1 item #2)` — 8 tests including `surfaces OPTED_OUT into providerError.error_message/error_reason/error_code` + `omits error_detail, content, accountEmail, and any non-allowlisted field from providerError` (canary-string privacy invariant) + `drops over-length values` + `non-string non-number error_code is dropped, not coerced` + `outcome remains Object.frozen including the nested providerError`
  - `src/workers/outbound-sender.test.ts` → `OPTED_OUT drift detection (Phase 3G.1 item #2)` — 5 tests including `emits fomo.send.opt_out_drift_detected with named provider fields when SendBlue returns OPTED_OUT` + `writes stop_active=true with source=opt_out_drift_carrier so the next cycle short-circuits` + `STILL emits fomo.send.failed with the provider error fields surfaced (no double-state-transition)` + `does NOT leak error_detail, content, accountEmail, or any non-allowlisted field into ANY audit row` + `does NOT emit opt_out_drift_detected for non-OPTED_OUT 400s`
- **New audit action:** `fomo.send.opt_out_drift_detected` allowlisted in `FOMO_AUDIT_ACTIONS` (new runtime const for static verification by ops tooling)
- **New memory-signal source:** `opt_out_drift_carrier` allowlisted in `MEMORY_SIGNAL_SOURCES`; `defaultConfidence = 1.0` (carrier authoritatively declined)
- **Privacy invariant proven:** raw response body (`content`, `error_detail`, `accountEmail`, `to_number`, `from_number`) NEVER persisted to any audit row; tested with canary string `CANARY-PAYLOAD-MUST-NOT-LEAK-12345` planted in mocked response body and verified absent across every audit detail dump

### Item #3 — needs_reauth visibility

- **Incident covered:** 2026-05-28 UTC; polling silently skipped founder for 18+ hours because `needs_reauth=true`; discovered only via manual psql query.
- **Regression tests:**
  - `src/workers/gmail-poll.test.ts` → `needs_reauth visibility (Phase 3G.1 item #3)` — 4 tests including `surfaces users_needs_reauth as a distinct count, not buried in users_skipped only` + `users_needs_reauth is zero when needs_reauth is false but user is otherwise skipped (e.g. no cursor)` + `users_needs_reauth and users_polled are mutually exclusive per user` + `cycle audit detail includes users_needs_reauth so operators can grep without parsing outcomes`
  - `src/workers/needs-reauth-boot-check.test.ts` → `findUsersNeedingReauth (Phase 3G.1 item #3)` — 6 tests including `returns one finding when the active user has needs_reauth=true (incident reproduction)` + `uses the cursorStore active-user set, not a broader token scan (founder directive)` (asserts orphan token row with no cursor is NOT surfaced) + `surfaces every user with needs_reauth=true when multiple cursors exist` + frozen return value
- **Surfaces:** distinct `users_needs_reauth` count on `fomo.poll.cycle` log event + on `gmail.poll.cycle` audit detail; `fomo.poll.needs_reauth_at_boot` WARN at boot per user with `needs_reauth=true`
- **Founder directive enforced:** boot check uses the SAME active-user set the polling worker iterates (`cursorStore.listUserIds()`), not a broader oauth_tokens scan

### Item #10 — memory_signals snapshot at boot

- **Incident covered:** 2026-05-29 01:00 UTC; `stop_active=true` from 2026-05-28 survived silently into the next day; lost ~10 min discovering via manual psql query.
- **Regression tests** (all in `src/workers/memory-signals-boot-snapshot.test.ts` → `snapshotMemorySignalsForBoot (Phase 3G.1 item #10)`):
  - `surfaces stop_active=true with named-safe fields only (incident reproduction)` — pins a deterministic 18-hour age via injected `now()` and confirms the exact shape from the original incident
  - `does NOT include the raw detail body in the snapshot (privacy invariant)` — plants canary string `DO-NOT-LEAK-INTO-BOOT-SNAPSHOT-12345` in `detail` and asserts it never reaches the snapshot
  - `prunes signals below the confidence threshold (default 0.5)`
  - `respects a caller-overridden confidence threshold`
  - `orders entries newest-first so the most recent change appears at the top`
  - `active_flag is null for kinds that do NOT carry active/inactive semantics`
  - `return value and each entry are frozen`
  - `returns empty when the user has no memory signals`
- **Boot event:** `fomo.memory_signals.snapshot_at_boot` INFO scoped to `FOMO_FOUNDER_USER_ID`, one entry per active signal
- **Named-safe fields per entry:** `kind`, `scope_key`, `age_seconds`, `source`, `confidence`, `active_flag` (boolean projection of `detail.active` ONLY for kinds in `KINDS_WITH_ACTIVE_FLAG` — currently `stop_active` only)
- **Privacy invariant proven:** raw `detail` body NEVER logged; tested with canary-string regression

---

## 4. Optional founder observations (live fault-injection from §2 of the runbook)

| Item | Did the new behavior show up as expected? | Notes |
|---|---|---|
| #10 boot snapshot | skipped — proven by 8 unit tests + privacy canary invariant | Live observation can be done post-merge against prod Neon at the next dev-server restart |
| #3 needs_reauth WARN | skipped — proven by 10 unit tests + cursorStore-scoping directive assertion | Live observation post-merge: `needs_reauth=true` is the current state on Neon (carryover from 3F.2/3G); first `pnpm dev` post-merge will surface the WARN |
| #2 OPTED_OUT (live SendBlue) | skipped — proven by 13 unit tests including raw-body privacy canary | Live observation requires intentional STOP → SQL-clear-stop_active → trigger alert; not exercised in this gate to keep founder phone unblocked for v0.5 |
| #1 migration fail-loud | proven by unit tests against PGlite (not real Neon, per founder directive 2026-05-29) | Live observation passive — every `pnpm dev` boot post-merge will run the verifier; current Neon has all 13 tables so no fail-loud expected |

Per the founder directive 2026-05-29 "Do not drop real Neon production tables for fault injection", §2 of the runbook scenarios are optional and §3 of this report cites the regression tests as the PASS evidence. The four items together add 39 new tests, every one mapped to a real dated incident from the 3F.2/3G runs.

---

## 5. Clean-stop confirmation (carry-over from 3G)

3G.1 doesn't change the kill-switch surface. Clean-stop was last verified during the 3G gate on 2026-05-29 (commit `8758567a`, see `docs/SMOKE_REPORT_3G.md` §9). The relevant `fomo.*.disabled` boot lines + 404 curl behavior are unchanged in 3G.1.

- [x] Carry-over from 3G — no kill-switch surface changes in 3G.1

---

## 6. Verdict

**[x] PASS** — every static check green, every per-item regression test cited above is in the `pass 900` count, no leaks (proven by canary-string privacy invariants on items #2 and #10), CI green on `4672f011`. **v0.5 friend beta gate may open.**

[ ] FAIL

Failures / followups:

- **None blocking PASS.** Items #4 / #5 / #6 / #7 / #8 / #9 / #11 from the production-hardening catalog (`project_3g1-production-hardening-candidates` memory) remain deferred to a post-v0.5 ops sprint per the locked scope. None of them blocks a friend tester from completing the v0.5 happy path.

---

## 7. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-05-29

---

## 8. What 3G.1 PASS does NOT promise

- Items #4 / #5 / #6 / #7 / #8 / #9 / #11 from the hardening catalog (post-v0.5 ops sprint candidates)
- Friend beta privacy copy / signed friend webhooks (v0.5 scope, gated by its own 6-question pre-phase confirmation)
- Snooze resurface scheduler (v0.3+)
- Auto-send (its own gate after v0.5 stability)
- Multi-tenant scale, operational dashboards, queue retries
- Live OPTED_OUT exercise against real SendBlue (proven by adapter + worker unit tests + the privacy-invariant canary; live verification deferred to ops sprint per founder directive — keeps founder phone unblocked for v0.5)
