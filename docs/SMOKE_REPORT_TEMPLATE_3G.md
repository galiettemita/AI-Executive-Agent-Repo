# Phase 3G Smoke Test Report — Full Founder Demo (v0.1 milestone)

> Fill this template after running every step in
> [`smoke-test-3g-full-demo.md`](smoke-test-3g-full-demo.md). Commit
> as `docs/SMOKE_REPORT_3G.md` once `VERDICT: PASS`. **v0.1 is
> declared done when this report lands on `main`. v0.5 friend beta
> cannot begin until then.**

---

**Founder:** _<name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3g-full-demo-smoke`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Demo alert_id (auto-discovered by evidence script):** _<UUID>_
**Founder Gmail account used:** _<email>_
**Founder phone number used (4-char suffix only):** _<last 4 digits>_
**Test email subject:** _<exact subject>_
**ngrok subdomain used:** _<e.g. xyz.ngrok-free.dev>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3E2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3F2.md` on `main` with `VERDICT: PASS`
- [ ] All 5 Neon migrations applied (0000_init through 0004_inbound_replies)
- [ ] SendBlue dashboard webhook URL is the current ngrok URL
- [ ] Gmail OAuth re-auth completed; `oauth_tokens.needs_reauth = false` for `founder`

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

| Var                                | Set? | Notes                                                                |
| ---------------------------------- | ---- | -------------------------------------------------------------------- |
| `DATABASE_URL`                     | ☐    | Neon Postgres                                                        |
| `BREVIO_TOKEN_KEK`                 | ☐    |                                                                      |
| `BREVIO_OAUTH_STATE_KEY`           | ☐    |                                                                      |
| `BREVIO_SESSION_SIGNING_KEY`       | ☐    |                                                                      |
| `GOOGLE_CLIENT_ID/SECRET`          | ☐    |                                                                      |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ☐    |                                                                      |
| `OPENAI_API_KEY`                   | ☐    |                                                                      |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☐    | `true`                                                               |
| `FOMO_RANKER_ENABLED`              | ☐    | `true`                                                               |
| `FOMO_SLACK_REVIEW_ENABLED`        | ☐    | `true`                                                               |
| `SLACK_BOT_TOKEN`                  | ☐    | `xoxb-...`                                                           |
| `SLACK_FOUNDER_CHANNEL_ID`         | ☐    |                                                                      |
| `SLACK_SIGNING_SECRET`             | ☐    |                                                                      |
| `SLACK_FOUNDER_USER_ID`            | ☐    |                                                                      |
| `FOMO_SEND_ENABLED`                | ☐    | `true`                                                               |
| `SENDBLUE_API_KEY_ID`              | ☐    |                                                                      |
| `SENDBLUE_API_SECRET_KEY`          | ☐    |                                                                      |
| `SENDBLUE_FROM_NUMBER`             | ☐    | E.164                                                                |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ☐    | E.164                                                                |
| `FOMO_FOUNDER_USER_ID`             | ☐    | `founder`                                                            |
| `FOMO_SENDBLUE_INBOUND_ENABLED`    | ☐    | `true`                                                               |
| `SENDBLUE_WEBHOOK_SECRET`          | ☐    | Scenario A confirmed by 3F.2                                         |
| `SENDBLUE_WEBHOOK_SECRET_HEADER`   | ☐    | Default `sb-signing-secret` (3F.2-confirmed)                         |
| `SENDBLUE_INBOUND_PUBLIC_URL`      | ☐    | Current ngrok URL                                                    |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ☐    | 60 recommended                                                       |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ☐    | 60 recommended                                                       |
| `BREVIO_DEV_MODE`                  | ☐    | UNSET                                                                |
| `FOMO_AUTO_SEND_ENABLED`           | ☐    | UNSET / false                                                        |
| `FOMO_FRIEND_BETA_ENABLED`         | ☐    | UNSET / false                                                        |

---

## 3. Commands run

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# OAuth re-auth in browser (§1 of runbook)

pnpm --filter @brevio/fomo run preflight:3g
pnpm --filter @brevio/fomo run build

FOMO_GMAIL_POLLING_MAX_CYCLES=60 \
FOMO_OUTBOUND_MAX_CYCLES=60 \
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g.log

# T2: ngrok http 8080 ; update SendBlue dashboard webhook URL if ngrok changed

# (Send the FOMO-worthy test email from your founder Gmail to yourself; §4)
# (Watch chain; approve in Slack; receive SendBlue text; §5)
# (Reply "tomorrow" to the SendBlue iMessage; §6)
# (Wait for cycle_cap_reached; Ctrl-C; §7)

pnpm --filter @brevio/fomo run smoke-evidence:3g 2>&1 | tee /tmp/fomo-3g-evidence.log

# Clean-stop verification (§9 of runbook)
```

---

## 4. Boot-time confirmation

Paste the boot log lines (verbatim, redacting secret values):

```
fomo.gmail.polling.enabled    interval_ms: ..., cycle_cap: 60
fomo.send.enabled             founder_phone_configured: true, auto_send_enabled: false
fomo.outbound.enabled         interval_ms: 10000, cycle_cap: 60
fomo.slack.review.enabled     interactivity_route_mounted: true
fomo.sendblue.inbound.enabled inbound_route_mounted: true, webhook_secret_header: sb-signing-secret
fomo.server.listening         sendblue_inbound_route_mounted: true, send_enabled: true,
                              gmail_polling_started: true, outbound_worker_started: true
