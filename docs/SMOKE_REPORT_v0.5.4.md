# Phase v0.5.4 Smoke Test Report — Second-Friend Cross-Tenant Smoke

> Filled after running every step in `smoke-test-v0.5.4-second-friend.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.4.md` once `VERDICT: PASS` on ALL
> FOUR evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4) AND the
> cross-tenant baseline diff in §6 shows no Morris/founder writes.
>
> **v0.5.4 PASS does NOT auto-unlock v1.0 or broad beta.** The next
> phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-04 13:58 → 14:17 UTC (~20-minute live window; report finalized same day)
**Branch:** `phase-v0.5.4-second-friend-smoke`
**Commit SHA at run time:** `c6338409c53b407fa3349485ddc5615195cecf77`
**Friend B's first name only** (PII boundary — do NOT paste full name): Sheila
**Friend B's phone (last 4 only):** 2787
**Friend B's Gmail domain (redact local-part):** `***@flatbush.org`
**Friend B's device:** iPhone (model unspecified at report time — iOS recent; Messages/iMessage flow worked end-to-end with blue bubbles)
**Morris user_id (for cross-tenant diff):** `25c1a707-811a-48a8-8ef7-fd1008057c89` (same value as `FOMO_V0_5_4_MORRIS_USER_ID`)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_v0.5.2.md` AND `docs/SMOKE_REPORT_v0.5.3.md` on `main` with `VERDICT: PASS` (commits `675b76a3` and `d2dbd626`)
- [x] Friend B briefed out-of-band on 2026-06-04 (channel: out-of-band per founder; verbal "yes" recorded)
- [x] Briefing covered all five topics (Gmail readonly, founder review, STOP, beta status, expected volume)
- [x] Friend B agreed verbally to participate
- [x] Friend B's phone is iMessage-capable iPhone
- [x] Friend B's Gmail is actively used
- [x] Friend B's Gmail added to Google Cloud Console test-user list
- [x] Friend B reachable during the smoke window
- [x] Morris is NOT being notified of this smoke (he remains unaware)
- [x] §0.A baseline snapshot captured into `/tmp/v0.5.4-baseline-stop-active.txt` and `/tmp/v0.5.4-baseline-contact-status.txt` BEFORE smoke start

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_4_FRIEND_BRIEFED` | ✅ | `true` (briefing assertion) |
| `FOMO_V0_5_4_FRIEND_NAME` | ✅ | first name only — "Sheila" |
| `FOMO_V0_5_4_BASELINE_CONFIRMED` | ✅ | `true` after §0.A capture |
| `FOMO_V0_5_4_MORRIS_USER_ID` | ✅ | `25c1a707-811a-48a8-8ef7-fd1008057c89` |
| `FOMO_V0_5_4_LEAK_CANARIES` | ✅ | `brevio-canary-cccc,brevio-canary-dddd` (different from v0.5.2) |
| `FOMO_V0_5_4_WINDOW_HOURS` | ✅ | default 24h |
| `FOMO_GMAIL_POLLING_MAX_CYCLES` | ✅ | 300 (≥ 60) |
| `FOMO_OUTBOUND_MAX_CYCLES` | ✅ | 300 (≥ 60) |
| `FOMO_FRIEND_BETA_BASE_URL` | ✅ | `https://unshivering-interaulic-beatriz.ngrok-free.dev` (static ngrok HTTPS) |

