# Phase v0.5.2 Smoke Test Report — Real Friend Beta

> Filled after running every step in `smoke-test-v0.5.2-real-friend.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.2.md` once `VERDICT: PASS` on both
> `smoke-evidence:v0.5.1` (substrate) AND `smoke-evidence:v0.5.2`
> (real-friend specifics). v0.5.2 PASS does **NOT** auto-unlock v1.0;
> the next phase runs its own 6-question gate.

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.2-real-friend-beta-smoke`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Friend's first name only** (PII boundary — do NOT paste full name): _<first name>_
**Friend's phone (last 4 only):** _<last 4>_
**Friend's Gmail domain (redact local-part):** _<e.g. ***@gmail.com>_
**Friend's device:** _<iPhone model / iOS version — REQUIRED for v0.5.2; Android disqualifies>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_v0.5.1.md` on `main` with `VERDICT: PASS`
- [ ] Friend was briefed out-of-band on _<date / channel>_ (iMessage / call / in-person)
- [ ] Briefing covered all five topics: Gmail readonly, founder review surface, STOP semantics, beta status, expected volume
- [ ] Friend agreed verbally to participate (NOT just "they read the privacy copy on /onboard")
- [ ] Friend's phone is iMessage-capable iPhone
- [ ] Friend's Gmail is actively used (not a dead inbox)
- [ ] Friend was reachable during the smoke window
- [ ] Google Cloud OAuth client has BOTH callback URLs registered at the public HTTPS ngrok URL

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_2_FRIEND_BRIEFED` | ☐ | `true` (the briefing assertion) |
| `FOMO_V0_5_2_LEAK_CANARIES` | ☐ | comma-separated canary substrings; matched what was put in the test email body |
| `FOMO_V0_5_2_WINDOW_HOURS` | ☐ | default 24h; raised if smoke spanned longer |
| `FOMO_GMAIL_POLLING_MAX_CYCLES` | ☐ | ≥ 60 (v0.5.2 preflight enforces this) |
| `FOMO_OUTBOUND_MAX_CYCLES` | ☐ | ≥ 60 (v0.5.2 preflight enforces this) |
| `FOMO_FRIEND_BETA_BASE_URL` | ☐ | HTTPS ngrok URL — NOT localhost (friend on different device) |

## 3. PASS criteria (12)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Friend briefed BEFORE invite mint | `fomo.onboard.invite_issued` audit row has `briefed_confirmed: true`, `phone_class: "real"`; `--confirm-briefed yes-friend-was-briefed` provided at mint time | ☐ |
| 2 | Invite token bound to friend's REAL E.164 (NOT in 555-fictional range) | issue script refuses non-555 phone without `--confirm-briefed`; audit detail `phone_class='real'` confirms | ☐ |
| 3 | Friend onboarded via `/onboard` on their own device | `users` row exists with `is_founder=false`, `phone_e164_hash` populated, `email` = friend's Gmail. Founder did NOT log in during this step. | ☐ |
| 4 | Friend received privacy copy at consent screen | Operator-confirmed: friend reported seeing the privacy paragraph rendered above "Connect with Google" — content matched `docs/privacy-copy-v0.5.md` verbatim | ☐ |
| 5 | Friend-safe Slack card rendered (UNCONDITIONAL — invariant of `alert.user_id !== founderUserId`) | Operator-confirmed: card showed NO body/snippet/attachment/message_id; footer "friend-owned (user redacted)" present | ☐ |
| 6 | Founder approved in Slack | `fomo.slack.approval_captured` audit row with `actor_user_id=founder`, alert.user_id=friend | ☐ |
| 7 | Real iMessage delivered to friend's real phone | `fomo.send.succeeded` audit row with `actor_user_id=<friend uuid>`, `provider_status=QUEUED`, `destination_slug=<friend's last 4>`. **Friend confirmed receipt out-of-band.** | ☐ |
| 8 | Friend texted STOP from their iMessage thread | `fomo.sendblue.stop_recorded` audit with `actor_user_id=<friend uuid>`, `provider_message_id` is Apple/SendBlue UUID-shaped (NOT `smoke-v0.5.*` synthetic) | ☐ |
| 9 | Per-friend STOP isolation | `memory_signals.stop_active` row exists for friend `user_id` with `active=true`. Founder's `stop_active` row UNTOUCHED (or absent). | ☐ |
| 10 | Founder regression during the same smoke window | A founder alert chained `detected → ranked → queued_for_review → approved → sent` and the founder's real iMessage arrived. v0.1 path NOT broken. | ☐ |
| 11 | Leak-canary scan clean | `smoke-evidence:v0.5.2` VERDICT: PASS with non-empty `FOMO_V0_5_2_LEAK_CANARIES` set and the canary substrings present in the friend test email body — confirms the scan was active and found ZERO hits | ☐ |
| 12 | Friend's users row is `is_founder=false` | DB query confirms; no privilege escalation through onboard | ☐ |