```

- [ ] All `fomo.*.enabled` lines present
- [ ] `sendblue_inbound_route_mounted: true`
- [ ] `gmail_polling_started: true`

---

## 5. Demo alert state trail (LOAD-BEARING)

Paste the FULL state trail of the demo alert (auto-discovered by the
evidence script). Confirm it includes EVERY transition below:

```
{alert_id}: detected          → ranked              (ranker labeled important score=...)
{alert_id}: ranked            → queued_for_review   (slack.founder_review posted; slack_ts=...)
{alert_id}: queued_for_review → approved            (slack:approved actor_slug=...)
{alert_id}: approved          → sent                (sendblue ok: provider_status=QUEUED)
{alert_id}: sent              → replied             (sendblue:snooze hint=tomorrow)
{alert_id}: replied           → snoozed             (sendblue:snooze hint=tomorrow until=...)
```

- [ ] Six transitions present, in this exact order
- [ ] alert_id is a real UUID (not `smoke3f2-*` synthetic)
- [ ] No `failed` / `send_status_unknown` states in the trail

---

## 6. PASS criteria checklist (each cited with specific row)

For each criterion paste the SPECIFIC evidence row from your queries:

**A. Gmail connected**
- oauth_tokens row: user=founder needs_reauth=_<f>_ obtained_at=_<ISO>_
- gmail_cursors row: user=founder history_id=_<N>_ last_polled_at=_<ISO>_

**B. Ranker works**
- rank_results row: id=_<N>_ label=important score=_<≥0.7>_ model=_<openai model id>_

**C. Slack review works**
- audit_log row: fomo.slack.approval_captured at=_<ISO>_ detail.alert_id=_<UUID>_
- state transition: queued_for_review → approved at=_<ISO>_

**D. SendBlue send works**
- audit_log row: fomo.send.succeeded at=_<ISO>_ detail.alert_id=_<UUID>_ provider_status=QUEUED
- tool_invocations row count: 1 (sendblue.send_user_message tied to alert_id)
- state transition: approved → sent at=_<ISO>_

**E. Reply parser works**
- audit_log row: fomo.sendblue.reply_parsed at=_<ISO>_ intent=snooze intent_source=classifier snooze_hint=tomorrow
- state transition: sent → replied at=_<ISO>_

**F. Memory / feedback writes**
- feedback_events row: id=_<N>_ kind=user_snoozed alert_id=_<UUID>_
- memory_signals touched during demo window: _<count>_

**G. No duplicate sends**
- tool_invocations(sendblue.send_user_message) for demo alert_id: count = 1

**H. No raw body leakage**
- Leak-canary scan result: _<X rows scanned, 0 hits>_

---

## 7. `pnpm smoke-evidence:3g` output (LOAD-BEARING)

Paste the full stdout. The verdict line at the bottom MUST read
`VERDICT: PASS` for this report to commit.

```
…
```

---

## 8. Founder observations

| Observation | Note |
|---|---|
| Did exactly one alert flow from email to snooze without intervention? | _<yes / yes but I…>_ |
| Was the iMessage text on your phone clear and professional (no "I think you should…" hedging, no [REDACTED] artifacts)? | _<yes / no — paste the actual text>_ |
| Did the ranker label the test email `important` on the first try, or did you need to revise the subject/body? | _<first try / revised — what changed>_ |
| Did anything in the chain fail and need a retry (Slack click double-fire, SendBlue rate limit, ngrok stall)? | _<no / yes — describe>_ |
| Did the leak-canary scan stay green? | _<yes / no — paste hits>_ |
| Anything else surprising? | _<one-line summary>_ |

---

## 9. Clean-stop confirmation

After the smoke, restart with every kill switch off:

```bash
FOMO_GMAIL_POLLING_ENABLED=false \
FOMO_RANKER_ENABLED=false \
FOMO_SLACK_REVIEW_ENABLED=false \
FOMO_SEND_ENABLED=false \
FOMO_SENDBLUE_INBOUND_ENABLED=false \
pnpm --filter @brevio/fomo dev
```

- [ ] `curl -X POST /sendblue/inbound` → HTTP 404
- [ ] `curl -X POST /slack/interactivity` → HTTP 404
- [ ] No `fomo.sendblue.*` / `fomo.send.*` / `fomo.slack.*` / `fomo.poll.cycle` rows landed after the cleanstop restart

---

## 10. Verdict

☐ **PASS** — every required check in §6 is green, demo-alert state trail
in §5 is complete (6 transitions), `smoke-evidence:3g` printed
`VERDICT: PASS`, no leaks, clean stop confirmed. **v0.1 is done.**
Phase 3G demo gate is the v0.1 milestone — passing it unblocks v0.5 (friend beta).

☐ **FAIL** — list failures below; v0.1 blocked until a re-run reaches PASS.

Failures / followups:

- _…_

---

## 11. v0.1 milestone sign-off

I (founder) confirm that as of _<date>_:

- A real email from my real Gmail account was picked up by the FOMO polling
  worker, ranked by the v0.1 ranker, posted to my real Slack channel for
  review, approved by me in Slack, sent to my real phone via SendBlue as
  an iMessage, replied to by me with a soft intent, parsed by the reply
  parser, and snoozed in the FOMO state store — all with **zero raw email
  body** persisted anywhere, **zero duplicate sends**, and **zero
  intervention** beyond the Slack approval click and the SMS reply.
- The substrate is real. v0.1 is done. Friend beta is unblocked.

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_

---

## 12. What v0.1 PASS does NOT promise

- Snooze resurface scheduler — explicitly out of v0.1
- Friend beta — gated behind v0.5
- Auto-send — gated behind its own gate
- Group chats — explicitly out
- Proactive follow-up messages — explicitly out
- Calendar / Drafting / Sending / MCP tools / Autonomous — L2+ surfaces, out
- Multi-tenant scale or rate limits — operational maturity is a separate concern

v0.5 friend beta starts with a fresh 6-question pre-phase gate.
