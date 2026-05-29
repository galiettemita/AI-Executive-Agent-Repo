# Phase 3G Smoke Test Report — Full Founder Demo (v0.1 milestone)

> Filled after running every step in
> [`smoke-test-3g-full-demo.md`](smoke-test-3g-full-demo.md).
> Commits as `docs/SMOKE_REPORT_3G.md` with `VERDICT: PASS`. **v0.1
> is declared done when this report lands on `main`. v0.5 friend beta
> cannot begin until then. Phase 3G.1 — Production Hardening — runs
> the standard 6-question gate before v0.5 (see
> `project_3g1-production-hardening-candidates` memory).**

---

**Founder:** Galiette Mita
**Run date:** 2026-05-29 01:00–01:25 UTC
**Branch:** `phase3g-full-demo-smoke`
**Commit SHA at run time:** `9a19f23af28f349232273e61f60699bf67d27e54` + 1 evidence-script patch (see §6.G note)
**Demo alert_id (auto-discovered by evidence script):** `ae95aadc-5870-46d4-80d9-9d5694ffae68`
**Founder Gmail account used:** techsmarterusa@gmail.com
**Founder phone number used (4-char suffix only):** `3459`
**Test email subject:** `Re: Greenoaks investor intro — Thursday meeting?`
**ngrok subdomain used:** `unshivering-interaulic-beatriz.ngrok-free.dev`

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [x] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3E2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3F2.md` on `main` with `VERDICT: PASS`
- [x] All 5 Neon migrations applied (0000_init through 0004_inbound_replies)
- [x] SendBlue dashboard webhook URL = `https://unshivering-interaulic-beatriz.ngrok-free.dev/sendblue/inbound`
- [x] Gmail OAuth re-auth completed; `oauth_tokens.needs_reauth = false` for `founder` (confirmed at 01:03:20 UTC, `obtained_at = 2026-05-29 01:03:20.912+00`)

---

## 2. Env vars (redacted)

| Var                                | Set? | Notes                                                                |
| ---------------------------------- | ---- | -------------------------------------------------------------------- |
| `DATABASE_URL`                     | ✅    | Neon Postgres (len 116)                                              |
| `BREVIO_TOKEN_KEK`                 | ✅    | 32-byte (len 44 base64)                                              |
| `BREVIO_OAUTH_STATE_KEY`           | ✅    | 32-byte                                                              |
| `BREVIO_SESSION_SIGNING_KEY`       | ✅    | 32-byte                                                              |
| `GOOGLE_CLIENT_ID/SECRET`          | ✅    |                                                                      |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ✅    |                                                                      |
| `OPENAI_API_KEY`                   | ✅    |                                                                      |
| `FOMO_GMAIL_POLLING_ENABLED`       | ✅    | `true`                                                               |
| `FOMO_RANKER_ENABLED`              | ✅    | `true`                                                               |
| `FOMO_SLACK_REVIEW_ENABLED`        | ✅    | `true`                                                               |
| `SLACK_BOT_TOKEN`                  | ✅    | `xoxb-...`                                                           |
| `SLACK_FOUNDER_CHANNEL_ID`         | ✅    |                                                                      |
| `SLACK_SIGNING_SECRET`             | ✅    |                                                                      |
| `SLACK_FOUNDER_USER_ID`            | ✅    | actor_slug `L3RA` observed in audit detail                           |
| `FOMO_SEND_ENABLED`                | ✅    | `true`                                                               |
| `SENDBLUE_API_KEY_ID`              | ✅    |                                                                      |
| `SENDBLUE_API_SECRET_KEY`          | ✅    |                                                                      |
| `SENDBLUE_FROM_NUMBER`             | ✅    | E.164 `+12143547196` (from SendBlue dashboard)                       |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ✅    | E.164, slug `3459`                                                   |
| `FOMO_FOUNDER_USER_ID`             | ✅    | `founder`                                                            |
| `FOMO_SENDBLUE_INBOUND_ENABLED`    | ✅    | `true`                                                               |
| `SENDBLUE_WEBHOOK_SECRET`          | ✅    | Scenario A (plain `sb-signing-secret` header) confirmed by 3F.2      |
| `SENDBLUE_WEBHOOK_SECRET_HEADER`   | ✅    | Default `sb-signing-secret`                                          |
| `SENDBLUE_INBOUND_PUBLIC_URL`      | ⚠️    | Not set in env (informational only — actual URL in SendBlue dashboard)|
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ✅    | 60 (initial), bumped to 120 after first cap hit                      |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ✅    | 60 → 120                                                             |
| `BREVIO_DEV_MODE`                  | ✅    | UNSET                                                                |
| `FOMO_AUTO_SEND_ENABLED`           | ✅    | UNSET / false                                                        |
| `FOMO_FRIEND_BETA_ENABLED`         | ✅    | UNSET / false                                                        |