## 4. `smoke-evidence:v0.5.1` output (substrate health, LOAD-BEARING)

Paste verbatim. Verdict at the bottom must read `VERDICT: PASS`.

```
…
```

## 5. `smoke-evidence:v0.5.2` output (real-friend specifics, LOAD-BEARING)

Paste verbatim. Verdict at the bottom must read `VERDICT: PASS`.

```
…
```

## 6. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend Slack card has NO Snippet section | ☐ | _<paste a redacted screenshot or describe>_ |
| Friend Slack card footer reads "friend-owned (user redacted)" | ☐ | |
| Friend Slack card shows sender, subject, ranker `Why`, label, score | ☐ | |
| Friend Slack card `Why` text is a SUMMARY (not a paraphrase containing body words) | ☐ | _<verify the text doesn't include any verbatim substring from the email body>_ |
| Founder Slack card (regression) STILL shows full Snippet + full footer | ☐ | |
| Friend received iMessage on their real iPhone | ☐ | _<friend confirmed at YYYY-MM-DD HH:MM>_ |
| Friend's STOP reply was an actual iMessage reply (not curl) | ☐ | |

## 7. Friend's experience (out-of-band feedback)

| Question | Friend's answer |
|---|---|
| Was the privacy copy clear when you first read it? | _<…>_ |
| Did anything about the onboarding feel surprising or alarming? | _<…>_ |
| The iMessage you got — did the wording feel like a useful summary, or did it leak any of the body text? | _<…>_ |
| Did STOP work the way you expected? | _<…>_ |
| Would you keep using Brevio post-beta? | _<…>_ |
| Anything you wish had been clearer before you clicked Connect? | _<…>_ |

(This section is the most important part of v0.5.2 — the substrate is proven by v0.5.1; THIS proves the experience.)

## 8. Founder observations

| Observation | Note |
|---|---|
| Did briefing the friend uncover anything you'd want to change in the privacy copy? | _<…>_ |
| Was the friend's STOP iMessage promptly recognized + acted on by the substrate? | _<…>_ |
| Did anything in audit_log surprise you (unexpected actor_user_id, missing rows, etc.)? | _<…>_ |
| Anything you'd want different about the friend-safe Slack card before showing it to a second friend? | _<…>_ |

## 9. Verdict

☐ **PASS** — every required check in §3 is green, every operator-confirmed check in §6 is green, friend feedback in §7 captured honestly, both evidence scripts printed `VERDICT: PASS`. **The next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

Failures / followups:

- _…_

## 10. Sign-off

- Founder signature: _<name>_
- Friend's first name + verbal consent to publish this report: _<friend first name> — yes / no>_
- Date: _<YYYY-MM-DD>_

## 11. Aftercare confirmation

- [ ] Friend was told the beta gate is complete
- [ ] Friend was told they can keep texting STOP / START anytime
- [ ] Friend was asked whether they want to remain onboarded post-beta
- [ ] If friend opted out: their `users` + `oauth_tokens` + `gmail_cursors` rows deleted; Google OAuth token revoked on Google's side

## 12. What v0.5.2 PASS does NOT promise (next-phase boundaries)

v0.5.2 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- A second friend — that's its own gate (could be v0.5.3 if pursued; could be skipped entirely)
- Auto-send — its own gate
- Snooze resurface scheduler — v0.3+ surface, separate gate
- Public self-serve onboarding — out indefinitely
- Multi-friend cross-tenant UI — out for v0.5.x
- Calendar / Drafting / MCP / browser automation — L2+ surfaces
- Admin dashboard — out
- Production scaling — out (still founder-on-laptop scale)
- Android/SMS fallback — its own future smoke

The next phase is decided AT THE NEXT 6-question gate, with the founder's full attention on whatever the v0.5.2 friend experience taught them.
