# Phase v0.5.4 Smoke Test Report — Second-Friend Cross-Tenant Smoke

> Filled after running every step in `smoke-test-v0.5.4-second-friend.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.4.md` once `VERDICT: PASS` on ALL
> FOUR evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4) AND the
> cross-tenant baseline diff in §6 shows no Morris/founder writes.
>
> **v0.5.4 PASS does NOT auto-unlock v1.0 or broad beta.** The next
> phase runs its own 6-question gate.

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.4-second-friend-smoke`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Friend B's first name only** (PII boundary — do NOT paste full name): _<first name>_
**Friend B's phone (last 4 only):** _<last 4>_
**Friend B's Gmail domain (redact local-part):** _<e.g. ***@gmail.com>_
**Friend B's device:** _<iPhone model / iOS version — REQUIRED; Android disqualifies>_
**Morris user_id (for cross-tenant diff):** _<UUID — same value as `FOMO_V0_5_4_MORRIS_USER_ID`>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_v0.5.2.md` AND `docs/SMOKE_REPORT_v0.5.3.md` on `main` with `VERDICT: PASS`
- [ ] Friend B briefed out-of-band on _<date / channel>_
- [ ] Briefing covered all five topics (Gmail readonly, founder review, STOP, beta status, expected volume)
- [ ] Friend B agreed verbally to participate
- [ ] Friend B's phone is iMessage-capable iPhone
- [ ] Friend B's Gmail is actively used
- [ ] Friend B's Gmail added to Google Cloud Console test-user list
- [ ] Friend B reachable during the smoke window
- [ ] Morris is NOT being notified of this smoke (he remains unaware)
- [ ] §0.A baseline snapshot captured into `/tmp/v0.5.4-baseline-stop-active.txt` and `/tmp/v0.5.4-baseline-contact-status.txt` BEFORE smoke start

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_4_FRIEND_BRIEFED` | ☐ | `true` (briefing assertion) |
| `FOMO_V0_5_4_FRIEND_NAME` | ☐ | first name only |
| `FOMO_V0_5_4_BASELINE_CONFIRMED` | ☐ | `true` after §0.A capture |
| `FOMO_V0_5_4_MORRIS_USER_ID` | ☐ | Morris's UUID |
| `FOMO_V0_5_4_LEAK_CANARIES` | ☐ | `brevio-canary-cccc,brevio-canary-dddd` (different from v0.5.2) |
| `FOMO_V0_5_4_WINDOW_HOURS` | ☐ | default 24h |
| `FOMO_GMAIL_POLLING_MAX_CYCLES` | ☐ | ≥ 60 |
| `FOMO_OUTBOUND_MAX_CYCLES` | ☐ | ≥ 60 |
| `FOMO_FRIEND_BETA_BASE_URL` | ☐ | HTTPS ngrok URL |

## 3. PASS criteria (16 — 12 v0.5.2 carry-forward + 4 NEW cross-tenant)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| 1 | Friend B briefed BEFORE invite mint | `fomo.onboard.invite_issued` audit has `briefed_confirmed=true`, `phone_class='real'` | ☐ |
| 2 | Invite token bound to Friend B's REAL E.164 (NOT 555-fictional) | audit `phone_class='real'`; issue script refused without `--confirm-briefed` | ☐ |
| 3 | Friend B onboarded via `/onboard` on their own device with their own Gmail | NEW `users` row, `is_founder=false`, `phone_e164_hash` populated, NOT Morris's UUID | ☐ |
| 4 | Friend B received privacy copy at consent screen | Operator-confirmed: Friend B reported seeing it; matched `docs/privacy-copy-v0.5.md` | ☐ |
| 5 | Friend-safe Slack card rendered for Friend B (no body/snippet/headers/message_id leak) | Operator-confirmed: card showed sender + subject + ranker `Why` only; footer "friend-owned (user redacted)" | ☐ |
| 6 | Founder approved in Slack | `fomo.slack.approval_captured` audit, actor=founder, alert.user_id=Friend B | ☐ |
| 7 | Real iMessage delivered to Friend B | `fomo.send.succeeded` for Friend B's `actor_user_id`, `provider_status=QUEUED`, `destination_slug=<Friend B's last 4>`. **Friend B confirmed receipt out-of-band.** | ☐ |
| 8 | Friend B texted STOP from real iMessage | `fomo.sendblue.stop_recorded` with `actor_user_id=Friend B`, `provider_message_id` is Apple/SendBlue UUID-shaped | ☐ |
| 9 | Per-friend STOP isolation — Friend B's `stop_active=true` row written | `memory_signals.stop_active` row keyed to Friend B's user_id, `active=true`, `source=user_confirmed` | ☐ |
| 10 | Founder regression during the same smoke window | A founder alert chained `detected → ranked → queued_for_review → approved → sent`; real iMessage to founder phone | ☐ |
| 11 | Leak-canary scan clean across Friend B's content | `smoke-evidence:v0.5.4` PASS with `FOMO_V0_5_4_LEAK_CANARIES` set + canary present in Friend B's test email body | ☐ |
| 12 | Friend B's `users.is_founder=false` | DB query confirms; no privilege escalation through `/onboard` | ☐ |
| **13** | **NEW — Morris's `stop_active` row UNTOUCHED throughout smoke window** | `memory_signals.stop_active` for Morris's user_id has `updated_at` predating smoke start (matches §0.A baseline); §6 diff shows no modification | ☐ |
| **14** | **NEW — Founder's `stop_active` row UNTOUCHED throughout smoke window** | `memory_signals.stop_active` for founder (if exists) has `updated_at` predating smoke start | ☐ |
| **15** | **NEW — Distinct `sendblue_contact_status` rows per friend** | Morris's row NOT overwritten in window; Friend B's row freshly written; founder's row (if exists) untouched. Per-user keyspace preserved. | ☐ |
| **16** | **NEW — v0.5.3 hardening still functional** | 7/7 hardening audit actions registered; `sendblue_contact_status` kind registered; at least one of `fomo.sendblue.contact_registered`/`_failed` fired during Friend B onboarding | ☐ |

