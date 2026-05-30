# Phase v0.5.1 Smoke Test Report — Multi-tenant Substrate

> Filled after running every step in `smoke-test-v0.5.1-multitenant-substrate.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.1.md` once `VERDICT: PASS`. **v0.5.2 (real friend smoke) cannot begin until this lands on `main`.**

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.1-multitenant-substrate`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Synthetic friend Gmail used:** _<email>_
**Synthetic friend phone (last 4 only):** `0002`

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3G1.md` on `main` with `VERDICT: PASS`
- [ ] Migrations 0005 + 0006 applied to Neon (verified via `pnpm migrate:neon` + `\d users` / `\d invite_tokens`)
- [ ] Google Cloud OAuth client lists BOTH `/oauth/google/callback` and `/onboard/callback` as valid redirect URIs

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_FRIEND_BETA_ENABLED` | ☐ | `true` (the smoke invariant) |
| `BREVIO_PHONE_HASH_KEY` | ☐ | 32-byte HMAC key (separate from `BREVIO_TOKEN_KEK`) |
| `FOMO_FRIEND_BETA_BASE_URL` | ☐ | ngrok URL the friend hits |

## 3. PASS criteria

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | `/onboard` mounted when switch on | boot log `fomo.onboard.enabled` + `onboard_route_mounted: true` | ☐ |
| 2 | `/onboard` unavailable when switch off | `curl /onboard` → HTTP 404 + `fomo.onboard.kill_switch_off` audit | ☐ |
| 3 | Two distinct synthetic phones used | founder env phone `…<last 4>` + synthetic friend `…0002`; distinct hashes | ☐ |
| 4 | Friend onboarding via `/onboard` succeeded | `users` has friend row with `is_founder=false` + `phone_e164_hash` populated; `invite_tokens` row has `consumed_at` + `consumed_user_id` matching the friend's `users.id`; `fomo.onboard.user_created` audit row exists | ☐ |
| 5 | Friend-safe Slack card used for non-founder | operator-confirmed: card has NO snippet, NO message_id, shows "friend-owned (user redacted)" in footer; sender + subject + ranker reason + label + score present | ☐ |
| 6 | Per-friend STOP isolation | `memory_signals.stop_active` row for friend `user_id` with `active=true`; founder's stop_active UNTOUCHED; `fomo.sendblue.stop_recorded` audit has `actor_user_id` = friend's UUID, NOT founder | ☐ |
| 7 | Founder flow still works | recent `alert_state_transitions` row: founder `approved → sent` (regression check — v0.1 path unaffected) | ☐ |
| 8 | No leak across all persisted stores | `smoke-evidence:v0.5.1` leak-canary scan: zero hits across audit + memory_signals; raw E.164 phones never persisted | ☐ |

## 4. `smoke-evidence:v0.5.1` output (LOAD-BEARING)

Paste verbatim. The verdict line at the bottom must read `VERDICT: PASS`.

```
…
```

## 5. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend Slack card has NO snippet section | ☐ | _<paste a redacted screenshot or description>_ |
| Friend Slack card footer reads "friend-owned (user redacted)" | ☐ | |
| Friend Slack card shows sender, subject, ranker reason, label, score | ☐ | |
| Founder Slack card (regression) STILL shows full snippet + full footer | ☐ | |
| `/onboard` returns HTTP 404 with `FOMO_FRIEND_BETA_ENABLED=false` | ☐ | |

## 6. Founder observations

| Observation | Note |
|---|---|
| Did the friend onboard via `/onboard` without you logging into anything? | _<yes / yes but I had to…>_ |
| Was the privacy copy clear when you read it as a "friend"? | _<yes / no — what was confusing>_ |
| Did the friend STOP truly leave the founder's `stop_active` untouched? | _<yes / no — paste detail>_ |
| Anything surprising? | _<one-line summary>_ |

## 7. Verdict

☐ **PASS** — every required check in §3 is green, every operator-confirmed check in §5 is green, evidence script printed `VERDICT: PASS`. **v0.5.2 (real friend smoke) may begin.**

☐ **FAIL** — list below.

Failures / followups:

- _…_

## 8. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_

## 9. What v0.5.1 PASS does NOT promise

- A real (not synthetic) friend onboarded — v0.5.2
- Snooze resurface scheduler — v0.3+
- Auto-send — its own gate after v0.5
- Multi-friend cross-tenant UI for the founder — out
- Calendar / Drafting / MCP — L2+, out
