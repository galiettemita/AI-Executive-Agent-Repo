# Phase 3F.2 Smoke Test Report — SendBlue Inbound Reply (founder-only)

> Fill this template after running every step in
> [`smoke-test-3f2-sendblue-inbound.md`](smoke-test-3f2-sendblue-inbound.md).
> Commit as `docs/SMOKE_REPORT_3F2.md` once `VERDICT: PASS`. **Phase
> 3G demo gate does NOT start until this report lands on `main`.**
>
> Founder directive 2026-05-26 (load-bearing): §5 below is
> founder-recorded and MUST be filled in honestly. `VERDICT: PASS`
> is NOT allowed unless real inbound webhook auth works end-to-end
> AND the runtime auth implementation matches what SendBlue actually
> sends.

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3f2-sendblue-inbound-smoke`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Founder Gmail account used:** _<email>_
**Founder phone number used (4-char suffix only):** _<last 4 digits>_
**SendBlue account email:** _<email>_
**ngrok subdomain used:** _<e.g. abc123.ngrok.app>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3E2.md` on `main` with `VERDICT: PASS`
- [ ] PR #33 (Phase 3F.1) merged
- [ ] SendBlue webhook configured in dashboard pointing at ngrok
- [ ] ngrok tunnel running + inspector accessible at `http://localhost:4040`

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

Confirm each was set during the run. Do NOT paste secret values.

| Var                                | Set? | Notes                                                                |
| ---------------------------------- | ---- | -------------------------------------------------------------------- |
| `DATABASE_URL`                     | ☐    | Neon Postgres                                                        |
| `BREVIO_TOKEN_KEK`                 | ☐    |                                                                      |
| `BREVIO_OAUTH_STATE_KEY`           | ☐    |                                                                      |
| `BREVIO_SESSION_SIGNING_KEY`       | ☐    |                                                                      |
| `GOOGLE_CLIENT_ID/SECRET`          | ☐    |                                                                      |
| `OPENAI_API_KEY`                   | ☐    |                                                                      |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☐    | `true`                                                               |
| `FOMO_RANKER_ENABLED`              | ☐    | `true`                                                               |
| `FOMO_SLACK_REVIEW_ENABLED`        | ☐    | `true`                                                               |
| `SLACK_BOT_TOKEN`                  | ☐    | `xoxb-...`                                                           |
| `SLACK_FOUNDER_CHANNEL_ID`         | ☐    |                                                                      |
| `SLACK_SIGNING_SECRET`             | ☐    |                                                                      |
| `SLACK_FOUNDER_USER_ID`            | ☐    |                                                                      |
| `FOMO_SEND_ENABLED`                | ☐    | `true` (needed for STOP-blocks-outbound test)                        |
| `SENDBLUE_API_KEY_ID`              | ☐    |                                                                      |
| `SENDBLUE_API_SECRET_KEY`          | ☐    |                                                                      |
| `SENDBLUE_FROM_NUMBER`             | ☐    | E.164                                                                |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ☐    | E.164                                                                |
| `FOMO_FOUNDER_USER_ID`             | ☐    |                                                                      |
| `FOMO_SENDBLUE_INBOUND_ENABLED`    | ☐    | **`true` (3F.2 invariant)**                                          |
| `SENDBLUE_WEBHOOK_SECRET`          | ☐    | The secret you set in SendBlue dashboard webhook config              |
| `SENDBLUE_WEBHOOK_SECRET_HEADER`   | ☐    | Default `sb-signing-secret`. CONFIRMED via §5 below: _<value used>_  |
| `SENDBLUE_INBOUND_PUBLIC_URL`      | ☐    | (informational; the ngrok URL you configured)                        |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ☐    | Recommended: 30                                                      |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ☐    | Recommended: 30                                                      |
| `BREVIO_DEV_MODE`                  | ☐    | UNSET                                                                |
| `FOMO_AUTO_SEND_ENABLED`           | ☐    | UNSET / false                                                        |
| `FOMO_FRIEND_BETA_ENABLED`         | ☐    | UNSET / false                                                        |

---

## 3. Commands run

Paste the actual commands. Order matters.