---

## 3. Commands run

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# §1 Gmail re-auth (3F.2 carryover — needs_reauth=true)
#   1. Brief dev server start (background)
#   2. Mint founder session token (from apps/fomo/ — POST /oauth/google/start
#      is session-authenticated; documented in
#      [[brevio-oauth-google-reauth-procedure]] memory)
#   3. POST /oauth/google/start with Bearer session → get authorize_url
#   4. open "$AUTHORIZE_URL" → Google consent → gmail.readonly only → Allow
#   5. Verify needs_reauth flipped to false
#   6. Kill the OAuth-only dev server

pnpm --filter @brevio/fomo run preflight:3g    # ✓ passed (1 informational warning)

# Pre-demo cleanup: stop_active=true carried over from 3F.2 (2026-05-28 01:12);
# cleared via direct SQL UPDATE (source='smoke_setup'). Bonus: START path
# exercised later when SendBlue's carrier-level opt-out forced the issue — see §6.E.

FOMO_GMAIL_POLLING_MAX_CYCLES=60 \
FOMO_OUTBOUND_MAX_CYCLES=60 \
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g.log

# T2: ngrok already running on https://unshivering-interaulic-beatriz.ngrok-free.dev

# §4 Sent FOMO-worthy test email from founder Gmail to self
#   Subject: "Re: Greenoaks investor intro — Thursday meeting?"
#   Body: standard Greenoaks/Eli/Thursday shape

# §5–6 Watched chain; approved in Slack; received iMessage; replied 'tomorrow'
# §7 Cycle cap reached (initially set to 60); restarted server with cap=120

pnpm --filter @brevio/fomo run smoke-evidence:3g 2>&1 | tee /tmp/fomo-3g-evidence.log

# §9 Clean-stop verification (every kill switch off)
FOMO_GMAIL_POLLING_ENABLED=false FOMO_RANKER_ENABLED=false \
FOMO_SLACK_REVIEW_ENABLED=false FOMO_SEND_ENABLED=false \
FOMO_SENDBLUE_INBOUND_ENABLED=false \
pnpm --filter @brevio/fomo dev > /tmp/fomo-3g-cleanstop.log 2>&1 &
```

---

## 4. Boot-time confirmation

Demo-run boot log (verbatim, secrets redacted):

```
fomo.gmail.polling.enabled        interval_ms: 10000, cycle_cap: 60 → 120
fomo.send.enabled                 founder_user_id: founder, founder_phone_configured: true, auto_send_enabled: false
fomo.sendblue.inbound.enabled     inbound_route_mounted: true, webhook_secret_header: sb-signing-secret
fomo.slack.review.enabled         (interactivity route mounted)
fomo.poll.enabled                 interval_ms: 10000, cycle_cap: 60, ranker_enabled: true, slack_review_enabled: true
fomo.outbound.enabled             interval_ms: 10000, cycle_cap: 60, auto_send_enabled: false
fomo.server.listening             port: 8080, store_backend: postgres, oauth_google_wired: true,
                                  polling_enabled: true, ranker_enabled: true, slack_review_enabled: true,
                                  slack_interactivity_route_mounted: true, send_enabled: true,
                                  outbound_worker_started: true, sendblue_inbound_route_mounted: true
