# Phase v0.5.5 Smoke Test Report — STOP Enforcement + Confirmation

> Filled after running every step in `smoke-test-v0.5.5-stop-enforcement.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.5.md` once **`VERDICT: PASS`** on ALL FIVE evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5) AND the cross-tenant baseline diff in §7 shows no non-founder writes.
>
> **Scaffolding-vs-runtime note:** if this report is being filled before the runtime implementation commit lands, `smoke-evidence:v0.5.5` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch and the §6 test sequence has been run end-to-end.
>
> **v0.5.5 PASS does NOT auto-unlock v1.0, friend C, auto-send, or any other phase.** The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-04 23:22 EDT (smoke start) → 2026-06-04 23:50 EDT (smoke end) → 2026-06-04 finalized as FAIL
**Branch:** `phase-v0.5.5-stop-enforcement`
**Scaffolding commit SHA:** `45c19bceb0538e47999cfda6b8de35ff530751ca`
**Runtime commit SHA:** `6f4cf392eb9e7b53b3d76c2f3d016bb529dddd82`
**Smoke window override (if any):** `FOMO_V0_5_5_WINDOW_HOURS=24` (default)
**Founder iPhone last 4 (for traceability — last 4 only):** 3459
**SendBlue from-number used:** +1 (214) 354-7196 (shared sandbox tier)

> **HEADLINE — VERDICT: FAIL (external blocker).** The v0.5.5 runtime is **verified working** by audit-log evidence (correct canonical wording attempted, correct timing, correct Q6 no-retry behavior, correct cross-tenant isolation). **But Test 1's success-path criteria (C3 / C8 / C11) cannot be exercised on the current SendBlue free/sandbox tier**, which re-flags the founder phone as `OPTED_OUT` the instant a STOP arrives and blocks ALL subsequent outbound — including the legally-customary unsubscribe-confirmation iMessage. Per founder decision 2026-06-04, the SendBlue-tier unblock question is routed to a fresh 6-question gate before any v0.5.5 retry. No §16 in this report.

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_v0.5.4.md` on `main` with `VERDICT: PASS` (PR #40 / commit `efa219a3`)
- [x] No friend involvement this phase (three-friend cap holds; Friend B was the last GUARANTEED smoke)
- [x] ngrok still healthy + forwarding the v0.5.4 static URL to localhost:8080 (pid 53193, `unshivering-interaulic-beatriz.ngrok-free.dev`)
- [x] SendBlue Sandbox tier active for `SENDBLUE_FROM_NUMBER` (account `orbitai-labs`, **shared** tier — see §12 blocker)
- [x] §1 baseline snapshot captured into `/tmp/v0.5.5-baseline-stop-active.txt` BEFORE smoke start
- [x] Founder iPhone is the device sending STOP/START and receiving confirmations (last 4: 3459)

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_5_BASELINE_CONFIRMED` | ☑ | set `true` after §1 capture (baseline written to `/tmp/v0.5.5-baseline-stop-active.txt`) |
| `FOMO_V0_5_5_WINDOW_HOURS` | ☑ | `24` (default) |

All other v0.5.4 env vars unchanged (no new requirements).