```bash
pnpm --filter @brevio/fomo run preflight:3f2

pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3f2.log

ngrok http 8080
# (SendBlue webhook URL updated in dashboard to ngrok URL + /sendblue/inbound)

# (Trigger one SendBlue POST to inspect auth — §4 of the runbook)

# (Smoke scenarios 1-5 per the runbook §5–§10, optional §11)

# (Wait for cap_reached lines; Ctrl-C the server)

pnpm --filter @brevio/fomo run smoke-evidence:3f2 2>&1 | tee /tmp/fomo-3f2-evidence.log
```

---

## 4. Boot-time confirmation

Paste the boot log lines (verbatim, redacting any secret value):

```
fomo.sendblue.inbound.enabled  ... inbound_route_mounted: true, webhook_secret_header: <value>
fomo.send.enabled              ... founder_phone_configured: true, auto_send_enabled: false
fomo.outbound.enabled          ... interval_ms: 10000, cycle_cap: 30, auto_send_enabled: false
fomo.slack.review.enabled      ... interactivity_route_mounted: true
fomo.server.listening          ... sendblue_inbound_route_mounted: true, send_enabled: true,
                                   outbound_worker_started: true
```

- [ ] `sendblue_inbound_route_mounted: true`
- [ ] `webhook_secret_header` matches the header SendBlue actually uses (see §5)

---

## 5. AUTH OBSERVATION (LOAD-BEARING; founder-recorded)

**Founder directive 2026-05-26:** the smoke gate must NOT assume the
3F.1 implementation matches SendBlue's real auth. Fill in this
section honestly. `VERDICT: PASS` is NOT allowed unless real auth
works end-to-end AND the runtime matches what was observed.

**Observation method used:** _<ngrok inspector at localhost:4040 /
server-log inspection / both / temporarily-added console.log of req.headers>_

**Observed webhook secret header name:**
_<exact lowercase header name, e.g. `sb-signing-secret` /
`x-sb-signature` / `<other>`>_

**Observed auth scheme:** _<one of:>_
- ☐ Scenario A — Plain shared secret in a named header (the header
  value equals the configured secret BYTE-FOR-BYTE)
- ☐ Scenario B — HMAC / signature over body (header value is a hex
  or base64 string, NOT the configured secret, and the configured
  secret was used as a signing key over the body)
- ☐ Scenario C — Something else (describe in detail below)

**Did the observed header value equal the literal `SENDBLUE_WEBHOOK_SECRET`?**
_<yes / no>_

**Was `SENDBLUE_WEBHOOK_SECRET_HEADER` overridden from the default `sb-signing-secret`?**
- If yes, what value? _<observed name>_
- If no, the default matched what SendBlue sent.

**Was a runtime patch required?**
- ☐ No — Scenario A held, the 3F.1 substrate is correct as shipped.
- ☐ Yes — a runtime patch was required. Commit SHA of the patch:
  _<sha>_. Brief description of what was patched:
  _<e.g. "SendBlue actually uses HMAC over body, not plain secret
  header; rewrote verifySendBlueWebhookSecret to verifySendBlueSignature
  with the observed scheme">_

**If Scenario B or C, describe the observed auth shape in detail
(header name, value format, whether the secret is a key over a
basestring, etc.):**

_<your description>_

---

## 6. Cycle + inbound log evidence (server stdout)

Paste the relevant log lines, in order. Redact any secret values.

