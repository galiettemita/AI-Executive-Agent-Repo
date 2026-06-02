# Phase v0.5.3 Smoke Test Report — Production Hardening

> Filled after running every step in `smoke-test-v0.5.3-production-hardening.md`.
> Committed as `docs/SMOKE_REPORT_v0.5.3.md` with all three evidence scripts
> printing `VERDICT: PASS`. **v0.5.3 PASS does NOT auto-unlock v1.0; the
> next phase runs its own 6Q gate.**

---

**Founder:** galiettemita
**Run date:** 2026-06-02 01:00 – 01:35 UTC
**Branch:** `phase-v0.5.3-production-hardening`
**Commit SHA at run time:** the chain ran against `94359255` (scaffolding HEAD); this report ships in a follow-up commit on the same branch.
**Alt-Gmail used (synthetic friend):** `messageinabottleco123@gmail.com` (throwaway Gmail created for this smoke; NOT a real friend, NOT Morris)
**Synthetic phone:** `+15550100099` (NANPA-reserved fictional range)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_v0.5.2.md` on `main` with `VERDICT: PASS` (merge commit `249ba465`)
- [x] No real friend involved — Morris stays opted out per his v0.5.2 aftercare
- [x] No SendBlue plan upgrade — Free Sandbox stays
- [x] Throwaway Gmail added to Google Cloud OAuth client as a test user
- [x] Server fresh-rebuilt from v0.5.3 source before the smoke (dist/ rebuilt; ran clean for the full smoke window)

## 2. PASS criteria (7)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Each fix has a regression test tied to the original incident | 23 new tests across 4 files (gate, refresh-helper, pool, reconcile); all green locally + 1053 total | ✅ |
| 2 | No manual SendBlue contact-add needed in the smoke | `/onboard/callback` auto-called `POST /api/v2/contacts` for `+15550100099`; SendBlue returned HTTP 200; `memory_signals.sendblue_contact_status` written with `registered: true`; `fomo.sendblue.contact_registered` audited. **Zero manual founder steps.** | ✅ |
| 3 | No manual OAuth refresh needed in the smoke | Force-expired founder's `oauth_tokens.expires_at` via psql to simulate a 1-min-past-expiry token; within ~15s the polling worker auto-called `refreshAccessToken`, saved the new token (`expires_at` jumped to ~1h in future), audited `fomo.oauth.refreshed`. **Zero manual `ops:refresh-oauth` runs.** Total 4 refresh audit rows in the smoke window. | ✅ |
| 4 | Neon connection interruption does not crash the process | Server uptime clean for the entire smoke (no `process.exit`, no `unhandled` events in `/tmp/fomo-v0.5.3.log`). pg pool error listener attached per test suite. Server PID `74970` survived all polling cycles. | ✅ |
| 5 | Missed SendBlue webhook can be detected by reconciliation | `pnpm ops:reconcile-sendblue` ran successfully. Detected 1 historical gap from v0.5.2: `615FCE9D-635D-4D9F-99D5-E725C7CEBB33` (Morris's original "Hi" iMessage that never reached our webhook during the v0.5.2 server-crash window). Audited as `fomo.sendblue.delivery_gap_detected`. **The exact v0.5.2 incident shape is now retroactively visible.** | ✅ |
| 6 | v0.5.2 smoke path still passes | `smoke-evidence:v0.5.1`, `smoke-evidence:v0.5.2` (48h window), and `smoke-evidence:v0.5.3` all printed `VERDICT: PASS`. The v0.5.2 evidence default 24h window slid past Morris's v0.5.2 briefed invite; widening to 48h via `FOMO_V0_5_2_WINDOW_HOURS=48` confirmed PASS — substrate behavior unchanged. | ✅ |
| 7 | No secrets / raw payloads / private data leaked | `smoke-evidence:v0.5.3` leak-canary scan: scanned 798 audit rows; zero hits for `BREVIO_TOKEN_KEK` material or connection strings. Per-item audit rows verified safe-only (refresh_token_rotated boolean but never refresh_token plaintext; SendBlue from_slug last-4 only; pool error message bounded to ≤200 chars). | ✅ |

## 3. `smoke-evidence:v0.5.1` output

```
[✓] Migrations + columns up to date on live Neon
      14 tables + 7 required columns present