## 3. PASS criteria (16 — 12 v0.5.2 carry-forward + 4 NEW cross-tenant)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Friend B briefed BEFORE invite mint | `fomo.onboard.invite_issued` audit has `briefed_confirmed=true`, `phone_class='real'` (v0.5.4 evidence C1: 1 row) | ✅ |
| 2 | Invite token bound to Friend B's REAL E.164 (NOT 555-fictional) | audit `phone_class='real'` (v0.5.4 C2: 1 invite) | ✅ |
| 3 | Friend B onboarded via `/onboard` on their own device with their own Gmail | NEW `users` row `8fbead5c-68b6-49fc-bf55-2bf9174c2e01`, `email=smita@flatbush.org`, `is_founder=false`, `phone_e164_hash=e1980d82…`, NOT Morris's UUID | ✅ |
| 4 | Friend B received privacy copy at consent screen | Operator-confirmed: Friend B reported landing on the "Connected to Brevio" page and that "they're all set" | ✅ |
| 5 | Friend-safe Slack card rendered for Friend B (no body/snippet/headers/message_id leak) | v0.5.4 C5: 6 slack-review audit rows for Friend B; operator-confirmed visually (see §9) | ✅ |
| 6 | Founder approved in Slack | v0.5.4 C6: 1 founder approval captured; alert `36ffe6d8-29f9-4131-9212-558e2cd50aa6` (user_id=8fbead5c…, score 0.92) | ✅ |
| 7 | Real iMessage delivered to Friend B | v0.5.4 C7: 1 `fomo.send.succeeded` for Friend B; `provider_status=QUEUED`; transitioned to `sent` at 14:08:52 UTC. **Friend B confirmed receipt out-of-band ("sheila got the text message. i see it!").** | ✅ |
| 8 | Friend B texted STOP from real iMessage | v0.5.4 C8: 1 real-iMessage STOP recorded for `8fbead5c…` (`provider_message_id` is SendBlue UUID-shaped) | ✅ |
| 9 | Per-friend STOP isolation — Friend B's `stop_active=true` row written | `memory_signals.stop_active` row for `8fbead5c…`, `active=true`, `source=user_confirmed`, `updated_at=2026-06-04 14:11:51.674982+00` | ✅ |
| 10 | Founder regression during the same smoke window | Alert `aa51b11f-8ec0-4f75-8e21-6ed7e0d1a010` (user_id=founder, score 0.92) chained `created (14:16:45) → queued_for_review → approved → sent (14:17:09)`; real iMessage delivered to founder phone (operator-confirmed: "i got the text message") | ✅ |
| 11 | Leak-canary scan clean across Friend B's content | v0.5.4 C11: scanned 533 audit + 2 memory + 28 transition rows; zero hits across 3 canary substrings | ✅ |
| 12 | Friend B's `users.is_founder=false` | v0.5.4 C12: 1 Friend B user; zero have `is_founder=true` | ✅ |
| **13** | **NEW — Morris's `stop_active` row UNTOUCHED throughout smoke window** | Morris row `updated_at=2026-06-01 22:04:04.182916+00` (predates smoke start by 2.5+ days); §6 diff confirms byte-identical | ✅ |
| **14** | **NEW — Founder's `stop_active` row UNTOUCHED throughout smoke window** | Founder row `updated_at=2026-05-29 01:15:54.023+00` (predates smoke by 6 days); §6 diff confirms byte-identical | ✅ |
| **15** | **NEW — Distinct `sendblue_contact_status` rows per friend** | Friend B contact_status row = 1 (fresh, `2026-06-04 14:03:05`); Morris row = 0 (NOT overwritten); pre-existing `14a6639f…` row untouched. Per-user keyspace preserved. | ✅ |
| **16** | **NEW — v0.5.3 hardening still functional** | v0.5.4 C16: 7/7 hardening audit actions registered; `sendblue_contact_status` kind registered; 1 contact-lifecycle audit row fired during Friend B onboarding | ✅ |

## 4. `smoke-evidence:v0.5.1` output (substrate)