```
# Upstream alert chain (§5 of runbook):
{ ... fomo.poll.cycle ... messages_ranked: 1 ... }
{ ... fomo.rank.completed ... label: "important" ... }
{ ... alert.created ... alert_id: <UUID-A> ... }
{ ... fomo.slack.posted ... }
{ ... fomo.slack.approval_captured ... }
{ ... fomo.send.succeeded ... alert_id: <UUID-A>, provider_status: QUEUED ... }
{ ... state.transitioned ... alert <UUID-A>: approved → sent ... }

# Scenario 2 — soft intent (§7 of runbook):
{ ... fomo.sendblue.inbound_received ... secret_header_present: true, secret_header_name: <name> ... }
{ ... fomo.sendblue.reply_parsed ... intent: snooze, intent_source: classifier, snooze_hint: tomorrow ... }
{ ... state.transitioned ... alert <UUID-A>: sent → replied ... }
{ ... state.transitioned ... alert <UUID-A>: replied → snoozed ... }
{ ... feedback.written ... kind: user_snoozed ... }

# Scenario 3 — STOP + enforcement (§8 of runbook):
{ ... fomo.sendblue.stop_recorded ... stop_active: true ... }
{ ... memory.upserted ... kind: stop_active ... }
# (new alert <UUID-B> trigger + approve)
{ ... fomo.outbound.cycle ... alerts_stop_enforced: 1 ... }
{ ... fomo.send.stop_enforced ... alert_id: <UUID-B> ... }
{ ... state.transitioned ... alert <UUID-B>: approved → failed ... }

# Scenario 4 — idempotency (§9 of runbook):
{ ... fomo.sendblue.inbound_received ... }
{ ... fomo.sendblue.reply_duplicate ... provider_message_id: <ID>, original_received_at: <earlier> ... }

# Scenario 5 — invalid auth rejection (§10 of runbook):
{ ... fomo.sendblue.inbound_received ... }
{ ... fomo.sendblue.signature_invalid ... error_code: secret_mismatch ... }

# Scenario 6 — START (§11 of runbook, optional):
{ ... fomo.sendblue.start_recorded ... stop_active: false ... }

# Caps fired:
{ ... fomo.outbound.cycle_cap_reached  cycles_run: 30, cycle_cap: 30 ... }
{ ... fomo.poll.cycle_cap_reached      cycles_run: 30, cycle_cap: 30 ... }
```

---

## 7. `pnpm smoke-evidence:3f2` output (LOAD-BEARING)

Paste the full stdout verbatim. The verdict line at the bottom must
read `VERDICT: PASS` for this report to commit.

```
…
```

Required-PASS checks:

- inbound_replies ≥ 1
- fomo.sendblue.reply_parsed with intent_source=classifier ≥ 1
- fomo.sendblue.stop_recorded ≥ 1
- fomo.sendblue.reply_duplicate ≥ 1
- fomo.sendblue.signature_invalid ≥ 1 with error_code: secret_mismatch | missing_header
- alert_state_transitions sent → replied ≥ 1
- alert_state_transitions replied → snoozed | ignored ≥ 1
- feedback_events from inbound ≥ 1
- memory_signals stop_active exists
- fomo.send.stop_enforced ≥ 1
- **Leak-canary scan clean**

Recommended-WARN (gate-passable):
- fomo.sendblue.start_recorded ≥ 1 (only if you ran scenario 6)

---

## 8. Founder observations

| Observation | Note |
|---|---|
| Did exactly the soft-intent state transition + the STOP enforcement fire as designed? | _<yes / surprising>_ |
| Did SendBlue retry a webhook on its own (you see a `reply_duplicate` you did NOT trigger via curl)? | _<no / yes — paste detail>_ |
| Did any iMessage arrive after STOP was active? | _<no / yes — STOP did not enforce, investigate>_ |
| Did the leak-canary scan stay green across all 5 scenarios? | _<yes / no — paste hits>_ |
| Anything else surprising? | _<one-line summary>_ |

---

## 9. Clean-stop confirmation

After the smoke, restart with the inbound + send switches off:

```bash
FOMO_SENDBLUE_INBOUND_ENABLED=false FOMO_SEND_ENABLED=false pnpm --filter @brevio/fomo run dev
```

You should see:

```
fomo.sendblue.inbound.disabled  ... /sendblue/inbound route NOT mounted
fomo.send.disabled              ... outbound sender worker dormant
fomo.server.listening           ... sendblue_inbound_route_mounted: false, send_enabled: false
```

- [ ] `/sendblue/inbound` returns 404 (`curl -X POST http://localhost:8080/sendblue/inbound` → 404)
- [ ] No `fomo.sendblue.*` audit rows written after the restart

---

## 10. Verdict

☐ **PASS** — every required check in §7 is green, §5 Auth Observation
is filled in honestly, runtime matches observed auth scheme (or
runtime was patched + the patch SHA recorded in §5), no leaks, clean
stop confirmed. Phase 3G demo gate may begin.

☐ **FAIL** — list failures below; Phase 3G blocked until a re-run
reaches PASS.

Failures / followups:

- _…_

---

## 11. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