[✓] fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS
[✓] MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)
[✓] Two-user synthetic smoke — friend(s) provisioned in users table
      3 friend row(s); founder still env-resolved (not in users)
[✓] invite_tokens lifecycle (issue → consume)
      issued=9, consumed=4 (≥1 issued + ≥1 consumed)
[✓] fomo.onboard.invite_issued audit row (≥1)
      7 issued
[✓] fomo.onboard.user_created audit row (≥1)
      3 created
[✓] Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder
      2 friend STOP event(s); 1 founder STOP event(s)
[✓] memory_signals.stop_active row exists for the friend (per-user isolation)
      friend_rows=2; user_ids=4606e1e7…, 25c1a707…
[✓] Founder flow regression — at least one approved → sent transition for founder
      4 recent approved→sent transition(s)
[✓] No raw phone / canary leakage across audit + memory_signals
      scanned 500 audit + 4 memory rows; zero hits

VERDICT: PASS
```

## 4. `smoke-evidence:v0.5.2` output (48h window)

```
[✓] Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)
      1 invite_issued audit row(s) with briefed_confirmed=true + phone_class='real'
[✓] At least one real friend onboarded with phone hash populated
      2 friend user(s) onboarded; IDs: 25c1a707…, 14a6639f…
[✓] Invite token consumed by the friend (atomic consume on OAuth success)
      2 invite(s) consumed by friend user_id(s)
[✓] Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)
      1 successful send(s) on behalf of friend(s); destination_slug last-4 only, no raw phone
[✓] Friend STOP captured from real iMessage thread (not synthetic curl)
      1 real-iMessage STOP(s) recorded for friend actor_user_id
[✓] memory_signals.stop_active row for friend (per-user keyspace)
      1 friend stop_active row(s); user_id(s): 25c1a707…
[✓] Founder regression — at least one founder approved → sent during the smoke window
      1 founder approved→sent transition(s) in the smoke window
[✓] Leak-canary scan — no forbidden substrings in persisted detail
      scanned 1220 audit + 2 memory + 22 transition rows; zero hits across 3 canary substring(s)

VERDICT: PASS  (48h window — the v0.5.2 briefed invite is just outside the default 24h)
```

## 5. `smoke-evidence:v0.5.3` output

```
[✓] All 7 v0.5.3 audit actions registered in FOMO_AUDIT_ACTIONS
      fomo.sendblue.contact_registered, fomo.sendblue.contact_registration_failed,
      fomo.send.contact_not_registered, fomo.oauth.refreshed, fomo.oauth.refresh_failed,
      fomo.db.connection_error, fomo.sendblue.delivery_gap_detected
[✓] 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
      present
[✓] Item #1: SendBlue contact auto-registration audit row present in smoke window
      1 audit row(s) — auto-registration fired
[✓] Item #2: OAuth auto-refresh fired at least once in smoke window
      4 refresh audit row(s)
[✓] Item #3: pg pool error handler best-effort audit count
      0 rows (server uptime was clean during the window — handler exists per test suite but never fired)
[✓] Item #4: SendBlue reconciliation audit count
      1 fomo.sendblue.delivery_gap_detected row(s) — reconciliation found + audited gaps
[✓] sendblue_contact_status memory_signal row written for friend onboarded in smoke window
      1 contact_status row(s) for friend(s)
[✓] Leak-canary scan: no raw secrets / connection strings in audit detail
      scanned 798 audit rows; zero hits