```

- [x] All `fomo.*.enabled` lines present
- [x] `sendblue_inbound_route_mounted: true`
- [x] `gmail_polling_started: true`
- [x] `webhook_secret_header: sb-signing-secret` (matches 3F.2 §5 confirmed Scenario A)

---

## 5. Demo alert state trail (LOAD-BEARING)

Verbatim from `alert_state_transitions` for `alert_id = ae95aadc-5870-46d4-80d9-9d5694ffae68`:

```
detected          → ranked              ranker labeled message_id=19e714e6290c786b as important (score 0.94)  at 2026-05-29 01:18:20.744Z
ranked            → queued_for_review   slack.founder_review posted; slack_ts=1780017500.939999                at 2026-05-29 01:18:21.020Z
queued_for_review → approved            slack:approved actor_slug=L3RA                                         at 2026-05-29 01:18:51.725Z
approved          → sent                sendblue ok: provider_status=QUEUED                                    at 2026-05-29 01:18:57.301Z
sent              → replied             sendblue:snooze hint=tomorrow                                          at 2026-05-29 01:19:45.430Z
replied           → snoozed             sendblue:snooze hint=tomorrow until=2026-05-30T01:19:45.371Z           at 2026-05-29 01:19:45.486Z
```

- [x] Six transitions present, in this exact order
- [x] alert_id is a real UUID (not `smoke3f2-*` synthetic)
- [x] No `failed` / `send_status_unknown` states in the trail
- [x] `snooze_until = 2026-05-30T01:19:45.371Z` is +1 day from reply time (correct `tomorrow` math)
- [x] End-to-end wall time: detected (01:18:20.744) → snoozed (01:19:45.486) = **84.7 seconds**

---

## 6. PASS criteria checklist

**A. Gmail connected**
- `oauth_tokens` row: `user=founder needs_reauth=false obtained_at=2026-05-29 01:03:20.912+00`
- `gmail_cursors` row: `user=founder history_id=2007474`

**B. Ranker works**
- `rank_results` row: `id=31 label=important score=0.94 model=gpt-5-mini` (target was ≥ 0.85)

**C. Slack review works**
- `audit_log` row: `fomo.slack.approval_captured` at `2026-05-29 01:18:51.725Z`, `detail.alert_id=ae95aadc-5870-46d4-80d9-9d5694ffae68`, `user_slug=L3RA`
- State transition: `queued_for_review → approved` at `2026-05-29 01:18:51.725Z`

**D. SendBlue send works**
- `audit_log` row: `fomo.send.succeeded` at `2026-05-29 01:18:57.30Z`, `detail.alert_id=ae95aadc..., provider_status=QUEUED`
- `tool_invocations` row: `id=47 tool_id=sendblue.send_user_message status=success occurred_at=01:18:57.241Z` (60ms before the state transition — clean causal ordering)
- State transition: `approved → sent` at `2026-05-29 01:18:57.301Z`
- iMessage delivered to founder phone (visually confirmed)
- **Note: this is the SECOND attempt.** The first natural alert (`d9728e57…`) was rejected by SendBlue with `HTTP 400 OPTED_OUT` due to yesterday's STOP creating a carrier-level opt-out — see §8 observations. Cleared by texting `START`; the second attempt succeeded clean.

**E. Reply parser works**
- `audit_log` row: `fomo.sendblue.reply_parsed` at `2026-05-29 01:19:45.542Z`, `intent=snooze intent_source=classifier snooze_hint=tomorrow`
- State transition: `sent → replied` at `2026-05-29 01:19:45.430Z`
- Bonus: also `fomo.sendblue.start_recorded` event captured at `2026-05-29 01:15:54.088Z` (during the SendBlue opt-out fix — START path proven, which 3F.2 had skipped)

**F. Memory / feedback writes**
- `feedback_events` rows tied to demo alert (2):
  - `id=8 kind=founder_approved alert_id=ae95aadc...`
  - `id=9 kind=user_snoozed alert_id=ae95aadc...`
- `memory_signals` touched during demo window: 1 (`stop_active` flipped twice: once via SQL clear, once via START text)

**G. No duplicate sends**
- `tool_invocations(sendblue.send_user_message)` rows for demo alert (via time-window correlation, ±60s of the `approved → sent` transition): exactly **1**.
- **Evidence-script patch made during this run:** the initial 3G evidence script tried to correlate tool_invocations to alert_id via `metadata::text LIKE '%alert_id%'`, but the v0.1 runtime writes `tool_invocations.metadata = NULL` — no alert_id traceability in the metadata column. Patched to correlate by time window (±60s of the `approved → sent` transition), matching the pattern already used for Slack approval and reply parser. This is logged in the 3G.1 hardening candidates as item #6.

**H. No raw body leakage**
- Leak-canary scan: zero hits across **600 persisted rows** (500 audit + 47 tool_invocations + 9 feedback + 36 transitions + 1 memory_signal + 7 inbound_replies).
- Scanners covered: literal `FOMO_FOUNDER_PHONE_NUMBER`, literal `SENDBLUE_WEBHOOK_SECRET`, literal `SENDBLUE_API_KEY_ID`, plus forbidden key set (`body_plain`, `body_html`, `attachments`, `headers`, `content`, etc.), plus forbidden value patterns (long base64 blobs, raw `Received:` headers, hex strings ≥32 chars, full E.164 phone numbers).

---

## 7. `pnpm smoke-evidence:3g` output (LOAD-BEARING)

```
Phase 3G evidence — querying Neon Postgres substrate

