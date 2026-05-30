# Phase v0.5.1 Smoke Test Report — Multi-tenant Substrate

> Filled after running every step in `smoke-test-v0.5.1-multitenant-substrate.md`.
> Committed as `docs/SMOKE_REPORT_v0.5.1.md`. **v0.5.2 (real friend smoke) cannot begin until this lands on `main`.**

---

**Founder:** galiettemita
**Run date:** 2026-05-30 02:42 UTC
**Branch:** `phase-v0.5.1-multitenant-substrate`
**Commit SHA at run time:** to be filled by the commit that lands this report (the smoke ran against `6c4ac661` through `9301f2ab`; the kill_switch_off boot-audit fix and this report ship in a follow-up commit on the same branch).
**Synthetic friend Gmail used:** gm3258@columbia.edu (a second Gmail account the founder controls; NOT a real second human — that's v0.5.2)
**Synthetic friend phone (last 4 only):** `0002` (full number is `+15550100002`, the NANPA-reserved fictional range)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_3G1.md` on `main` with `VERDICT: PASS`
- [x] Migrations 0005 + 0006 applied to Neon (verified via `pnpm migrate:neon` + `\d users` / `\d invite_tokens` — all 5 v0.5.1 columns present with correct types + UNIQUE indexes)
- [x] Google Cloud OAuth client lists BOTH `/oauth/google/callback` and `/onboard/callback` as valid redirect URIs (and both the `https://unshivering-interaulic-beatriz.ngrok-free.dev/*` and `http://localhost:8080/*` variants for flexibility)

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_FRIEND_BETA_ENABLED` | ✅ | `true` for the smoke; `false` for the §9 clean-stop |
| `BREVIO_PHONE_HASH_KEY` | ✅ | 32-byte base64 from `openssl rand -base64 32`; separate from `BREVIO_TOKEN_KEK` per separation-of-duties |
| `FOMO_FRIEND_BETA_BASE_URL` | ✅ | `https://unshivering-interaulic-beatriz.ngrok-free.dev` (free static ngrok subdomain) |

## 3. PASS criteria

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | `/onboard` mounted when switch on | boot log `fomo.onboard.enabled onboard_route_mounted: true privacy_copy_bytes: 3212` + `fomo.server.listening onboard_route_mounted: true` | ✅ |
| 2 | `/onboard` unavailable when switch off | §9 curls: `/onboard`, `/onboard/start`, `/onboard/callback` all return HTTP 404; `fomo.onboard.kill_switch_off` audit row written at boot with `stage: boot, detail: 'FOMO_FRIEND_BETA_ENABLED is not "true"'` | ✅ |
| 3 | Two distinct synthetic phones used | founder env phone `…3459` + synthetic friend `…0002`; distinct phone_e164_hash values | ✅ |
| 4 | Friend onboarding via `/onboard` succeeded | `users` row exists: id=`4606e1e7-7cc0-4ce4-b4e9-0b67a4d38941`, email=`gm3258@columbia.edu`, `is_founder=false`, `phone_e164_hash` + `phone_e164_encrypted` populated. `invite_tokens` row #3 has `consumed_at=2026-05-30 02:15:09.748+00` + `consumed_user_id=4606e1e7-…` matching the friend's id. `fomo.onboard.user_created` audit row exists with same actor_user_id and safe-only detail (`token_hash_prefix`, `intended_phone_slug=0002`, `gmail_history_id`) | ✅ |
| 5 | Friend-safe Slack card used for non-founder | Operator-confirmed (§6 screenshot): card has NO Snippet section, NO message_id text, NO model_name/prompt_version, footer reads "friend-owned (user redacted)", sender + subject + ranker reason + label (`important`) + score (`0.93`) all present, Approve/Reject buttons present. Second independent confirmation: the Sequoia-subject card sent accidentally from the alt Gmail rendered the same friend-safe shape. | ✅ |
| 6 | Per-friend STOP isolation | `memory_signals.stop_active` row for friend `user_id=4606e1e7-…` with `active=true, source=user_confirmed`. Founder's `stop_active` row UNTOUCHED (still `active=false, updated_at=2026-05-29 01:15:54`). `fomo.sendblue.stop_recorded` audit has `actor_user_id=4606e1e7-…` (friend), NOT founder. Safe-only detail: `from_slug=0002` (last 4), `stop_active=true`. Bonus: outbound worker autonomously emitted `fomo.send.stop_enforced` when an approved friend alert tried to fire AFTER the STOP — proving per-user STOP enforcement at the live send path, not just at memory_signals write. | ✅ |
| 7 | Founder flow still works | Alert `0be8c76d-…` (user_id=`founder`, subject "a16z partner intro") chain: `detected → ranked → queued_for_review → approved → sent`. `fomo.send.attempted` + `fomo.send.succeeded` audits with `destination_slug=3459` (last 4), `provider_status=QUEUED`, `provider_message_handle` recorded, `template_version=founder-text-v0.1.0`. Real iMessage arrived on the founder phone (operator-confirmed). | ✅ |
| 8 | No leak across all persisted stores | `smoke-evidence:v0.5.1` leak-canary scan: scanned 500 audit + 2 memory rows; **zero hits**. No raw E.164, no raw email body, no token plaintext, no canary string in any persisted detail. | ✅ |

## 4. `smoke-evidence:v0.5.1` output (LOAD-BEARING)

```
Phase v0.5.1 evidence — multi-tenant substrate smoke

users (friends, phone_e164_hash IS NOT NULL, is_founder=false): 1
  id=4606e1e7… email=g***@columbia.edu hash=97f98e05…

invite_tokens: issued=3 consumed=2

audit_log fomo.onboard.*: invite_issued=3 user_created=2 invite_invalid=4 phone_mismatch=0

fomo.sendblue.stop_recorded: founder=1 friend=1
memory_signals.stop_active: founder=1 friend=1

founder approved → sent transitions: 3

Leak-canary scan (raw E.164 phones must NEVER appear in persisted detail) ...

========================================================================
Phase v0.5.1 evidence summary
========================================================================
  [✓] Migrations + columns up to date on live Neon
        14 tables + 7 required columns present
  [✓] fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS
        fomo.onboard.invite_issued, fomo.onboard.user_created, fomo.onboard.kill_switch_off
  [✓] MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)
        (no regression)
  [✓] Two-user synthetic smoke — friend(s) provisioned in users table
        1 friend row(s); founder still env-resolved (not in users)
  [✓] invite_tokens lifecycle (issue → consume)
        issued=3, consumed=2 (≥1 issued + ≥1 consumed)
  [✓] fomo.onboard.invite_issued audit row (≥1)
        3 issued
  [✓] fomo.onboard.user_created audit row (≥1)
        2 created
  [✓] Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder
        1 friend STOP event(s); 1 founder STOP event(s)
  [✓] memory_signals.stop_active row exists for the friend (per-user isolation)
        friend_rows=1; user_ids=4606e1e7…
  [✓] Founder flow regression — at least one approved → sent transition for founder
        3 recent approved→sent transition(s)
  [✓] No raw phone / canary leakage across audit + memory_signals
        scanned 500 audit + 2 memory rows; zero hits

VERDICT: PASS  (operator must additionally confirm friend-safe Slack card was rendered visually + clean-stop refused /onboard with the switch off)
```

## 5. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend Slack card has NO snippet section | ✅ | Screenshot in chat at §6; card had Sender / Subject / Ranker label / Why / Approve+Reject / footer only — no Snippet block |
| Friend Slack card footer reads "friend-owned (user redacted)" | ✅ | Exact phrasing confirmed |
| Friend Slack card shows sender, subject, ranker reason, label, score | ✅ | All five fields present; ranker `Why` text was egress-safe (no body excerpts) |
| Founder Slack card (regression) STILL shows full snippet + full footer | ✅ | a16z card had Snippet section heading, `Model: gpt-5-mini`, `Prompt: ranker-v0.1.0`, footer `user: founder` + `message_id: 19e76bce4172cd90` |
| `/onboard` returns HTTP 404 with `FOMO_FRIEND_BETA_ENABLED=false` | ✅ | curl confirmed for `/onboard`, `/onboard/start`, `/onboard/callback` — all 404 |

## 6. Founder observations

| Observation | Note |
|---|---|
| Did the friend onboard via `/onboard` without you logging into anything? | Yes — the URL handed off to incognito tab; Google account picker → alt Gmail → consent → "You're connected" page. No founder login required. |
| Was the privacy copy clear when you read it as a "friend"? | Initial copy ("We'll text the iMessage thread for the phone ending in 0002") read as imminent action. Sharpened mid-smoke to lead with "no text was sent during onboarding" + doubly-conditional framing ("when an email looks genuinely important, the founder reviews it in Slack first. If they approve it..."). New copy renders correctly (privacy_copy_bytes=3212). |
| Did the friend STOP truly leave the founder's `stop_active` untouched? | Yes — founder row `updated_at` unchanged from `2026-05-29 01:15:54` while friend's row was created fresh at `2026-05-30 02:22:53` under a distinct user_id. Plus bonus organic evidence: the outbound worker autonomously emitted `fomo.send.stop_enforced` for a friend alert that fired AFTER the friend STOP, while the founder Approve in the same window flowed `approved → sent` cleanly. |
| Anything surprising? | Three real bugs surfaced during smoke (and got fixed before merge): (1) privacy-copy path resolver counted `..` segments assuming source layout but `pnpm dev` runs compiled `dist/`, so the loader looked in `apps/docs/` instead of `backend/docs/` and crashed at boot; (2) `/onboard` callback called `setPhone` (pure UPDATE in Postgres) without first inserting a users row, leaving orphan oauth_tokens + gmail_cursors and "user_created" audit rows pointing at a UUID that didn't exist — the InMemory store auto-created the row, so unit tests passed; (3) success-page wording read as imminent SMS-send to the synthetic phone. Each got a regression test added (3 new gated-PG tests + one onboard.test.ts wording test) so the same gaps can't reopen. The 30-cycle worker caps tripped twice mid-smoke (once for polling, once for outbound) — a safety guard, but worth raising via `FOMO_OUTBOUND_MAX_CYCLES`/equivalent during longer smoke sessions. |

## 7. Verdict

✅ **PASS** — every required check in §3 is green, every operator-confirmed check in §5 is green, evidence script printed `VERDICT: PASS`. **v0.5.2 (real friend smoke) may begin once this report lands on `main`.**

Failures / followups: none blocking merge. Followups deferred to v0.5.2 design or later:

- Identify the env var name for the polling worker's cycle cap (it didn't pick up `FOMO_POLLING_MAX_CYCLES`); document both worker caps in `apps/fomo/.env.3b3.example`.
- For v0.5.2, real friend's phone bound to the invite token at issue time — confirm AAD-bound encryption path holds end-to-end against a non-synthetic phone.
- Consider sharpening the SendBlue inbound runbook payload to use `from_number` consistently (the v0.5.1 runbook said `number`; route extractor accepts only `from_number`/`fromNumber`/`from`). Already patched in this PR.

## 8. Sign-off

- Founder signature: galiettemita
- Date: 2026-05-30

## 9. What v0.5.1 PASS does NOT promise

- A real (not synthetic) friend onboarded — v0.5.2
- Snooze resurface scheduler — v0.3+
- Auto-send — its own gate after v0.5
- Multi-friend cross-tenant UI for the founder — out
- Calendar / Drafting / MCP — L2+, out