## 4. `smoke-evidence:v0.5.1` output (substrate)

```
…
```

## 5. `smoke-evidence:v0.5.2` output (first-friend specifics still hold)

```
…
```

(If the wall-clock window slid past v0.5.2's last audit row, set `FOMO_V0_5_2_WINDOW_HOURS` to widen — known followup D from v0.5.3.)

## 6. Cross-tenant baseline diff (THE v0.5.4 LOAD-BEARING SECTION)

### §6.A `stop_active` baseline (captured §0.A, BEFORE smoke):

```
<paste /tmp/v0.5.4-baseline-stop-active.txt>
```

### §6.B `stop_active` post-smoke:

```
<paste /tmp/v0.5.4-post-stop-active.txt>
```

### §6.C diff:

```
<paste /tmp/v0.5.4-stop-active.diff>
```

**Expected diff shape:**
- Exactly ONE row added: Friend B's stop_active=true with `updated_at` inside the smoke window.
- ZERO modifications to Morris's row (Morris's `updated_at`, `detail`, `source` all identical between baseline and post).
- ZERO modifications to founder's row.

**Operator confirms:** ☐ Yes, the diff shows only Friend B's added row; Morris and founder are byte-identical between baseline and post.

### §6.D `sendblue_contact_status` baseline + post diff:

```
<paste baseline + post + diff for sendblue_contact_status>
```

**Operator confirms:** ☐ Friend B has a fresh `sendblue_contact_status` row; Morris's row is unchanged.

## 7. `smoke-evidence:v0.5.3` output (hardening still wired)

```
…
```

## 8. `smoke-evidence:v0.5.4` output (cross-tenant proof)

```
…
```

## 9. Operator-confirmed visual checks

| Check | Confirmed? | Notes |
|---|---|---|
| Friend B Slack card has NO Snippet section | ☐ | _<paste a redacted screenshot or describe>_ |
| Friend B Slack card footer reads "friend-owned (user redacted)" | ☐ | |
| Friend B Slack card shows sender, subject, ranker `Why`, label, score | ☐ | |
| Friend B Slack card `Why` text is a SUMMARY (not paraphrase containing body words) | ☐ | |
| Founder Slack card (regression) STILL shows full Snippet + full footer | ☐ | |
| Friend B received iMessage on their real iPhone | ☐ | _<friend confirmed at YYYY-MM-DD HH:MM>_ |
| Friend B's STOP reply was an actual iMessage (not curl) | ☐ | |
| Morris was not contacted during the smoke window | ☐ | (operator-confirmed; out-of-band) |

## 10. Friend B's experience (out-of-band feedback)

| Question | Friend B's answer |
|---|---|
| Was the privacy copy clear when you first read it? | _<…>_ |
| Did anything about onboarding feel surprising or alarming? | _<…>_ |
| The iMessage you got — did the wording feel like a useful summary, or did it leak body text? | _<…>_ |
| Did STOP work the way you expected? | _<…>_ |
| Would you keep using Brevio post-beta? | _<…>_ |
| Anything you wish had been clearer before you clicked Connect? | _<…>_ |

(This is the most important part of v0.5.4 alongside §6 — the substrate and hardening are proven by v0.5.1–v0.5.3; THIS proves cross-tenant experience.)

## 11. Founder observations

| Observation | Note |
|---|---|
| Did briefing Friend B uncover anything you'd want to change in the privacy copy or briefing script? | _<…>_ |
| Did the cross-tenant baseline diff surface anything unexpected (any row movement at all)? | _<…>_ |
| Anything in audit_log that surprised you (unexpected actor_user_id, missing rows, contact-gate fires)? | _<…>_ |
| Did v0.5.3 hardening "just work" through this run, or was anything manually intervened? | _<…>_ |
| What would you want different before a hypothetical Friend C? (Not committing to a Friend C — just capturing learnings.) | _<…>_ |

## 12. Verdict

☐ **PASS** — all 16 criteria green; §6 cross-tenant diff shows no Morris/founder writes; all four evidence scripts `VERDICT: PASS`; Friend B feedback captured honestly. **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

Failures / followups:

- _…_

## 13. Sign-off

- Founder signature: _<name>_
- Friend B's first name + verbal consent to publish this report: _<first name> — yes / no>_
- Date: _<YYYY-MM-DD>_

## 14. Aftercare confirmation

- [ ] Friend B was told the second-friend gate is complete
- [ ] Friend B was told they can keep texting STOP / START anytime
- [ ] Friend B was asked whether they want to remain onboarded post-beta
- [ ] If Friend B opted out: their `users` + `oauth_tokens` + `gmail_cursors` rows deleted; Google OAuth revoked on Google's side
- [ ] Morris's state was re-verified after smoke (stop_active row matches §0.A baseline)

## 15. What v0.5.4 PASS does NOT promise

v0.5.4 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- A third friend — its own gate (could be skipped entirely)
- Public self-serve onboarding — out indefinitely
- Auto-send — its own gate
- SendBlue dedicated-line upgrade — out
- Periodic reconciliation worker — still on-demand from v0.5.3
- Production scaling sprint — out
- Dashboard — out
- Calendar / Drafting / MCP / browser automation — L2+ surfaces
- Android/SMS fallback — its own future smoke

The next phase is decided AT THE NEXT 6-question gate, with the founder's full attention on whatever Friend B's experience taught them.