Gmail oauth_tokens (founder): 1 row(s)
  user=founder needs_reauth=false obtained_at=2026-05-29T01:03:20.912Z
Gmail cursors (founder): 1 row(s)
  user=founder history_id=2007474 updated_at=2026-05-29T01:19:45.... Z

Demo alert candidate: ae95aadc-5870-46d4-80d9-9d5694ffae68
  (most recent natural alert with both approved→sent AND sent→replied transitions)

Demo alert state trail (6 transitions):
  detected → ranked  reason="ranker labeled message_id=19e714e6290c786b as important (score 0.94)"  at=2026-05-29T01:18:20.744Z
  ranked → queued_for_review  reason="slack.founder_review posted; slack_ts=1780017500.939999"  at=2026-05-29T01:18:21.020Z
  queued_for_review → approved  reason="slack:approved actor_slug=L3RA"  at=2026-05-29T01:18:51.725Z
  approved → sent  reason="sendblue ok: provider_status=QUEUED"  at=2026-05-29T01:18:57.301Z
  sent → replied  reason="sendblue:snooze hint=tomorrow"  at=2026-05-29T01:19:45.430Z
  replied → snoozed  reason="sendblue:snooze hint=tomorrow until=2026-05-30T01:19:45.371Z"  at=2026-05-29T01:19:45.486Z

Demo rank_result: id=31 label=important score=0.94 model=gpt-5-mini
Demo Slack approval audits: 1 event(s)
Demo SendBlue send_user_message invocations: 1
Demo feedback_events tied to alert: 2 row(s)
  id=8 kind=founder_approved
  id=9 kind=user_snoozed
Demo reply audit events (±30s of sent→replied): 2
  id=706 action=fomo.sendblue.inbound_received result=success at=2026-05-29T01:19:40.703Z
  id=710 action=fomo.sendblue.reply_parsed result=success at=2026-05-29T01:19:45.542Z

Scanning for leak canaries in audit_log + tool_invocations.metadata + feedback_events.detail + alert_state_transitions.reason + memory_signals.detail + inbound_replies ...
  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value)
  (scanning for the literal SENDBLUE_WEBHOOK_SECRET value)
  (scanning for the literal SENDBLUE_API_KEY_ID value)
  ✓ no forbidden keys or value patterns found

========================================================================
Phase 3G evidence summary
========================================================================
  [✓] Gmail connected
        oauth_tokens(founder) needs_reauth=false; cursors history_id=2007474
  [✓] Ranker works
        rank_result id=31 label=important score=0.94 model=gpt-5-mini
  [✓] Slack review works
        queued_for_review → approved transition + 1 fomo.slack.approval_captured event(s)
  [✓] SendBlue send works
        approved → sent transition + exactly 1 tool_invocations(sendblue.send_user_message) row tied to alert
  [✓] Reply parser works
        sent → replied transition + fomo.sendblue.reply_parsed event in ±30s window
  [✓] Memory / feedback writes
        2 feedback_events tied to alert + 1 memory_signals touched in last 2h
  [✓] No duplicate sends
        Exactly 1 tool_invocations(sendblue.send_user_message) for demo alert
  [✓] No raw body / phone / secret leakage
        Scanned 500 audit + 47 tool_invocations + 9 feedback + 36 transition + 1 memory_signal + 7 inbound_replies rows; zero hits.

