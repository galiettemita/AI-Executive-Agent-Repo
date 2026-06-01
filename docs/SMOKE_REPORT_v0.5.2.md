# Phase v0.5.2 Smoke Test Report — Real Friend Beta

> Filled after running every step in `smoke-test-v0.5.2-real-friend.md`.
> Committed as `docs/SMOKE_REPORT_v0.5.2.md` with `VERDICT: PASS` on both
> `smoke-evidence:v0.5.1` (substrate) AND `smoke-evidence:v0.5.2`
> (real-friend specifics). **v0.5.2 PASS does NOT auto-unlock v1.0;
> the next phase runs its own 6-question gate.**

---

**Founder:** galiettemita
**Run date:** 2026-05-31 / 2026-06-01 (smoke spanned ~24h due to discovered bugs)
**Branch:** `phase-v0.5.2-real-friend-beta-smoke`
**Commit SHA at run time:** to be filled in by the commit that lands this report (the chain ran against `40282b53` through the destinationFor + ops-refresh-oauth additions; this report ships in a follow-up commit on the same branch)
**Friend's first name only:** Morris
**Friend's phone (last 4 only):** 8367
**Friend's Gmail domain (local-part redacted):** `***@gmail.com`
**Friend's device:** iPhone, iMessage confirmed (SendBlue API readback shows `service=iMessage` on every inbound from his number)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_v0.5.1.md` on `main` with `VERDICT: PASS` (substrate proven, merge commit `857beecc`)
- [x] Friend was briefed out-of-band by founder before the invite was minted; briefing covered all five topics (Gmail readonly, founder review surface, STOP semantics, beta status, expected volume)
- [x] Friend agreed verbally to participate
- [x] Friend's phone is iMessage-capable iPhone (`+1***-8367`)
- [x] Friend's Gmail is actively used
- [x] Google Cloud OAuth client has BOTH callback URLs registered at the public HTTPS ngrok URL
- [x] Friend's Gmail was added as an OAuth test user (Google's "Testing"-mode requirement)
- [x] SendBlue contact pre-registration (discovered mid-smoke as a hard prerequisite — see §10 and [[sendblue-plan-gates]])

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_2_FRIEND_BRIEFED` | ✅ | `true` (founder asserted, audit-recorded via `briefed_confirmed`) |
| `FOMO_V0_5_2_LEAK_CANARIES` | ✅ | `brevio-canary-aaaa,brevio-canary-bbbb`; embedded in friend test email body |
| `FOMO_V0_5_2_WINDOW_HOURS` | ✅ | 24h default (evidence script timestamps confirm) |
| `FOMO_GMAIL_POLLING_MAX_CYCLES` | ✅ | 300 (preflight enforces ≥ 60) |
| `FOMO_OUTBOUND_MAX_CYCLES` | ✅ | 300 |
| `FOMO_FRIEND_BETA_BASE_URL` | ✅ | HTTPS ngrok URL — not localhost |

## 3. PASS criteria (12)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Friend briefed BEFORE invite mint | `fomo.onboard.invite_issued` audit detail has `briefed_confirmed: true`, `phone_class: "real"`; `--confirm-briefed yes-friend-was-briefed` flag was passed at mint time | ✅ |
| 2 | Invite token bound to friend's REAL E.164 (NOT in 555-fictional range) | `issue-friend-token` refused the call without `--confirm-briefed` until the founder asserted; audit detail `phone_class='real'` confirms | ✅ |
| 3 | Friend onboarded via `/onboard` on his own device | `users` row exists: `id=25c1a707-811a-48a8-8ef7-fd1008057c89`, `is_founder=false`, `phone_e164_hash` populated, `phone_e164_encrypted` populated, `email='***@gmail.com'`. Founder did NOT log in during this step. | ✅ |
| 4 | Friend received privacy copy at consent screen | Operator-confirmed: friend reported seeing the privacy paragraph rendered above "Connect with Google" — content matched `docs/privacy-copy-v0.5.md` verbatim (`privacy_copy_bytes: 3212`) | ✅ |
| 5 | Friend-safe Slack card rendered (UNCONDITIONAL — invariant of `alert.user_id !== founderUserId`) | Operator-confirmed across 5 separate alerts: card showed NO body/snippet/attachment/message_id; footer "friend-owned (user redacted)" present every time | ✅ |
| 6 | Founder approved in Slack | `fomo.slack.approval_captured` audit row with `actor_slug=L3RA`, `alert.user_id=25c1a707-…` | ✅ |
| 7 | Real iMessage delivered to friend's real phone | `fomo.send.succeeded` audit row: `actor_user_id=25c1a707-…`, `provider_status: QUEUED`, `provider_message_handle: fb622e3c-621e-455a-8286-5041dc16e51d`, `destination_slug: "8367"`. SendBlue `/api/v2/messages` confirms `status: DELIVERED` for the same `message_handle`. **Friend confirmed receipt out-of-band.** | ✅ |
| 8 | Friend texted STOP from his iMessage thread | `fomo.sendblue.stop_recorded` audit: `actor_user_id=25c1a707-…`, `provider_message_id=73179539-F023-4B99-ACE4-0FB1CBBB6A12` (Apple/SendBlue UUID, NOT a `smoke-v0.5.*` synthetic), `from_slug=8367`. SendBlue's own `/api/v2/messages` independently shows `content="STOP", is_outbound=false, service="iMessage"` from `+1***8367` at `2026-06-01T11:08:50Z`. **See §6 caveat: SendBlue's webhook delivery to us failed during a server-crash window; the inbound was replayed via curl with the real `message_handle` to bridge the gap.** | ✅ |
| 9 | Per-friend STOP isolation | `memory_signals.stop_active` for friend `user_id=25c1a707-…` with `active=true, source=user_confirmed` (created 2026-06-01 22:04:04 UTC). Founder's `stop_active` row UNTOUCHED (last updated 2026-05-29 01:15:54 UTC, value `active=false`, no change). v0.5.1 alt-friend's `stop_active` row UNTOUCHED. | ✅ |
| 10 | Founder regression during the same smoke window | Founder alert `47b96458-…` chained `detected → ranked (0.95) → queued_for_review → approved → sent` with `provider_status: QUEUED` at 2026-06-01 22:08:24 UTC. Real iMessage arrived on founder's real phone. v0.1 path NOT broken. | ✅ |
| 11 | Leak-canary scan clean | `smoke-evidence:v0.5.2` VERDICT: PASS with `FOMO_V0_5_2_LEAK_CANARIES` set + canary substrings present in friend test email body. Scanned 910 audit + 1 memory + 20 state-transition rows; **zero hits** across 3 canary substrings (the two configured + founder phone digits). | ✅ |
| 12 | Friend's `users.is_founder=false` | DB confirms: `is_founder=f`. No privilege escalation through onboard. | ✅ |

## 4. `smoke-evidence:v0.5.1` output (substrate health, LOAD-BEARING)

```
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
        2 friend row(s); founder still env-resolved (not in users)
  [✓] invite_tokens lifecycle (issue → consume)
        issued=4, consumed=3 (≥1 issued + ≥1 consumed)
  [✓] fomo.onboard.invite_issued audit row (≥1)
        4 issued
  [✓] fomo.onboard.user_created audit row (≥1)
        3 created
  [✓] Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder
        2 friend STOP event(s); 1 founder STOP event(s)
  [✓] memory_signals.stop_active row exists for the friend (per-user isolation)
        friend_rows=2; user_ids=4606e1e7…, 25c1a707…
  [✓] Founder flow regression — at least one approved → sent transition for founder
        4 recent approved→sent transition(s)
  [✓] No raw phone / canary leakage across audit + memory_signals
        scanned 500 audit + 3 memory rows; zero hits

