# Phase v0.5.5 Smoke Test Report — STOP Enforcement + Confirmation

> Filled after running every step in `smoke-test-v0.5.5-stop-enforcement.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.5.md` once **`VERDICT: PASS`** on ALL FIVE evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5) AND the cross-tenant baseline diff in §7 shows no non-founder writes.
>
> **Scaffolding-vs-runtime note:** if this report is being filled before the runtime implementation commit lands, `smoke-evidence:v0.5.5` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch and the §6 test sequence has been run end-to-end.
>
> **v0.5.5 PASS does NOT auto-unlock v1.0, friend C, auto-send, or any other phase.** The next phase runs its own 6-question gate.

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.5-stop-enforcement`
**Scaffolding commit SHA:** _<git rev-parse — from the SCAFFOLDING commit>_
**Runtime commit SHA:** _<git rev-parse — from the RUNTIME commit; same branch>_
**Smoke window override (if any):** `FOMO_V0_5_5_WINDOW_HOURS=` _<default 24>_
**Founder iPhone last 4 (for traceability — last 4 only):** _<last 4>_
**SendBlue from-number used:** _<+1XXXXXXXXXX>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_v0.5.4.md` on `main` with `VERDICT: PASS` (PR #40 / commit `efa219a3`)
- [ ] No friend involvement this phase (three-friend cap holds; Friend B was the last GUARANTEED smoke)
- [ ] ngrok still healthy + forwarding the v0.5.4 static URL to localhost:8080
- [ ] SendBlue Sandbox tier active for `SENDBLUE_FROM_NUMBER`
- [ ] §1 baseline snapshot captured into `/tmp/v0.5.5-baseline-stop-active.txt` BEFORE smoke start
- [ ] Founder iPhone is the device sending STOP/START and receiving confirmations

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_5_BASELINE_CONFIRMED` | ☐ | `true` after §1 capture |
| `FOMO_V0_5_5_WINDOW_HOURS` | ☐ | default 24 |

All other v0.5.4 env vars unchanged (no new requirements).

## 3. PASS criteria (12 — STOP Enforcement + Confirmation)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | All 4 v0.5.5-NEW audit actions registered in `FOMO_AUDIT_ACTIONS` | `fomo.sendblue.stop_confirmation_sent`, `_failed`, `fomo.alert.suppressed_stop_active`, `fomo.poll.skipped_stop_active` present in registry | ☐ |
| C2 | Alert-creation short-circuit fires when `stop_active=true` | `fomo.alert.suppressed_stop_active` audit row exists; ZERO new alert rows for STOP'd user in smoke window | ☐ |
| C3 | STOP confirmation reply sent on inbound STOP | `fomo.sendblue.stop_confirmation_sent` audit row with `provider_status=QUEUED`; founder confirms iMessage received | ☐ |
| C4 | Idempotency — duplicate STOP within 24h does NOT re-send confirmation | Two STOPs within 24h → exactly 1 `stop_confirmation_sent` audit row in window for that actor | ☐ |
| C5 | START re-enables alerts | After START, a new alert is created for the previously-STOP'd actor when the next FOMO-worthy email arrives | ☐ |
| C6 | Polling-after-STOP suppression | `fomo.poll.skipped_stop_active` audit row(s) present; polling continued (cycle_number incremented) but `alerts_created` for the STOP'd user = 0 | ☐ |
| C7 | Cross-tenant isolation | Baseline-vs-post `stop_active` diff: only founder's row changed; Morris (`25c1a707-…`), gm3258 (`4606e1e7-…`), Sheila residual (`8fbead5c-…`) all byte-identical | ☐ |
| C8 | Confirmation wording deterministic + friendly | Canonical phrases "You're unsubscribed" + "Text START to turn Brevio back on" present in detail.message_preview; operator confirms wording visually | ☐ |
| C9 | STOP confirmation contains zero email-content leakage | Leak-canary scan over `stop_confirmation_sent` audit detail returns zero hits across `brevio-canary-`, `Subject:`, `From:`, `@gmail.com` | ☐ |
| C10 | Failure-mode handled per Q6 — best-effort audit, no retry | After §6 Test 4 induced failure: `fomo.sendblue.stop_confirmation_failed` audit row exists; NO subsequent `_sent` row follows for the same actor within the window | ☐ |
| C11 | Founder regression — founder STOP triggered a confirmation to founder phone | `fomo.sendblue.stop_confirmation_sent` audit row with `actor_user_id=<founder>`; iMessage received on founder phone; subsequent START re-enabled alerts | ☐ |
| C12 | All prior smoke-evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4) still PASS after v0.5.5 changes | 5 × `VERDICT: PASS` from chained `smoke-evidence:v0.5.{1,2,3,4,5}` | ☐ |

## 4. `smoke-evidence:v0.5.1` output (substrate)

```
<paste verbatim>
```

## 5. `smoke-evidence:v0.5.2` output (first-friend specifics still hold)

```
<paste verbatim>
```

If the wall-clock window slid past v0.5.2's last audit row, set `FOMO_V0_5_2_WINDOW_HOURS=48` (or higher) and re-run that one.

## 6. `smoke-evidence:v0.5.3` output (hardening still wired)

```
<paste verbatim>
```

## 7. `smoke-evidence:v0.5.4` output (cross-tenant proof still holds)

```
<paste verbatim>
```

## 8. `smoke-evidence:v0.5.5` output (STOP enforcement proof)

```
<paste verbatim>
```

## 9. Cross-tenant baseline diff (THE v0.5.5 LOAD-BEARING SECTION)

### §9.A `stop_active` baseline (captured §1, BEFORE smoke):

```
<paste /tmp/v0.5.5-baseline-stop-active.txt>
```

### §9.B `stop_active` post-smoke:

```
<paste /tmp/v0.5.5-post-stop-active.txt>
```

### §9.C diff:

```
<paste /tmp/v0.5.5-stop-active.diff>
```

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
| Test 1: Founder iPhone received STOP confirmation iMessage | ☐ | _<arrival time>_ |
| Test 1: Confirmation wording contained "You're unsubscribed" + "Text START" | ☐ | _<paste exact text or describe>_ |
| Test 1: Confirmation contained NO email body / sender / subject | ☐ | (cross-check with C9 evidence) |
| Test 2: Second STOP within 24h → founder iPhone did NOT receive a second confirmation | ☐ | |
| Test 3: After START, a new FOMO alert was created for founder (Slack card appeared) | ☐ | |
| Test 4: Induced SendBlue failure → no confirmation iMessage; `_failed` audit row in dev log | ☐ | |
| Test 4: No retry happened (no `_sent` row followed `_failed` for the same actor) | ☐ | (cross-check with C10 evidence) |
| Test 5: Morris row untouched in `stop_active` table | ☐ | (cross-check with §9 diff) |
| Test 5: gm3258 row untouched | ☐ | |
| Test 5: Sheila residual row untouched | ☐ | |

## 11. Founder observations

| Observation | Note |
|---|---|
| Did Test 1's confirmation feel friendly or robotic? (addresses Sheila's §10 feedback in spirit) | _<…>_ |
| Did Test 2's idempotency feel obvious to a user, or would the silence be confusing? | _<…>_ |
| Did Test 3's START round-trip feel symmetric? (any reason to send a "you're re-enabled" confirmation too?) | _<…>_ |
| Did Test 4's failure-mode (silent on failure, audit only) feel right, or is a retry needed? | _<…>_ |
| Did the §9 cross-tenant diff show any unexpected row movement? | _<…>_ |
| Anything in audit_log that surprised you? | _<…>_ |
| What would you want different before either: (a) shipping the iMessage tone rewrite (B1), or (b) starting Personalized Importance Learning substrate (C1)? | _<…>_ |

## 12. Verdict

☐ **PASS** — all 12 criteria green; §9 cross-tenant diff shows no non-founder writes; all 5 evidence scripts `VERDICT: PASS`; operator visual + iMessage checks confirmed. **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

Failures / followups:

- _…_

## 13. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 14. Aftercare confirmation

- [ ] If Test 4 left `SENDBLUE_API_KEY_ID` broken, it was restored
- [ ] If the founder is left STOP'd after the smoke and wants alerts to resume, a final START was sent
- [ ] No friend deletion ops (no friend involved)
- [ ] Morris re-verified as untouched (last check before commit; cross-reference §9.C diff)

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