```
users (friends, phone_e164_hash IS NOT NULL, is_founder=false): 4
  id=4606e1e7… email=g***@columbia.edu hash=97f98e05…
  id=25c1a707… email=m***@gmail.com hash=4afbda35…
  id=14a6639f… email=m***@gmail.com hash=ad01a6b6…
  id=8fbead5c… email=s***@flatbush.org hash=e1980d82…

invite_tokens: issued=10 consumed=5

audit_log fomo.onboard.*: invite_issued=8 user_created=4 invite_invalid=14 phone_mismatch=3

fomo.sendblue.stop_recorded: founder=1 friend=3
memory_signals.stop_active: founder=1 friend=3

founder approved → sent transitions: 5

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
        4 friend row(s); founder still env-resolved (not in users)
  [✓] invite_tokens lifecycle (issue → consume)
        issued=10, consumed=5 (≥1 issued + ≥1 consumed)
  [✓] fomo.onboard.invite_issued audit row (≥1)
        8 issued
  [✓] fomo.onboard.user_created audit row (≥1)
        4 created
  [✓] Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder
        3 friend STOP event(s); 1 founder STOP event(s)
  [✓] memory_signals.stop_active row exists for the friend (per-user isolation)
        friend_rows=3; user_ids=4606e1e7…, 25c1a707…, 8fbead5c…
  [✓] Founder flow regression — at least one approved → sent transition for founder
        5 recent approved→sent transition(s)
  [✓] No raw phone / canary leakage across audit + memory_signals
        scanned 500 audit + 6 memory rows; zero hits

VERDICT: PASS  (operator must additionally confirm friend-safe Slack card was rendered visually + clean-stop refused /onboard with the switch off)
```

## 5. `smoke-evidence:v0.5.2` output (first-friend specifics still hold)

```
========================================================================
Phase v0.5.2 evidence summary
========================================================================
  [✓] Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)
        1 invite_issued audit row(s) with briefed_confirmed=true + phone_class='real'
  [✓] At least one real friend onboarded with phone hash populated
        1 friend user(s) onboarded; IDs: 8fbead5c…
  [✓] Invite token consumed by the friend (atomic consume on OAuth success)
        1 invite(s) consumed by friend user_id(s)
  [✓] Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)
        1 successful send(s) on behalf of friend(s); destination_slug last-4 only, no raw phone
  [✓] Friend STOP captured from real iMessage thread (not synthetic curl)
        1 real-iMessage STOP(s) recorded for friend actor_user_id
  [✓] memory_signals.stop_active row for friend (per-user keyspace)
        1 friend stop_active row(s); user_id(s): 8fbead5c…
  [✓] Founder regression — at least one founder approved → sent during the smoke window
        1 founder approved→sent transition(s) in the smoke window
  [✓] Leak-canary scan — no forbidden substrings in persisted detail
        scanned 528 audit + 2 memory + 28 transition rows; zero hits across 3 canary substring(s)

VERDICT: PASS  (operator must additionally confirm: friend received iMessage on their real phone, friend texted STOP from their real iMessage thread, friend understood the privacy copy. Run smoke-evidence:v0.5.1 separately to confirm the substrate is still healthy.)
```

Note: v0.5.2's evidence script — although nominally about "the v0.5.2 friend (Morris)" — searches a rolling 24h window. Because Sheila (Friend B in v0.5.4) was onboarded *within* that same window, the v0.5.2 script's checks resolve against Sheila's run rather than Morris's. The criteria (one briefed real-phone invite, one friend onboarded, one approve→sent chain, one real-iMessage STOP) all hold against Sheila, which is exactly the regression behaviour we want: the v0.5.2 contract continues to be satisfied by whichever live friend is most recent.

## 6. Cross-tenant baseline diff (THE v0.5.4 LOAD-BEARING SECTION)

### §6.A `stop_active` baseline (captured §0.A, BEFORE smoke):

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
 founder                              | stop_active | {                                            +| user_confirmed | 2026-05-29 01:15:54.023+00
                                      |             |     "active": false,                         +|                |
                                      |             |     "recorded_at": "2026-05-29T01:15:54.023Z"+|                |
                                      |             | }                                             |                |
(3 rows)
```

### §6.B `stop_active` post-smoke:

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

### §6.C diff:

```
10a11,14
>  8fbead5c-68b6-49fc-bf55-2bf9174c2e01 | stop_active | {                                            +| user_confirmed | 2026-06-04 14:11:51.674982+00
>                                       |             |     "active": true,                          +|                |
>                                       |             |     "recorded_at": "2026-06-04T14:11:51.639Z"+|                |
>                                       |             | }                                             |                |
15c19
< (3 rows)
---
> (4 rows)
```

**Expected diff shape:**
- Exactly ONE row added: Friend B's stop_active=true with `updated_at` inside the smoke window.
- ZERO modifications to Morris's row (Morris's `updated_at`, `detail`, `source` all identical between baseline and post).
- ZERO modifications to founder's row.

**Operator confirms:** ✅ Yes, the diff shows only Friend B's added row. Morris (`25c1a707…`), gm3258 (`4606e1e7…` — pre-existing v0.5.1 synthetic), and founder are byte-identical between baseline and post. The only change is the single appended row for `8fbead5c…` (Sheila), STOP recorded at 14:11:51 UTC, `source=user_confirmed`.

### §6.D `sendblue_contact_status` baseline + post diff:

Baseline:
```
               user_id                |          kind           |                     detail                      |   source    |          updated_at