VERDICT: PASS
```

## 6. Operator-confirmed checks

| Check | Confirmed? | Notes |
|---|---|---|
| Polling worker auto-refreshed founder's access_token after forcing expiry | ✅ | `expires_at` jumped from `01:28:56` (force-expired) → `02:29:57` (fresh Google token). `fomo.oauth.refreshed` audit with `provider: 'google'`, `refresh_token_rotated: false`. Zero refresh_token plaintext in audit. |
| Friend's sendblue_contact_status memory_signal populated by `/onboard/callback` | ✅ | `messageinabottleco123@gmail.com` (id `14a6639f-…`) onboarded; signal `registered: true, registered_at: 2026-06-02T01:28:50` written automatically. |
| Outbound worker refused to send when contact_status.registered=false | ✅ | Mechanically proven by 5 regression tests (the gate, happy path, no-signal allow, STOP precedence, canary privacy). Live exercise not required since SendBlue accepted the test phone (returned HTTP 200) — the failure path is fully tested in CI. |
| Server stayed UP throughout the smoke window (no `process.exit(1)` from pool error) | ✅ | PID `74970` persistent for the full ~35-minute smoke window. Zero `fomo.db.connection_error` events fired (Neon was healthy); handler attachment verified per test suite. |
| `pnpm ops:reconcile-sendblue` produced sensible output (gap count + handles) | ✅ | Found 1 historical gap (`615FCE9D-…` — Morris's "Hi" from the v0.5.2 incident). Audited as `fomo.sendblue.delivery_gap_detected`. Real-incident shape now retroactively detected. |

## 7. Founder observations

| Observation | Note |
|---|---|
| Did the four fixes feel like the right shape? Anything you'd refactor before v0.5.4? | The four fixes mapped cleanly onto the v0.5.2 incidents — each one closed a specific gap. No refactor itch. The "registered: true" memory_signal label is slightly fuzzy (it means "we POSTed to /api/v2/contacts," not "SendBlue marked the contact verified for outbound") — but the substrate's behavior on real outbound is gated by SendBlue's own response, which is the correct seam. |
| Any bug surfaced during the smoke that's NOT one of the four items? | One smoke-time friction: the v0.5.2 evidence script's default 24h window slid past Morris's briefed-invite at `2026-06-01 00:09` UTC. Widening to 48h confirmed v0.5.2 PASS criteria still hold. Worth considering a `--since-merge-commit` mode for evidence scripts in a future phase so smoke windows track substrate state, not wall-clock. |
| If you ran a longer overnight test (e.g. 24h+), did the substrate stay alive? | Smoke ran for ~35 minutes uninterrupted. Polling + outbound workers cycled cleanly; OAuth auto-refresh fired 4 times (founder token + the throwaway Gmail's token both refreshed). No crashes. Longer overnight test deferred to a future stability gate if desired. |
| SendBlue plan-level limitation worth noting | Per SendBlue support (received 2026-06-01): on Free Sandbox plan, automatic outbound onboarding via API is not applicable — you must add the contact to the dashboard (or via `/api/v2/contacts`) AND have them text the sender number once to fully verify. Item #1 implements exactly that first step; the second step is the friend's action. Upgrading to a dedicated line removes the "friend must text first" step entirely. Captured in [[sendblue-plan-gates]]. |

## 8. Verdict

✅ **PASS** — all 7 criteria green, all three evidence scripts `VERDICT: PASS`, operator checks confirmed. **Next phase runs its own 6-question gate.**

Followups (non-blocking, captured for future gates):

- Periodic auto-reconciliation worker (currently on-demand-only per founder correction #4)
- Production scaling beyond founder-on-laptop (Neon Starter, Render dyno sizing, etc.)
- Second-friend onboarding flow (separate gate)
- Auto-send (separate gate)
- SendBlue dedicated-line upgrade — removes the "friend must text first" step from the verified-contact flow; cost decision for whoever's running Brevio at scale
- Evidence script window-relative-to-merge-commit (smoke usability improvement)

## 9. Sign-off

- Founder signature: galiettemita
- Date: 2026-06-02

## 10. What v0.5.3 PASS does NOT promise

- Periodic auto-reconciliation worker (still on-demand; future phase if desired)
- Production scaling beyond founder-on-laptop
- Second friend onboarding (its own gate)
- Auto-send / snooze / calendar / MCP / admin dashboard — all still out
- Android/SMS fallback — its own future smoke
- SendBlue dedicated-line upgrade (still on Free Sandbox)

The next phase is decided at a fresh 6-question gate.