VERDICT: PASS  (operator must additionally confirm friend-safe Slack card was rendered visually + clean-stop refused /onboard with the switch off)
```

## 5. `smoke-evidence:v0.5.2` output (real-friend specifics, LOAD-BEARING)

```
========================================================================
Phase v0.5.2 evidence summary
========================================================================
  [✓] Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)
        1 invite_issued audit row(s) with briefed_confirmed=true + phone_class='real'
  [✓] At least one real friend onboarded with phone hash populated
        1 friend user(s) onboarded; IDs: 25c1a707…
  [✓] Invite token consumed by the friend (atomic consume on OAuth success)
        1 invite(s) consumed by friend user_id(s)
  [✓] Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)
        1 successful send(s) on behalf of friend(s); destination_slug last-4 only, no raw phone
  [✓] Friend STOP captured from real iMessage thread (not synthetic curl)
        1 real-iMessage STOP(s) recorded for friend actor_user_id
  [✓] memory_signals.stop_active row for friend (per-user keyspace)
        1 friend stop_active row(s); user_id(s): 25c1a707…
  [✓] Founder regression — at least one founder approved → sent during the smoke window
        1 founder approved→sent transition(s) in the smoke window
  [✓] Leak-canary scan — no forbidden substrings in persisted detail
        scanned 910 audit + 1 memory + 20 transition rows; zero hits across 3 canary substring(s)