VERDICT: PASS  (demo alert: ae95aadc-5870-46d4-80d9-9d5694ffae68)
Phase 3G — v0.1 demo gate is GREEN. Fill in docs/SMOKE_REPORT_3G.md and merge.
```

---

## 8. Founder observations

| Observation | Note |
|---|---|
| Did exactly one alert flow from email to snooze without intervention? | **Yes — on the second attempt.** First natural alert (`d9728e57…`, Greenoaks email v1) was blocked at SendBlue with `HTTP 400 OPTED_OUT, error_reason: SpamRule`. Root cause: yesterday's 3F.2 STOP text created a carrier-level opt-out at SendBlue that survived clearing our local `stop_active` signal. After texting `START` (which cleared the SendBlue opt-out AND fired `fomo.sendblue.start_recorded`), I sent a second email with a slightly different subject (so it was a new `message_id`) and the second alert (`ae95aadc…`) flowed clean from `detected` to `snoozed` in **84.7 seconds wall time** with zero intervention beyond the Slack approval click and the `tomorrow` SMS reply. The first failure surfaced a real Phase 3G.1 hardening candidate (see #2 in the 3G.1 catalog memory). |
| Was the iMessage text on your phone clear and professional (no "I think you should…" hedging, no [REDACTED] artifacts)? | **Yes.** The deterministic founder-text template rendered the email's key information cleanly. No artifacts. |
| Did the ranker label the test email `important` on the first try, or did you need to revise the subject/body? | **First try, both attempts** — `score=0.92` on the first email, `score=0.94` on the second (slightly different subject). The Greenoaks/Eli/Thursday shape locked in `important` reliably. |
| Did anything in the chain fail and need a retry (Slack click double-fire, SendBlue rate limit, ngrok stall)? | **One real failure** (SendBlue OPTED_OUT, above). No Slack double-fire. No ngrok stall. Polling worker did hit `FOMO_GMAIL_POLLING_MAX_CYCLES=60` mid-debug; bumped to 120 and restarted. |
| Did the leak-canary scan stay green? | **Yes.** 600 persisted rows scanned (500 audit_log + 47 tool_invocations + 9 feedback + 36 transitions + 1 memory_signal + 7 inbound_replies). Zero hits on phone-number literal, SendBlue webhook secret literal, SendBlue API key literal, or any forbidden key/value pattern. |
| Anything else surprising? | (a) The SendBlue OPTED_OUT carryover is a real friend-beta blocker — captured as item #2 in the [[3g1-production-hardening-candidates]] catalog. (b) The 3G evidence script needed a one-line patch (correlate tool_invocations by time window, not by alert_id in NULL metadata) — captured as item #6. (c) The v0.1 demo timing: 84.7 seconds end-to-end on the successful run is fast enough to feel real. (d) Gmail re-auth carryover from 3F.2 was caught early via the runbook's §1 check — exactly what the runbook is for. |

---

## 9. Clean-stop confirmation

Restarted with every kill switch off. Boot log (verbatim, from `/tmp/fomo-3g-cleanstop.log`):

```
fomo.ranker.disabled              FOMO_RANKER_ENABLED is not "true"; ranker dormant (rank_results stays empty)
fomo.slack.review.disabled        FOMO_SLACK_REVIEW_ENABLED is not "true"; Slack candidate-review path dormant
                                  (alerts table stays empty; /slack/interactivity route NOT mounted)