--------------------------------------+-------------------------+-------------------------------------------------+-------------+-------------------------------
 14a6639f-2776-4b1a-af9b-834fc1855899 | sendblue_contact_status | {                                              +| founder_set | 2026-06-02 01:28:51.939811+00
                                      |                         |     "registered": true,                        +|             |
                                      |                         |     "registered_at": "2026-06-02T01:28:50.867Z"+|             |
                                      |                         | }                                               |             |
(1 row)
```

Post-smoke:
```
               user_id                |          kind           |                     detail                      |   source    |          updated_at
--------------------------------------+-------------------------+-------------------------------------------------+-------------+-------------------------------
 14a6639f-2776-4b1a-af9b-834fc1855899 | sendblue_contact_status | {                                              +| founder_set | 2026-06-02 01:28:51.939811+00
                                      |                         |     "registered": true,                        +|             |
                                      |                         |     "registered_at": "2026-06-02T01:28:50.867Z"+|             |
                                      |                         | }                                               |             |
 8fbead5c-68b6-49fc-bf55-2bf9174c2e01 | sendblue_contact_status | {                                              +| founder_set | 2026-06-04 14:03:05.085965+00
                                      |                         |     "registered": true,                        +|             |
                                      |                         |     "registered_at": "2026-06-04T14:03:04.599Z"+|             |
                                      |                         | }                                               |             |
(2 rows)
```

Diff:
```
7c7,11
< (1 row)
---
>  8fbead5c-68b6-49fc-bf55-2bf9174c2e01 | sendblue_contact_status | {                                              +| founder_set | 2026-06-04 14:03:05.085965+00
>                                       |                         |     "registered": true,                        +|             |
>                                       |                         |     "registered_at": "2026-06-04T14:03:04.599Z"+|             |
>                                       |                         | }                                               |             |
> (2 rows)
```

**Operator confirms:** ✅ Friend B has a fresh `sendblue_contact_status` row (auto-registered by v0.5.3 hardening during her `/onboard` at 14:03:05). The pre-existing `14a6639f…` row is byte-identical between baseline and post. Morris (`25c1a707…`) has NO `sendblue_contact_status` row in either snapshot — never had one, never written one, no cross-tenant overwrite.

## 7. `smoke-evidence:v0.5.3` output (hardening still wired)

```
========================================================================
Phase v0.5.3 evidence summary
========================================================================
  [✓] All 7 v0.5.3 audit actions registered in FOMO_AUDIT_ACTIONS
        fomo.sendblue.contact_registered, fomo.sendblue.contact_registration_failed, fomo.send.contact_not_registered, fomo.oauth.refreshed, fomo.oauth.refresh_failed, fomo.db.connection_error, fomo.sendblue.delivery_gap_detected
  [✓] 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
        present
  [✓] Item #1: SendBlue contact auto-registration audit row present in smoke window
        1 audit row(s) — auto-registration fired
  [✓] Item #2: OAuth auto-refresh fired at least once in smoke window
        4 refresh audit row(s)
  [✓] Item #3: pg pool error handler best-effort audit count
        0 rows (server uptime was clean during the window — handler exists per test suite but never fired)
  [✓] Item #4: SendBlue reconciliation audit count
        0 gap rows (run pnpm ops:reconcile-sendblue during the smoke window to populate; expected 0 gaps if webhook delivery was healthy)
  [✓] sendblue_contact_status memory_signal row written for friend onboarded in smoke window
        1 contact_status row(s) for friend(s)
  [✓] Leak-canary scan: no raw secrets / connection strings in audit detail
        scanned 530 audit rows; zero hits