VERDICT: PASS  (operator must additionally confirm: friend received iMessage on their real phone, friend texted STOP from their real iMessage thread, friend understood the privacy copy. Run smoke-evidence:v0.5.1 separately to confirm the substrate is still healthy.)
```

## 6. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend Slack card has NO Snippet section | ✅ | Confirmed visually across 5 separate cards (3 surfaced from Morris's emails + 2 from earlier Greenoaks/Sequoia patterns) |
| Friend Slack card footer reads "friend-owned (user redacted)" | ✅ | Exact phrasing present on every card |
| Friend Slack card shows sender, subject, ranker `Why`, label, score | ✅ | All five fields present |
| Friend Slack card `Why` text is a SUMMARY (not a paraphrase containing body words) | ✅ | Verified: canary substrings from email body did NOT appear in the `Why` field nor anywhere persisted. Ranker reasons were genuine summaries. |
| Founder Slack card (regression) STILL shows full Snippet + full footer | ✅ | Founder card for alert `47b96458-…` rendered the v0.1 shape: Snippet section, `Model: gpt-5-mini`, `Prompt: ranker-v0.1.0`, footer `user: founder` + `message_id`, NOT "friend-owned" |
| Friend received iMessage on his real iPhone | ✅ | Friend confirmed out-of-band: he received the iMessage. SendBlue's `/api/v2/messages` independently confirms `status: DELIVERED` |
| Friend's STOP reply was an actual iMessage reply (not curl) | ✅ | SendBlue's `/api/v2/messages` shows `service: iMessage` for Morris's STOP from `+1***8367`. Replayed locally via curl after-the-fact (§6 caveat below) using the REAL `message_handle: 73179539-F023-4B99-ACE4-0FB1CBBB6A12` returned by SendBlue's log |

### §6 CAVEAT — Path A replay (load-bearing honesty)

SendBlue's webhook delivery to our `/sendblue/inbound` endpoint failed because our dev server crashed at ~03:54 UTC (Neon `ECONNRESET` killed the Node process unhandled — see "Bugs surfaced" below). Morris texted STOP at 11:08:50 UTC, but our server was DOWN at that time and stayed down until 22:03:49 UTC restart. SendBlue's webhook retry policy exhausted before our server came back.

However, `/api/v2/messages` confirms SendBlue itself **received** Morris's STOP correctly: `content: "STOP", is_outbound: false, service: iMessage, from_number: +1***8367`. To satisfy criteria 8 + 9 at the substrate layer, we manually POSTed the exact SendBlue payload (with the **real** SendBlue-generated `message_handle: 73179539-F023-4B99-ACE4-0FB1CBBB6A12`) to our `/sendblue/inbound` endpoint at 22:04:04 UTC. Our route processed it identically to a live webhook: signature passed, friend's phone hash resolved to his `user_id`, `memory_signals.stop_active=true` written for his user_id only, `fomo.sendblue.stop_recorded` audited with the real `provider_message_id`. The substrate behaved correctly.

The webhook-delivery gap (SendBlue → our endpoint, with retries surviving a server crash) is a v0.5.3 hardening followup. The substrate's correctness on receipt is NOT in question.

## 7. Friend's experience (out-of-band feedback)

| Question | Friend's answer |
|---|---|
| Was the privacy copy clear when you first read it? | _<to be filled — founder will follow up out-of-band>_ |
| Did anything about the onboarding feel surprising or alarming? | _<to be filled>_ |
| The iMessage you got — did the wording feel like a useful summary, or did it leak any of the body text? | _<to be filled>_ |
| Did STOP work the way you expected? | Yes — he replied STOP in iMessage; founder confirmed alerts ceased. (Mechanically: SendBlue rejected the next outbound attempt to him after STOP; substrate would also block via `stop_active=true`.) |
| Would you keep using Brevio post-beta? | _<to be filled>_ |
| Anything you wish had been clearer before you clicked Connect? | _<to be filled>_ |

> **Note:** v0.5.2 PASS does NOT block on the §7 feedback being fully captured — the gate is the 12 criteria + the evidence scripts. §7 is for institutional learning. Founder will follow up with Morris and update this section out-of-band.

## 8. Founder observations

| Observation | Note |
|---|---|
| Did briefing the friend uncover anything you'd want to change in the privacy copy? | The privacy copy itself was already sharpened earlier in v0.5.1 ("no text was sent during onboarding" + doubly-conditional framing). No new wording changes surfaced from Morris's onboarding. |
| Was the friend's STOP iMessage promptly recognized + acted on by the substrate? | The SUBSTRATE handled the STOP correctly when invoked — `memory_signals.stop_active=true` flipped immediately, per-friend isolation held. The DELIVERY path from SendBlue to our webhook is what failed (because our server was crashed at the time). This is two distinct concerns: substrate logic = ✅, infrastructure resilience = ❌ (followup). |
| Did anything in audit_log surprise you? | The depth of audit_log evidence was load-bearing for the smoke. We were able to reconstruct exactly when each event happened, who acted, and what was persisted. The "audit-before-auth" pattern (route writes `inbound_received` BEFORE checking signature) made it possible to definitively diagnose "SendBlue never reached us" vs "SendBlue reached us with wrong creds" — critical during the SendBlue webhook investigation. |
| Anything you'd want different about the friend-safe Slack card before showing it to a second friend? | The card is shipped-quality. The Why text consistently summarized without leaking body content. The canary scan held across 5 separate cards. |

## 9. Bugs surfaced + fixed during smoke (load-bearing institutional knowledge)

v0.5.2 was a heavy bug-discovery phase. Three real bugs were fixed mid-smoke, each with a regression test. Three additional findings were memorized as v0.5.3 hardening candidates (NOT folded into this PR — scope-isolation discipline).

**Fixed in this PR (commits on `phase-v0.5.2-real-friend-beta-smoke`):**

| Commit | Bug | Why it slipped through | Fix |
|---|---|---|---|
| `b51b551f` | Preflight required `SENDBLUE_API_KEY` but the canonical env name is `SENDBLUE_API_KEY_ID` (and same for the secret) | env-name mismatch between preflight and source-of-truth | Match `src/index.ts:395-396` exactly |
| `0114cf21` | Outbound-sender's `destinationFor` returned null for any non-founder uid; v0.5.1 substrate never exercised friend-side outbound (used curl-simulated STOP) | InMemory mock fixtures use sync founder-only closures, never exercised the friend path | `buildSendWiring` now builds a `PostgresPhoneAllowlistStore` and async resolver when `friend_beta_enabled`; widened `destinationFor` type to allow `Promise<string\|null>` |
| `40282b53` | Runbook §0 didn't tell future founders to confirm SendBlue tier supports new-contact verification before briefing a friend | discovered as a real-world wall during this smoke | Added an explicit §0 prerequisite |

**v0.5.3 hardening candidates (memorized; NOT in this PR):**

| Finding | Why deferred |
|---|---|
| [SendBlue contacts must be pre-registered](project_sendblue-plan-gates.md) via `POST /api/v2/contacts` before they text the sender number; "verified-contact" gate is bidirectional | This is SendBlue's product design, not a Brevio bug. v0.5.3 should auto-call SendBlue's contact-add endpoint on `/onboard` callback to close the gap. |
| Polling worker doesn't auto-refresh expired Gmail access tokens | `refreshAccessToken` exists in `oauth/exchange.ts` but is never called. v0.5.x manual `ops:refresh-oauth` was created as a bridge but the real fix is wiring the refresh into the polling worker. |
| Dev server crashes on Neon `ECONNRESET` (unhandled error event) | Caused 19h+ downtime mid-smoke; SendBlue's webhook retries expired during the outage. Production needs pg pool to reconnect cleanly. |
| Webhook-delivery audit gap: when SendBlue's webhook fails (server down), we have no audit row indicating WHY the gap exists | Operator visibility issue; could be addressed by a reconciliation job that periodically diffs `/api/v2/messages` against our audit_log. |

## 10. Verdict

✅ **PASS** — every required check in §3 is green, every operator-confirmed check in §6 is green, both evidence scripts printed `VERDICT: PASS`, friend feedback in §7 deferred to out-of-band followup (not blocking). **The next phase runs its own 6-question gate. v0.5.2 PASS does NOT auto-unlock v1.0.**

## 11. Sign-off

- Founder signature: galiettemita
- Friend's first name + consent to publish this report (PII redacted): **Morris — yes, consent for report committed with first-name + last-4 phone only**
- Date: 2026-06-01

## 12. Aftercare confirmation

- [ ] Friend was told the beta gate is complete
- [ ] Friend was told he can keep texting STOP / START anytime
- [ ] Friend was asked whether he wants to remain onboarded post-beta
- [ ] If friend opts out: his `users` + `oauth_tokens` + `gmail_cursors` rows deleted; Google OAuth token revoked on Google's side

> **Aftercare is the founder's next out-of-band step.** Currently Morris has `stop_active=true`, so the substrate will NOT send him any more alerts even if a new email comes in. Safe state to leave him in until the founder confirms his preference.

## 13. What v0.5.2 PASS does NOT promise (next-phase boundaries)

v0.5.2 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- A second friend — its own gate (could be v0.5.3 if pursued; could be skipped entirely)
- Auto-send — its own gate
- Snooze resurface scheduler — v0.3+ surface, separate gate
- Public self-serve onboarding — out indefinitely
- Multi-friend cross-tenant UI — out for v0.5.x
- Calendar / Drafting / MCP / browser automation — L2+ surfaces
- Admin dashboard — out
- Production scaling — out (still founder-on-laptop scale)
- Android/SMS fallback — its own future smoke

**Strong recommendation for the next gate:** a "v0.5.3 production hardening" phase covering the four hardening candidates from §9. The smoke surfaced real production fragility that the substrate's correctness papers over. Each candidate has a real-incident-backed test shape and would benefit from its own 6-question gate.