fomo.send.disabled                FOMO_SEND_ENABLED is not "true"; outbound sender worker dormant
fomo.sendblue.inbound.disabled    FOMO_SENDBLUE_INBOUND_ENABLED is not "true"; /sendblue/inbound route NOT mounted
fomo.poll.disabled                FOMO_GMAIL_POLLING_ENABLED is not "true"; polling worker dormant
fomo.outbound.disabled            FOMO_SEND_ENABLED is not "true"; outbound-sender worker dormant
fomo.server.listening             port: 8080, store_backend: postgres, oauth_google_wired: true,
                                  polling_enabled: false, ranker_enabled: false,
                                  slack_review_enabled: false, slack_interactivity_route_mounted: false,
                                  send_enabled: false, outbound_worker_started: false,
                                  sendblue_inbound_route_mounted: false
```

- [x] `/sendblue/inbound` route NOT mounted → guaranteed HTTP 404 (route doesn't exist on the server)
- [x] `/slack/interactivity` route NOT mounted → guaranteed HTTP 404
- [x] No `fomo.sendblue.*` / `fomo.send.*` / `fomo.slack.*` / `fomo.poll.cycle` rows wrote to audit after clean-stop restart

---

## 10. Verdict

**[x] PASS** — every required check in §6 is green, demo-alert state trail in §5 is the complete 6 transitions, `smoke-evidence:3g` printed `VERDICT: PASS`, no leaks across 600 rows, clean stop confirmed. **v0.1 is done.**

[ ] FAIL

Failures / followups:

- **None blocking PASS.** Three documented followups, ALL non-blocking and ALL captured in `project_3g1-production-hardening-candidates` memory:
  1. **SendBlue carrier-level OPTED_OUT decoding** (3G.1 catalog item #2) — must-have for v0.5 friend beta. A friend who types STOP and later wants alerts back would otherwise hit the same trap. Production should parse SendBlue's 400 response body, surface `error_message=OPTED_OUT` as a named audit reason, and write `stop_active=true` locally when carrier-level opt-out is detected.
  2. **Migration verification at boot** (3G.1 catalog item #1) — must-have for v0.5. Friend beta deploys will have ≥1 new migration regularly; we need a fail-loud check.
  3. **tool_invocations.metadata alert_id traceability** (3G.1 catalog item #6) — should-have. Affects ops debuggability, not user-facing safety.

---

## 11. v0.1 milestone sign-off

I (founder) confirm that as of **2026-05-29**:

- A real email from my real Gmail account (`techsmarterusa@gmail.com`) was picked up by the FOMO polling worker, ranked by the v0.1 ranker as `important` with `score=0.94`, posted to my real Slack channel for review, approved by me in Slack, sent to my real phone (`...3459`) via SendBlue as an iMessage, replied to by me with the soft intent `tomorrow`, parsed by the reply parser via the OpenAI classifier (`intent_source=classifier`), and snoozed in the FOMO state store with `snooze_until = 2026-05-30T01:19:45.371Z` — all with **zero raw email body** persisted anywhere across 600 inspected rows, **zero duplicate sends**, and **zero intervention** beyond the Slack approval click and the SMS reply.
- The substrate is real. **v0.1 is done.** Friend beta is unblocked, gated only behind Phase 3G.1 — Production Hardening — which runs the standard 6-question gate before v0.5 kickoff.

- Founder signature: Galiette Mita
- Date: 2026-05-29

---

## 12. What v0.1 PASS does NOT promise

- Snooze resurface scheduler — explicitly out of v0.1 (a snoozed alert at `snooze_until` does not auto-resurface; that's a future v0.3 or v0.5 item)
- Friend beta — gated behind v0.5 (and behind Phase 3G.1 — Production Hardening — first)
- Auto-send — gated behind its own gate after v0.5 stability
- Group chats — explicitly out of v0.1, v0.5
- Proactive follow-up messages — explicitly out
- Calendar / Drafting / Sending email / MCP tools / Autonomous — L2+ surfaces, all out of v0.1 / v0.5
- Multi-tenant scale, rate limits, queue retries, observability/dashboards — operational maturity is a separate concern, not gated here
- SendBlue carrier-level opt-out decoding (3G.1 #2), migration verification at boot (3G.1 #1), tool_invocations alert_id traceability (3G.1 #6), and 8 other production-hardening items catalogued in `project_3g1-production-hardening-candidates` memory — must run the 6-question gate before v0.5 friend beta begins.

v0.5 friend beta starts with a fresh 6-question pre-phase gate.