VERDICT: PASS  (operator must additionally confirm: ECONNRESET simulation did NOT crash the dev server; ops:reconcile-sendblue produced sensible output. Run smoke-evidence:v0.5.1 + smoke-evidence:v0.5.2 separately to confirm prior PASS criteria still hold.)
```

## 8. `smoke-evidence:v0.5.4` output (cross-tenant proof)

```
========================================================================
Phase v0.5.4 evidence summary — 16 criteria (12 v0.5.2 carry-forward + 4 NEW cross-tenant)
========================================================================
  [✓] C1: Friend B briefed BEFORE invite mint (audit briefed_confirmed=true + phone_class=real)
        1 briefed-real invite_issued row(s) in window
  [✓] C2: Invite token bound to a real (non-NANPA-fictional) E.164
        phone_class='real' confirmed on 1 v0.5.4 invite(s)
  [✓] C3: Friend B onboarded — new users row, is_founder=false, NOT Morris, phone hash populated
        1 Friend B user(s) onboarded in window; IDs: 8fbead5c…
  [!] C4: Privacy copy rendered at /onboard (operator-confirmed; substrate check only)
        No fomo.onboard.enabled audit row in window. Server may not have been booted on the v0.5.4 branch during the smoke.
  [✓] C5: Friend-safe Slack card posted for Friend B alert (operator-confirmed redaction)
        6 slack-review audit row(s) for Friend B actor_user_id. Operator must additionally confirm the card shape (no Snippet, footer "friend-owned (user redacted)").
  [✓] C6: Founder approved in Slack (fomo.slack.approval_captured, actor=founder)
        1 founder approval(s) captured in window
  [✓] C7: Real iMessage delivered to Friend B (fomo.send.succeeded for Friend B actor_user_id)
        1 successful send(s) on behalf of Friend B; destination_slug last-4 only
  [✓] C8: Friend B STOP from real iMessage thread (NOT synthetic curl)
        1 real-iMessage STOP(s) recorded for Friend B
  [✓] C9: memory_signals.stop_active row for Friend B (per-user keyspace, freshly written)
        1 Friend B stop_active row(s) updated within smoke window
  [✓] C10: Founder regression — at least one founder approved → sent during the smoke window
        1 founder approved→sent transition(s) in window
  [✓] C11: Leak-canary scan — no forbidden substrings in persisted detail
        scanned 533 audit + 2 memory + 28 transition rows; zero hits across 3 canary substring(s)
  [✓] C12: Friend B is_founder=false (no privilege escalation through onboard)
        1 Friend B user(s); zero have is_founder=true
  [✓] C13 (NEW): Morris's stop_active row UNTOUCHED throughout smoke window
        Morris stop_active row exists; updated_at predates smoke window (no cross-tenant write). user_id=25c1a707…
  [✓] C14 (NEW): Founder's stop_active row UNTOUCHED throughout smoke window
        Founder stop_active row exists; updated_at predates smoke window (no cross-tenant write).
  [✓] C15 (NEW): Distinct sendblue_contact_status rows per friend (no overwrite, Morris's row untouched)
        Friend B contact_status row(s)=1 (fresh), Morris contact_status row(s)=0 (NOT overwritten in window), founder rows=0. Per-user keyspace preserved.
  [✓] C16 (NEW): v0.5.3 hardening still functional (registry intact + contact lifecycle fired)
        7/7 hardening audits registered; sendblue_contact_status kind registered; 1 contact-lifecycle audit row(s) fired during Friend B onboarding.

VERDICT: PASS  (operator must additionally confirm: Friend B received iMessage on their real phone; Friend B texted STOP from their real iMessage thread; Friend B understood the privacy copy; Morris + founder were unaware of the smoke and their state matches the §0 baseline snapshot. Run smoke-evidence:v0.5.1 + v0.5.2 + v0.5.3 separately to confirm prior PASS criteria still hold.)
```

C4 yellow flag explanation: the script searches `audit_log` for `fomo.onboard.enabled`, but that event is currently emitted only as a structured stderr log line at boot (no audit row). The criterion is operator-confirmed in §9; the script's automated check is correctly downgraded from `✗` to `!`. Recommend lifting the boot event into `audit_log` in a future hardening pass so the script can pass it automatically.

## 9. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend B Slack card has NO Snippet section | ✅ | Verified visually during smoke; founder approved the right card (alert `36ffe6d8…`) without seeing email body in the card |
| Friend B Slack card footer reads "friend-owned (user redacted)" | ✅ | Verified visually |
| Friend B Slack card shows sender, subject, ranker `Why`, label, score | ✅ | Verified visually |
| Friend B Slack card `Why` text is a SUMMARY (not paraphrase containing body words) | ✅ | Verified visually |
| Founder Slack card (regression) STILL shows full Snippet + full footer | ✅ | Verified visually on alert `aa51b11f…` |
| Friend B received iMessage on their real iPhone | ✅ | Confirmed at 2026-06-04 ~14:09 UTC ("sheila got the text message. i see it!") |
| Friend B's STOP reply was an actual iMessage (not curl) | ✅ | `fomo.sendblue.stop_recorded` fired from real inbound webhook; `provider_message_id` SendBlue-shaped |
| Morris was not contacted during the smoke window | ✅ | Operator confirms out-of-band; Morris remains unaware. Final verification in §14 |

## 10. Friend B's experience (out-of-band feedback)

> Asked out-of-band immediately after smoke completion. Awaiting reply at report-draft time — fill in verbatim from her response, do NOT paraphrase or invent.

| Question | Friend B's answer (verbatim) |
|---|---|
| Was the privacy copy clear when you first read it? | "it was clear" |
| Did anything about onboarding feel surprising or alarming? | "alarming that google kept saying that the website was unverified and could be unsafe." |
| The iMessage you got — did the wording feel like a useful summary, or did it leak body text? | "it was pretty unnatural and didnt feel humna-like at all (it felt more like a robot) and summary was cut off after the first 5 words so i didnt get a good jist of the email." |
| Did STOP work the way you expected? | "yes. but brevio didnt send me a text to confirm whether STOP worked, which was what i was expecting." |
| Would you keep using Brevio post-beta? | "yes" — but separately requested account deletion post-beta (see §13 + §14). Read together: would in principle use it, but not ready to be left on right now. |
| Anything you wish had been clearer before you clicked Connect? | "Nope, i just wished the wording of the BREVIO text was written in a clearer, friendlier way-- like as uf my friend was texting me" |

(This is the most important part of v0.5.4 alongside §6 — the substrate and hardening are proven by v0.5.1–v0.5.3; THIS proves cross-tenant experience.)

## 11. Founder observations

| Observation | Note |
|---|---|
| Did briefing Friend B uncover anything you'd want to change in the privacy copy or briefing script? | The Brevio-side privacy copy held up (Friend B reported it was "clear" — §10 row 1). The *briefing* missed something material: Friend B was alarmed by Google's "unverified app" interstitial during OAuth and we hadn't warned her about it (§10 row 2). Future briefing script must include a heads-up: "you'll see a Google 'unverified app' / 'this app could be unsafe' warning during sign-in — that's expected for our beta until Google completes verification." Captured as §12 followup #6. |
| Did the cross-tenant baseline diff surface anything unexpected (any row movement at all)? | No. Diff was exactly the predicted shape: one added row for Friend B, zero modifications to Morris/founder/gm3258 rows. Cross-tenant isolation confirmed at the DB level. |
| Anything in audit_log that surprised you (unexpected actor_user_id, missing rows, contact-gate fires)? | First poll cycle pulled 113 messages → 6 alerts across 4 users in a single cycle, flooding Slack with founder/Morris/gm3258 cards alongside Friend B's. None were accidentally approved (operator was warned to ignore them mid-smoke), but this is noise worth addressing in a v0.5.5-class hardening — see §12 Followups item 1. |
| Did v0.5.3 hardening "just work" through this run, or was anything manually intervened? | Yes — `sendblue_contact_status` auto-registered Sheila at `/onboard`; reconciliation script not invoked (not needed; 0 gaps); OAuth refresh fired 4 times in window with no failures. One v0.5.3-orthogonal incident handled by a temporary diagnostic patch (see §12 Followups item 2). |
| What would you want different before a hypothetical Friend C? (Not committing to a Friend C — just capturing learnings.) | No notes. Per the three-friend-beta-cap rule (locked 2026-06-03), Friend C is OPTIONAL and not auto-scheduled. Default after v0.5.4 PASS is (a) fix onboarding gaps OR (b) move up the product stack — NOT (c) another friend. No specific gap surfaced by this run that would change that default. |

## 12. Verdict

✅ **PASS** — all 16 criteria green; §6 cross-tenant diff shows no Morris/founder writes; all four evidence scripts `VERDICT: PASS`; Friend B feedback being captured honestly out-of-band (§10 to be patched into this report once Sheila replies). **Next phase runs its own 6-question gate.**

☐ FAIL

Followups (non-blocking — file under v0.5.5 hardening candidates, NOT v0.5.4 PASS preconditions):

1. **Polling-after-STOP wastes compute + adds Slack noise.** After Sheila replied STOP at 14:11:51, two additional alerts for her (`6caae0d5` at 14:12:31, `6ed0822a` at 14:13:12) were created from her live inbox and posted to Slack as `queued_for_review`. Outbound is correctly gated by `stop_active` (the alerts would be refused if approved), so this is not a privacy leak — but it's wasted compute and a footgun if the founder ever clicks the wrong card. Candidate fix: short-circuit the alert-creation pipeline when `stop_active=true` for that user. Polling can continue (keeps Gmail cursor warm), but alerts should never be created.

2. **Drizzle wraps query errors and swallows the underlying postgres `cause`.** Boot failure at the migrations verifier surfaced only as `"Failed query: SELECT to_regclass($1) AS r\\nparams: public.alert_state_transitions"` — the real cause (`SSL/TLS required`, SQLSTATE `28000`) was buried on `err.cause`. Patched `apps/fomo/src/index.ts:1438-1450` to log `error_code`, `cause_message`, `cause_code`, `cause_stack`, and `stack`. This is a tiny observability-only change (no runtime behavior modified) and is included in the v0.5.4 PR. Do NOT revert — it's load-bearing for any future migration-verifier diagnosis.

3. **Stale shell `DATABASE_URL` exports silently override the env file.** Founder's `~/.zshrc` (or similar) exports a stale Render-staging `DATABASE_URL` that overrode the Neon URL in `apps/fomo/.env.3b3.local` when a terminal wasn't explicitly re-sourced. Cost: ~5 minutes mid-smoke to diagnose. Post-PASS aftercare item: purge the stale export from shell config. Runbook update: every smoke terminal must `set -a; source apps/fomo/.env.3b3.local; set +a` immediately before any `pnpm` command. Memory saved as `feedback_stale-database-url-shell-export.md`.

4. **`smoke-evidence-v0.5.4.ts` had two `= ANY($array)` query bugs** that crashed the script on first run against the live Postgres (`op ANY/ALL (array) requires array on right side`, SQLSTATE 42809). Drizzle's `sql` tagged template expands a JS array into N separate `$N` placeholders rather than binding it as a single `TEXT[]`. Fixed in this PR by rewriting both sites to `IN (${sql.join(...)})`. The unit tests for the script (if any) didn't catch it because they ran against PGlite, which apparently handles the same input shape differently. Worth a regression test against live Postgres (gated; same pattern as `feedback_inmemory-mock-divergence.md` from v0.5.1).

5. **`fomo.onboard.enabled` boot event is structured-log-only, not an `audit_log` row.** Caused C4 to be `!` (warn) in the evidence script even though privacy copy was operator-confirmed rendered. Candidate fix: emit a `fomo.onboard.enabled` audit row at boot so the C4 check passes automatically.

6. **Google OAuth "unverified app" warning was alarming to Friend B** (her §10 row 2: _"alarming that google kept saying that the website was unverified and could be unsafe."_). The Brevio OAuth app is in Testing mode in Google Cloud Console, which surfaces a scary interstitial Sheila had to click through. Real adoption blocker. Candidate fix: submit the OAuth app for Google verification (CASA assessment for restricted scopes — multi-week process). Worth lining up before any next beta cohort. Until then, the briefing script should explicitly warn friends: "you'll see a Google 'unverified app' warning — that's expected for our beta and means our verification request is still pending."

7. **iMessage copy tone is robotic and the summary appears truncated** (her §10 row 3: _"it was pretty unnatural and didnt feel humna-like at all (it felt more like a robot) and summary was cut off after the first 5 words so i didnt get a good jist of the email"_; reinforced by row 6: _"i just wished the wording of the BREVIO text was written in a clearer, friendlier way-- like as uf my friend was texting me"_). Two distinct issues bundled here: **(a)** voice/register of the outbound iMessage template needs rewriting in a warmer, friend-texting-friend tone — currently reads as machine-generated; **(b)** the email-body summary in the iMessage appears to be truncated to ~5 words, which kills the value prop of the alert (Sheila got the sender + subject but couldn't tell what the email was actually *about*). Investigate the outbound template / summary length budget. Both are v0.5.5-grade UX fixes, not v0.5.4 blockers.

8. **STOP confirmation reply is missing** (her §10 row 4: _"yes [STOP worked]. but brevio didnt send me a text to confirm whether STOP worked, which was what i was expecting."_). Sheila confirmed STOP succeeded via the server-side audit + her experience that no further alerts arrived, but she expected a "You're unsubscribed — text START to re-enable" reply from Brevio. Industry standard for SMS/iMessage opt-out flows. Candidate fix: emit a one-shot confirmation iMessage on STOP receipt. Small surface change; high trust-building value. Worth coupling with item #7's tone rewrite.

## 13. Sign-off

- Founder signature: Galiette Mita
- Friend B's first name + verbal consent to publish this report: Sheila — **yes** (verbal consent given out-of-band on 2026-06-04 to Galiette Mita; Sheila also requested full account deletion post-beta — actioned in §14 aftercare).
- Date: 2026-06-04

## 14. Aftercare confirmation

- [ ] Friend B was told the second-friend gate is complete (Step 24 of the runbook — to send after this report commits)
- [ ] Friend B was told they can keep texting STOP / START anytime (covered by §24 aftercare message)
- [x] Friend B was asked whether they want to remain onboarded post-beta (asked out-of-band 2026-06-04; **she chose deletion**)
- [ ] **DELETION SCHEDULED for "tomorrow / later this week"** (founder's preference, captured 2026-06-04). Sequence when executed: (a) POST Sheila's `refresh_token` to `https://oauth2.googleapis.com/revoke` to invalidate the Google grant programmatically (founder-side, no action required from Sheila); (b) DELETE FROM `oauth_tokens` WHERE user_id='8fbead5c-…'; (c) DELETE FROM `gmail_cursors` WHERE user_id='8fbead5c-…'; (d) DELETE FROM `users` WHERE id='8fbead5c-…'. Memory_signals (stop_active, sendblue_contact_status) and audit_log rows retain her UUID for ~retention window — not personally identifying.
- [x] Morris's state was re-verified after smoke (stop_active row matches §0.A baseline) — final re-check at 2026-06-04 ~14:25 UTC immediately before commit: Morris's row `updated_at=2026-06-01 22:04:04.182916+00` is byte-identical to §6.A baseline; founder + gm3258 rows also unchanged; only Sheila's added row in the diff. `/tmp/v0.5.4-final-stop-active.txt` matches `/tmp/v0.5.4-post-stop-active.txt` exactly.

## 15. What v0.5.4 PASS does NOT promise

v0.5.4 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- A third friend — its own gate (could be skipped entirely per the three-friend-beta-cap memory; Friend C is OPTIONAL)
- Public self-serve onboarding — out indefinitely
- Auto-send — its own gate
- SendBlue dedicated-line upgrade — out
- Periodic reconciliation worker — still on-demand from v0.5.3
- Production scaling sprint — out
- Dashboard — out
- Calendar / Drafting / MCP / browser automation — L2+ surfaces
- Android/SMS fallback — its own future smoke

The next phase is decided AT THE NEXT 6-question gate, with the founder's full attention on whatever Friend B's experience taught them.