## 3. PASS criteria (12 — STOP Enforcement + Confirmation)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | All 4 v0.5.5-NEW audit actions registered in `FOMO_AUDIT_ACTIONS` | preflight ✓ passed clean | ☑ |
| C2 | Alert-creation short-circuit fires when `stop_active=true` | covered by C6 poll-suppression evidence (2 `poll.skipped_stop_active` rows in window). Founder-only smoke, no synthetic email after STOP to test alert-creation path specifically. | ☑ (poll path) / N/A (alert-creation path, no email after STOP) |
| C3 | STOP confirmation reply sent on inbound STOP | ❌ **BLOCKED EXTERNAL**: SendBlue free-tier returned `OPTED_OUT` (error_code 402, http 400) on all 3 STOP attempts. Runtime fired the send call with correct canonical body (visible in SendBlue dashboard as "Failed to send"); provider refused. | ❌ blocked (see §12) |
| C4 | Idempotency — duplicate STOP within 24h does NOT re-send confirmation | Cannot exercise without a successful `_sent`. Logic is unit-tested in repo (`STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS = 24 * 60 * 60 * 1000` + `withinIdempotencyWindow` guard in [apps/fomo/src/routes/sendblue-inbound.ts:582-623](apps/fomo/src/routes/sendblue-inbound.ts#L582-L623)). | ⚠ runtime-verified at code level; integration-blocked |
| C5 | START re-enables alerts | Not exercised — Test 3 (START + Gmail send + Slack approve) skipped after C3 blocker. `start_recorded` evidence ✓ (2 rows). | ⚠ partial (start_recorded ✓, Slack approve path untested) |
| C6 | Polling-after-STOP suppression | 2 `fomo.poll.skipped_stop_active` audit rows during the smoke window — polling continued (cycle_number incremented past 20), zero alerts created for founder while `stop_active=true`. | ☑ |
| C7 | Cross-tenant isolation | Manual baseline-vs-post diff: **only founder's row changed**; Morris (`25c1a707…`), gm3258 (`4606e1e7…`), Sheila residual (`8fbead5c…`) byte-identical. See §9. (Evidence-script flagged C7 as FAIL due to its 24h window including Sheila's *pre-smoke* timestamp `2026-06-04 14:11`; that's a script-window-slide false positive, not a real regression.) | ☑ (manual diff) / ✗ (script — false positive) |
| C8 | Confirmation wording deterministic + friendly | SendBlue dashboard shows the exact attempted body **"You're unsubscribed from Brevio. Text START to turn it back on."** matching the `STOP_CONFIRMATION_BODY` constant in [apps/fomo/src/routes/sendblue-inbound.ts:582](apps/fomo/src/routes/sendblue-inbound.ts#L582). Cannot reach the `detail.message_preview` field of a `_sent` audit row because no `_sent` exists. | ⚠ code-level ✓, integration-blocked |
| C9 | STOP confirmation contains zero email-content leakage | Leak-canary scan over 3 `stop_confirmation_failed` audit rows + 4 `stop_recorded` rows + 6 `inbound_received` rows: **zero hits** across `brevio-canary-`, `Subject:`, `From:`, `@gmail.com`. (Evidence-script returned PASS even with no `_sent` rows.) | ☑ |
| C10 | Failure-mode handled per Q6 — best-effort audit, no retry | **3 `fomo.sendblue.stop_confirmation_failed` audit rows; zero retry-violations** (no `_sent` followed any `_failed` for the same actor in window). Runtime correctly implements Q6 behavior. *Note: the failures were real OPTED_OUT, not the induced-broken-key path from §6 Test 4 — Test 4 was not separately run because the real failure produced identical evidence.* | ☑ |
| C11 | Founder regression — founder STOP triggered a confirmation to founder phone | Same blocker as C3. `stop_recorded` for `actor_user_id=founder` confirmed (3 rows); `_sent` did not fire due to SendBlue OPTED_OUT. | ❌ blocked (see §12) |
| C12 | All prior smoke-evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4) still PASS after v0.5.5 changes | v0.5.1 ✓, v0.5.2 ✓ (with `FOMO_V0_5_2_WINDOW_HOURS=168` per runbook §7), v0.5.3 ✓, v0.5.4 ✗ (window-slide: v0.5.5's own writes now fall inside v0.5.4's wall-clock window, triggering "row updated within window" false positive for Morris + founder). Per-script breakdown in §4–§8. | ⚠ 3/4 PASS; v0.5.4 fails on script window semantics, not a real regression |

## 4. `smoke-evidence:v0.5.1` output (substrate) — **VERDICT: PASS**

```
[✓] Migrations + columns up to date on live Neon
      14 tables + 7 required columns present
[✓] fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS
[✓] MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)
[✓] Two-user synthetic smoke — friend(s) provisioned in users table (3 friend rows)
[✓] invite_tokens lifecycle (issue → consume) — issued=10, consumed=5
[✓] fomo.onboard.invite_issued audit row (8 issued)
[✓] fomo.onboard.user_created audit row (4 created)
[✓] Per-friend STOP isolation — 3 friend STOP events; 4 founder STOP events
[✓] memory_signals.stop_active per-user — friend_rows=3
[✓] Founder flow regression — 5 recent approved→sent
[✓] No raw phone / canary leakage — scanned 500 audit + 6 memory rows; zero hits

VERDICT: PASS
```

## 5. `smoke-evidence:v0.5.2` output (with `FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: PASS**

```
[✓] Briefing recorded on real-phone invite — 2 invite_issued audit rows
[✓] Real friend onboarded with phone hash — 3 friend users
[✓] Invite token consumed — 3 invites
[✓] Founder approval → real iMessage delivered — 2 successful sends
[✓] Friend STOP captured from real iMessage thread — 2 STOPs
[✓] memory_signals.stop_active per-user — 3 friend rows
[✓] Founder regression — 3 approved→sent in window
[✓] Leak-canary scan — scanned 2000 audit + 6 memory + 66 transition rows; zero hits

VERDICT: PASS
```

> Window was widened from default 24h to 168h per runbook §7 known issue (v0.5.2 data is ~4 days old as of v0.5.5 run).

## 6. `smoke-evidence:v0.5.3` output (hardening still wired) — **VERDICT: PASS**

```
[✓] All 7 v0.5.3 audit actions registered in FOMO_AUDIT_ACTIONS
[✓] 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
[✓] Item #1: SendBlue contact auto-registration (1 audit row)
[✓] Item #2: OAuth auto-refresh fired (13 refresh audit rows)
[✓] Item #3: pg pool error handler (0 — clean uptime)
[✓] Item #4: SendBlue reconciliation (0 gap rows — webhook delivery healthy)
[✓] sendblue_contact_status row for friend in window (1 row)
[✓] Leak-canary scan — 989 audit rows; zero hits

VERDICT: PASS
```

## 7. `smoke-evidence:v0.5.4` output (with `FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: FAIL (script-window false positives)**

```
[✓] C1–C12: all PASS (registry, two-friend lifecycle, friend-safe Slack card, sends, STOPs, isolation, no leak, no escalation, hardening intact)
[✗] C13: Morris's stop_active row UNTOUCHED — 1 Morris row updated within window
[✗] C14: Founder's stop_active row UNTOUCHED — 1 founder row updated within window
[✓] C15: Distinct sendblue_contact_status rows per friend
[✓] C16: v0.5.3 hardening still functional

VERDICT: FAIL — 2 criteria failed.
```

> **Both failures are evidence-script window-slide false positives, not real regressions.** C13 fires because Morris was STOP'd during the v0.5.4 smoke itself (which was *yesterday*, inside the 168h window). C14 fires because *today's v0.5.5 smoke* wrote to the founder's `stop_active` row, which the v0.5.4 script can't distinguish from "Friend B wrote to founder's keyspace" (the actual cross-tenant attack it's designed to catch). The v0.5.4 smoke originally landed `VERDICT: PASS` on 2026-06-04 (PR #40); rerunning today within the same wall-clock window cannot reproduce that PASS because the evidence script lacks a smoke-start cutoff. Followup: evidence-script semantics improvement candidate.

## 8. `smoke-evidence:v0.5.5` output (STOP enforcement proof) — **VERDICT: FAIL**

```
[✗] C3: STOP confirmation reply sent — No stop_confirmation_sent audit row in window.
[✓] C4: Idempotency — no actor has >1 stop_confirmation_sent (0 total)
[!] C5: START re-enables alerts — WARN: no alert created for actor after START (Test 3 not exercised)
[✓] C6: Polling-after-STOP suppression — 2 poll-skipped audit rows in window
[✗] C7: Cross-tenant isolation — non-founder user_id(s) updated in window: 8fbead5c…
[!] C8: Confirmation wording — WARN: canonical phrases not detected in 0 confirmation previews
[✓] C9: STOP confirmation contains zero email-content leakage
[✓] C10: Failure-mode handled — 3 failure audit rows; zero retry-violations
[✗] C11: Founder regression — No stop_confirmation_sent row for actor_user_id='founder'
[!] C12: All prior smoke-evidence scripts — operator must verify

VERDICT: FAIL — at least one criterion failed.
```

> **Interpretation:**
> - **C3, C11**: Real failures, external blocker — SendBlue OPTED_OUT on the shared sandbox tier. Unblock routed to next 6Q gate (see §12).
> - **C8**: WARN because no `_sent` rows exist to scan; canonical wording was attempted (SendBlue dashboard "Failed to send" shows exact body).
> - **C7**: False positive — script's 24h window includes Sheila residual's `updated_at = 2026-06-04 14:11:51` (which is ~13h before smoke start). Sheila's row was NOT written during v0.5.5 smoke — see §9 manual diff.
> - **C5**: Not exercised (Test 3 skipped after C3 blocker).
> - **C12**: Operator-confirmed in §4-§7 above (3/4 PASS; v0.5.4 false positive).

## 9. Cross-tenant baseline diff (THE v0.5.5 LOAD-BEARING SECTION)

### §9.A `stop_active` baseline (captured §1, BEFORE smoke):

```
               user_id                |    kind     |                    detail                     |     source     |          updated_at           
--------------------------------------+-------------+-----------------------------------------------+----------------+-------------------------------
 25c1a707-811a-48a8-8ef7-fd1008057c89 | stop_active | {                                            +| user_confirmed | 2026-06-01 22:04:04.182916+00
                                      |             |     "active": true,                          +|                | 
                                      |             |     "recorded_at": "2026-06-01T22:04:04.052Z"+|                | 
                                      |             | }                                             |                | 
 4606e1e7-7cc0-4ce4-b4e9-0b67a4d38941 | stop_active | {                                            +| user_confirmed | 2026-05-30 02:22:53.869586+00
                                      |             |     "active": true,                          +|                | 
                                      |             |     "recorded_at": "2026-05-30T02:22:53.869Z"+|                | 
                                      |             | }                                             |                | 
 8fbead5c-68b6-49fc-bf55-2bf9174c2e01 | stop_active | {                                            +| user_confirmed | 2026-06-04 14:11:51.674982+00
                                      |             |     "active": true,                          +|                | 
                                      |             |     "recorded_at": "2026-06-04T14:11:51.639Z"+|                | 
                                      |             | }                                             |                | 
 founder                              | stop_active | {                                            +| user_confirmed | 2026-05-29 01:15:54.023+00
                                      |             |     "active": false,                         +|                | 
                                      |             |     "recorded_at": "2026-05-29T01:15:54.023Z"+|                | 
                                      |             | }                                             |                | 
(4 rows)
```

### §9.B `stop_active` post-smoke:

```
               user_id                |    kind     |                    detail                     |     source     |          updated_at           
--------------------------------------+-------------+-----------------------------------------------+----------------+-------------------------------
 25c1a707-811a-48a8-8ef7-fd1008057c89 | stop_active | { "active": true, "recorded_at": "2026-06-01T22:04:04.052Z" } | user_confirmed | 2026-06-01 22:04:04.182916+00
 4606e1e7-7cc0-4ce4-b4e9-0b67a4d38941 | stop_active | { "active": true, "recorded_at": "2026-05-30T02:22:53.869Z" } | user_confirmed | 2026-05-30 02:22:53.869586+00
 8fbead5c-68b6-49fc-bf55-2bf9174c2e01 | stop_active | { "active": true, "recorded_at": "2026-06-04T14:11:51.639Z" } | user_confirmed | 2026-06-04 14:11:51.674982+00
 founder                              | stop_active | { "active": true, "recorded_at": "2026-06-05T03:43:01.363Z" } | user_confirmed | 2026-06-05 03:43:01.385+00
(4 rows)
```

### §9.C diff (THE load-bearing cross-tenant check — PASS):

```
15,17c15,17
<  founder | stop_active | { "active": false, "recorded_at": "2026-05-29T01:15:54.023Z" } | user_confirmed | 2026-05-29 01:15:54.023+00
---
>  founder | stop_active | { "active": true,  "recorded_at": "2026-06-05T03:43:01.363Z" } | user_confirmed | 2026-06-05 03:43:01.385+00
```

**ONLY the founder's row changed.** Morris, gm3258, Sheila residual are byte-identical to baseline. The v0.5.5 evidence script's C7 FAIL is a window-slide false positive (Sheila's `updated_at` happens to fall within the script's 24h window but was set ~13h before smoke start).

**Expected diff shape:**
- The founder's `stop_active` row was modified (updated_at + detail). Final detail JSON depends on the last test run: if §6 Test 3 (START) was last, `active=false`; if §6 Test 4 (induced failure) was last, `active=true` and possibly NO `stop_confirmation_sent_at`.
- ZERO modifications to Morris's row (`25c1a707-…`). updated_at unchanged from `2026-06-01 22:04:04.182916+00`.
- ZERO modifications to gm3258's row (`4606e1e7-…`).
- ZERO modifications to Sheila's residual row (`8fbead5c-…`). updated_at unchanged from `2026-06-04 14:11:51.674982+00`.
- Row count UNCHANGED (founder's row already existed; v0.5.5 updates it in place, not adds it).

**Operator confirms:** ☐ Diff shows only founder's row changed; Morris, gm3258, Sheila residual all byte-identical between baseline and post.

## 10. Operator-confirmed visual + iMessage checks

| Check | Confirmed? | Notes |
|---|---|---|
| Test 1: Founder iPhone received STOP confirmation iMessage | N/A | Blocked — SendBlue OPTED_OUT refused outbound. Dashboard shows "Failed to send" for the attempt. |
| Test 1: Confirmation wording contained "You're unsubscribed" + "Text START" | N/A | Blocked — no `_sent`. Canonical body **"You're unsubscribed from Brevio. Text START to turn it back on."** matched in SendBlue dashboard payload preview against the `STOP_CONFIRMATION_BODY` source constant. |
| Test 1: Confirmation contained NO email body / sender / subject | N/A | Blocked — no `_sent`. Code-path inspection + C9 leak-scan over `_failed` rows show zero leakage. |
| Test 2: Second STOP within 24h → founder iPhone did NOT receive a second confirmation | N/A | Idempotency cannot be exercised without a successful first `_sent`. Code-level verified ([apps/fomo/src/routes/sendblue-inbound.ts:582-623](apps/fomo/src/routes/sendblue-inbound.ts#L582-L623)). |
| Test 3: After START, a new FOMO alert was created for founder (Slack card appeared) | N/A | Test 3 skipped after C3 external blocker. `start_recorded` audit ✓ (2 rows). |
| Test 4: Induced SendBlue failure → no confirmation iMessage; `_failed` audit row in dev log | ☑ | Real OPTED_OUT failure produced 3 `stop_confirmation_failed` audit rows — identical evidence shape to the induced-broken-key path. Test 4 not separately re-run. |
| Test 4: No retry happened (no `_sent` row followed `_failed` for the same actor) | ☑ | C10 ✓ — zero retry-violations across 3 `_failed` rows. |
| Test 5: Morris row untouched in `stop_active` table | ☑ | §9.C diff — Morris (`25c1a707…`) byte-identical to baseline. |
| Test 5: gm3258 row untouched | ☑ | §9.C diff — gm3258 (`4606e1e7…`) byte-identical to baseline. |
| Test 5: Sheila residual row untouched | ☑ | §9.C diff — Sheila residual (`8fbead5c…`) byte-identical to baseline. |

## 11. Founder observations

| Observation | Note |
|---|---|
| Did Test 1's confirmation feel friendly or robotic? (addresses Sheila's §10 feedback in spirit) | Not exercised — external blocker. Deferred to post-unblock retry. |
| Did Test 2's idempotency feel obvious to a user, or would the silence be confusing? | Not exercised — depends on a successful first `_sent`. Deferred. |
| Did Test 3's START round-trip feel symmetric? (any reason to send a "you're re-enabled" confirmation too?) | Not exercised — Test 3 skipped after C3 blocker. Deferred. |
| Did Test 4's failure-mode (silent on failure, audit only) feel right, or is a retry needed? | Real OPTED_OUT failure was clean: 3 `_failed` audit rows, no user-visible noise, no retry. Q6 behavior felt right — failure-mode is fine as-is. |
| Did the §9 cross-tenant diff show any unexpected row movement? | No — only founder's row changed. Morris / gm3258 / Sheila residual byte-identical. Isolation held. |
| Anything in audit_log that surprised you? | The runtime correctly attempts the send and surfaces the provider refusal via `_failed` — exactly the Q6 best-effort/no-retry shape. The surprise was upstream (SendBlue tier), not in the substrate. |
| What would you want different before either: (a) shipping the iMessage tone rewrite (B1), or (b) starting Personalized Importance Learning substrate (C1)? | First answer the SendBlue tier question via a fresh 6Q gate. Neither (a) nor (b) is the right next step until the STOP-confirmation pathway is provably deliverable end-to-end. |

## 12. Verdict

☐ **PASS** — all 12 criteria green; §9 cross-tenant diff shows no non-founder writes; all 5 evidence scripts `VERDICT: PASS`; operator visual + iMessage checks confirmed. **Next phase runs its own 6-question gate.**

☑ **FAIL (external blocker)** — runtime is verified working at code + audit level; C3 / C8 / C11 cannot be exercised on the current SendBlue free/sandbox tier because the provider re-flags the founder phone as `OPTED_OUT` the instant a STOP arrives, blocking the unsubscribe-confirmation outbound itself. Not a code regression. Per founder decision 2026-06-04, the SendBlue-tier unblock question is routed to a **fresh 6-question gate before any v0.5.5 retry**. No §16 unblock-paths section in this report by design — the unblock direction is the next gate's job.

Failures / followups:

- **F1 (blocker, external):** SendBlue shared sandbox tier returns `OPTED_OUT` (err 402 / http 400) on the unsubscribe-confirmation outbound itself. Unblock options (dedicated number / tier upgrade / mock-provider for smoke / alternate carrier) → **next 6Q gate**.
- **F2 (script semantics, non-blocking):** `smoke-evidence:v0.5.4` flags Morris's + founder's `stop_active` row as "updated within window" when re-run today (window-slide false positive — see §7). v0.5.4 originally landed PASS on 2026-06-04 (PR #40); manual §9 diff confirms no real regression. Followup: add a smoke-start cutoff to evidence-script semantics.
- **F3 (script semantics, non-blocking):** `smoke-evidence:v0.5.5` C7 flags Sheila residual as "non-founder updated in window" — Sheila's `updated_at` was ~13h before smoke start. Same window-slide class as F2.
- **F4 (followup, deferred):** Idempotency (C4) and START re-enable (C5) are code-level verified but integration-blocked behind F1. Both retry once F1 unblocks.

**v0.5.5 PASS does NOT auto-unlock v1.0, Friend C, auto-send, or any other phase.** Equally: **v0.5.5 FAIL does NOT auto-trigger a v0.5.5 retry, a SendBlue tier upgrade, or any specific unblock direction.** The next phase is decided at the next 6-question gate.

## 13. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-04
- No friend consent needed this phase (founder-only smoke)
- Verdict acknowledged: **FAIL (external blocker)**; unblock direction deferred to next 6Q gate

## 14. Aftercare confirmation

- [x] N/A — Test 4 was not separately run (real OPTED_OUT produced equivalent evidence); `SENDBLUE_API_KEY_ID` was never broken
- [ ] **Open:** founder is currently left `stop_active=true` (post-smoke state in §9.B). Resuming alerts requires either (a) sending a real START once SendBlue inbound delivery is restored, or (b) a manual `stop_active=false` write. Decision deferred to next 6Q gate alongside the SendBlue tier question.
- [x] No friend deletion ops (no friend involved)
- [x] Morris re-verified as untouched — §9.C diff shows Morris's row byte-identical between baseline (`/tmp/v0.5.5-baseline-stop-active.txt`) and post-smoke

## 15. What v0.5.5 PASS does NOT promise

v0.5.5 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- A Personalized Importance Learning substrate — its own future-phase candidate (see [docs/personalized-importance-learning.md](personalized-importance-learning.md) and [FOMO_PLAN.md §17](../FOMO_PLAN.md)); may be pulled forward before v1.0 if friend-beta false positives damage trust
- Friend C onboarding — three-friend cap; Friend C is OPTIONAL
- Auto-send — its own gate per FOMO_PLAN v0.8
- iMessage tone rewrite + summary length fix (B1) — separate candidate from Sheila's §10 feedback
- Google OAuth verification submission (B3) — multi-week external process
- A new email provider (`EmailContextProvider` abstraction per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md): Gmail is the only active provider)
- A new model provider (OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md))
- Dashboard / web UI
- Calendar / Drafting / MCP / browser automation — L2+ surfaces

The next phase is decided AT THE NEXT 6-question gate.
