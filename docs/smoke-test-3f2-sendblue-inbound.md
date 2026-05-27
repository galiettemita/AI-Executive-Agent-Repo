# Phase 3F.2 — Founder SendBlue Inbound Reply Smoke Test (Runbook)

> Founder-only smoke gate. Branch: `phase3f2-sendblue-inbound-smoke`.
> Required deliverable: `docs/SMOKE_REPORT_3F2.md` with `VERDICT: PASS`
> committed to this branch before merge. **Phase 3G demo gate does
> NOT begin until 3F.2 PASS is on `main`.**
>
> Founder directive 2026-05-26: this smoke gate must EXPLICITLY VERIFY
> SendBlue's real webhook authentication behavior. The smoke MUST NOT
> assume the 3F.1 substrate auth implementation is correct. If observed
> behavior differs from the runtime, fix runtime code before any PASS
> report.

This is the first time the **inbound reply** flow fires for real:

```
real founder reply via iMessage → SendBlue webhook → POST /sendblue/inbound
  → audit-first → kill-switch check → webhook-secret header verify
  → JSON parse → from-number allowlist → inbound_replies dedup
  → reply parser (deterministic STOP/START OR OpenAI classifier)
  → state transition + feedback event + memory_signal update
  → (next outbound cycle) STOP enforcement blocks future sends
```

3F.1 shipped the substrate (`/sendblue/inbound` route + reply parser +
`stop_active` memory_signal + STOP enforcement in outbound-sender).
3F.2 proves the chain works against a real SendBlue webhook with real
auth + real founder replies.

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3E2.md` on `main` with `VERDICT: PASS`
- [ ] PR #33 (Phase 3F.1 substrate) merged to `main`
- [ ] Working `apps/fomo/.env.3b3.local` from prior phases
- [ ] **ngrok** installed (free tier is fine; smoke uses both the tunnel and ngrok's request inspector at `http://localhost:4040`)
- [ ] SendBlue account from 3E.2 still active

> **Cost note.** This run uses real OpenAI calls (ranker for the
> upstream alert + classifier for one soft inbound intent) and real
> SendBlue API calls (one outbound iMessage for the alert + zero
> outbound iMessages while STOP is active). Total spend ≈ a couple of
> pennies. Bound the inbox to 1–2 test emails.

---

## 1. Configure the SendBlue webhook (one-time setup)

Open the SendBlue dashboard → **Webhooks** (or **API** → **Webhooks**;
exact navigation depends on the dashboard version).

1. **Add a new webhook** (or edit an existing one):
   - **URL**: `https://<your-ngrok-subdomain>.ngrok.app/sendblue/inbound`
     (the public URL you'll create in §3 below). For now, use a
     placeholder — you'll update this after starting ngrok.
   - **Events**: subscribe to **inbound messages** (the event SendBlue
     fires when someone texts your SendBlue number). Exact event name
     depends on the dashboard; pick the one that triggers when a
     reply is received, NOT one that triggers on outbound delivery
     status.
   - **Secret**: paste a long random string (e.g., `openssl rand -hex 32`).
     **Copy this value** — you'll set it as `SENDBLUE_WEBHOOK_SECRET`
     in §2 below. SendBlue will echo this verbatim in a request
     header on every webhook POST (per
     [docs.sendblue.com/getting-started/webhooks](https://docs.sendblue.com/getting-started/webhooks)).
2. **Save**.

Don't worry about the placeholder URL — you'll come back to update
it once ngrok is running.

---

## 2. Extend `.env.3b3.local` with the 3F.2 vars

```bash
# NEW in 3F.2:
FOMO_SENDBLUE_INBOUND_ENABLED=true
SENDBLUE_WEBHOOK_SECRET=<the long random secret you set in step 1>

# Optional but informational (the server doesn't read it):
SENDBLUE_INBOUND_PUBLIC_URL=https://<your-ngrok-subdomain>.ngrok.app/sendblue/inbound

# Optional — only set AFTER you observe the actual header SendBlue
# uses (see §4 Auth Mechanism Confirmation). Default if unset:
# `sb-signing-secret`.
# SENDBLUE_WEBHOOK_SECRET_HEADER=<observed header name>

# Bounded smoke window (recommended; 30 cycles × 10s = ~5 minutes):
FOMO_GMAIL_POLLING_MAX_CYCLES=30
FOMO_OUTBOUND_MAX_CYCLES=30
```

Source it:

```bash
unset SENDBLUE_WEBHOOK_SECRET SENDBLUE_WEBHOOK_SECRET_HEADER SENDBLUE_INBOUND_PUBLIC_URL FOMO_SENDBLUE_INBOUND_ENABLED
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity (NEVER echo the actual secret):
echo "inbound_enabled=$FOMO_SENDBLUE_INBOUND_ENABLED  webhook_secret_len=${#SENDBLUE_WEBHOOK_SECRET}  header_override=${SENDBLUE_WEBHOOK_SECRET_HEADER:-<default sb-signing-secret>}"
```

Then preflight:

```bash
pnpm --filter @brevio/fomo run preflight:3f2
```

Must end with `✓ Preflight passed.` (a few WARN lines are OK —
specifically the warning about confirming the actual header name
during smoke).

---

## 3. Start ngrok + start the server

In **terminal T1**, start the server:

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3f2.log
```

Look for:

```
fomo.sendblue.inbound.enabled  ... inbound_route_mounted: true, webhook_secret_header: sb-signing-secret
fomo.server.listening          ... sendblue_inbound_route_mounted: true
```

The `webhook_secret_header` field tells you which header the route
will read — that's load-bearing for §4 below.

In **terminal T2**, start ngrok:

```bash
ngrok http 8080
```

Two things you need from ngrok's output:

1. The public HTTPS URL (something like
   `https://abc123.ngrok.app`). Append `/sendblue/inbound` to it and
   paste into your SendBlue webhook URL in the dashboard (§1).
2. The local inspector URL: `http://localhost:4040`. **Open this
   in your browser now** — you'll need it in §4.

---

## 4. AUTH MECHANISM CONFIRMATION (LOAD-BEARING)

**This is the founder-mandated step that gates everything else.**

The 3F.1 substrate assumes SendBlue uses a **plain shared secret in
a request header** (not HMAC over the body). SendBlue's public docs
confirm shared-secret-header but do NOT name the exact header. This
step inspects a real SendBlue POST and confirms the assumption
empirically.

### Trigger one real SendBlue POST

Easiest way: text yourself something (anything — "ping") from a
different iMessage account to your SendBlue number. SendBlue will
deliver it via webhook within a few seconds.

If you don't have a second iMessage account handy, an alternative:
in the SendBlue dashboard, find a "test webhook" or "send sample
event" button (if it exists for your account). Or skip ahead to §5
(triggering an alert) and observe the webhook that fires when you
text back.

### Observe the request — two paths

**Path A (recommended): ngrok inspector**

1. Open `http://localhost:4040` in your browser.
2. Find the POST request to `/sendblue/inbound` in the request list.
3. Click it. Look at the **Request → Headers** section.
4. Identify which header carries the literal secret value you
   configured in §1.

**Path B: server log inspection**

1. In T1, the route already audits `fomo.sendblue.inbound_received`
   with `secret_header_present: true|false`. Check the log:
   ```bash
   grep 'fomo.sendblue.inbound_received' /tmp/fomo-3f2.log | tail -3
   ```
2. If `secret_header_present: false`, the route did NOT find the
   secret in the configured header — proceed to the decision tree
   below.
3. (Optional, more verbose) Temporarily add a one-line
   `console.log(JSON.stringify(req.headers))` to
   `apps/fomo/src/routes/sendblue-inbound.ts` inside
   `tryHandleSendBlueInboundRequest` (just after `readBody` returns)
   to see EVERY inbound header. **Revert this line before commit.**

### Decision tree

Look at the headers you observed. Answer this question:

**Q: Does any header contain the LITERAL VALUE of the secret you
configured in step 1?** (Same string, byte-for-byte.)

- **Scenario A — YES, a header (e.g., `sb-signing-secret`,
  `x-sb-secret`, or whatever name) contains the literal secret.**
  - Note the header name.
  - If it equals `sb-signing-secret` (the runtime default): proceed
    to §5. No env change required.
  - If it differs: set `SENDBLUE_WEBHOOK_SECRET_HEADER=<observed
    name>` in `.env.3b3.local`, re-source, restart the server in T1.
    Confirm the boot log shows the new value in
    `fomo.sendblue.inbound.enabled  webhook_secret_header: <observed name>`.
    Then proceed to §5.

- **Scenario B — NO. A header contains a hex/base64 string, or a
  string prefixed with `sha256=` or `v1=`, that does NOT equal the
  literal secret.** SendBlue is signing the body, not echoing the
  secret. **STOP THE SMOKE.**
  - Capture:
    - The header name + value (redact the value to first 8 chars).
    - The exact body bytes SendBlue sent (from ngrok inspector
      Request → Body).
    - The configured secret length (`echo ${#SENDBLUE_WEBHOOK_SECRET}` — don't paste the value).
  - Paste all three to Claude with a request for a runtime patch.
  - Do NOT proceed to §5 until the runtime is patched + re-tested.
  - Do NOT commit `VERDICT: PASS`.

- **Scenario C — neither A nor B (header is empty / not present
  / completely different shape).** Check:
  - SendBlue dashboard webhook config — did you save the secret?
  - The webhook URL in the dashboard — is it exactly your ngrok URL
    + `/sendblue/inbound`?
  - The event subscription — did you subscribe to inbound messages?
  - The ngrok tunnel — is it actually forwarding to port 8080?
  - The server — does the boot log show
    `sendblue_inbound_route_mounted: true`?
  - Fix and retry.

**Record your observation.** You'll fill in §5 of the smoke report
with: observed header name, observed scheme, whether the header
value equals the configured secret literally, whether a runtime
patch was required.

---

## 5. Smoke scenario 1 — trigger an upstream alert + outbound iMessage

Send yourself a test email from any non-founder address with subject
`Reminder: deposit due tonight` (the same shape that worked for 3C.4
/ 3E.2). Watch T1:

```
fomo.poll.cycle      ... messages_observed: 1, messages_ranked: 1, alerts_created: 1
fomo.rank.completed  ... label: "important"
alert.created        ... alert_id: <UUID-A>
fomo.slack.posted    ... slack_ts: ...
```

Open Slack, click ✅ Approve on the newest card. Within ~10s:

```
fomo.outbound.cycle      ... alerts_sent: 1
fomo.send.attempted      ... destination_slug: <last 4 of your phone>
fomo.send.succeeded      ... provider_status: QUEUED
state.transitioned       ... alert <UUID-A>: approved → sent
```

**Check your phone** — one iMessage arrived from your SendBlue
number. This is what you'll reply to in the next scenarios.

> If the iMessage doesn't arrive, fix that first (re-run 3E.2 mental
> model: OAuth re-walk, SendBlue creds, founder phone). 3F.2 cannot
> proceed without an iMessage to reply to.

---

## 6. (Optional sanity) Confirm `stop_active` is NOT yet set

```bash
psql "$DATABASE_URL" -P pager=off -c "SELECT user_id, kind, detail, updated_at FROM memory_signals WHERE kind='stop_active' AND user_id='founder';"
```

Expected: 0 rows (you haven't sent STOP yet).

---

## 7. Smoke scenario 2 — soft intent (classifier path)

**Reply to the FOMO iMessage with the exact text:** `tomorrow`

Within ~5 seconds, watch T1:

```
fomo.sendblue.inbound_received  ... secret_header_present: true, secret_header_name: <whatever you set>
fomo.sendblue.reply_parsed      ... intent: snooze, intent_source: classifier, snooze_hint: tomorrow, snooze_until: <24h from now>
state.transitioned              ... alert <UUID-A>: sent → replied
state.transitioned              ... alert <UUID-A>: replied → snoozed
feedback.written                ... kind: user_snoozed, alert_id: <UUID-A>
```

**Sanity via psql:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT alert_id, from_state, to_state, reason, at
FROM alert_state_transitions
WHERE alert_id = '<UUID-A>'
ORDER BY at DESC LIMIT 5;
"
```

Should show `sent → replied → snoozed`.

> **No second iMessage on your phone.** 3F.1 does not implement
> snooze resurface — recording only. If you got a second message,
> something is wrong; ping Claude before continuing.

---

## 8. Smoke scenario 3 — STOP (deterministic + enforcement)

**Reply to the FOMO iMessage with the exact text:** `STOP`

Within ~5 seconds, watch T1:

```
fomo.sendblue.inbound_received  ...
fomo.sendblue.stop_recorded     ... stop_active: true, from_slug: <last 4>
feedback.written                ... kind: stop
memory.upserted                 ... kind: stop_active, detail: { active: true, ... }
```

**Sanity:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail FROM memory_signals WHERE kind='stop_active' AND user_id='founder';
"
```

Should show `{"active": true, ...}`.

### Now prove STOP blocks a future outbound send

Trigger a NEW alert (send yourself another important-looking
email). Watch T1:

```
fomo.poll.cycle           ... alerts_created: 1
alert.created             ... alert_id: <UUID-B>
fomo.slack.posted         ...
```

Click ✅ Approve on the new Slack card. Within ~10s:

```
fomo.outbound.cycle       ... alerts_considered: 1, alerts_sent: 0, alerts_stop_enforced: 1
fomo.send.stop_enforced   ... alert_id: <UUID-B>, reason: stop_active memory_signal is true for this user
state.transitioned        ... alert <UUID-B>: approved → failed
```

**Check your phone — NO second iMessage arrived.** SendBlue was
NEVER called for `<UUID-B>` (no `fomo.send.attempted` row for this
alert).

**Sanity:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT action, detail, occurred_at
FROM audit_log
WHERE action = 'fomo.send.stop_enforced'
ORDER BY occurred_at DESC LIMIT 3;
"
```

---

## 9. Smoke scenario 4 — idempotency (SendBlue retry safety)

SendBlue retries on non-2xx. Simulate a retry by re-POSTing your
earlier `tomorrow` payload via curl. Easiest path:

1. Open ngrok inspector at `http://localhost:4040`.
2. Find the original `tomorrow` POST. Click **Replay** (ngrok
   inspector has a Replay button that re-sends the exact same
   request, including all headers + the original body).

Within ~1s in T1:

```
fomo.sendblue.inbound_received  ...
fomo.sendblue.reply_duplicate   ... provider_message_id: <SendBlue's ID>, original_received_at: <earlier timestamp>
```

**No new state transition, no new feedback event, no new memory
update.** The route short-circuited at the `inbound_replies` UNIQUE
constraint and returned 200 OK to SendBlue.

**Sanity:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS dup_audits FROM audit_log WHERE action='fomo.sendblue.reply_duplicate';
SELECT COUNT(*) AS feedback_snoozes FROM feedback_events WHERE kind='user_snoozed';
"
```

`dup_audits` ≥ 1. `feedback_snoozes` should be exactly 1 (NOT
doubled by the retry).

> If ngrok inspector doesn't have a Replay button, alternative:
> craft a manual curl with the same body + headers SendBlue sent.
> See "Manual curl for idempotency" at the bottom of this runbook.

---

## 10. Smoke scenario 5 — invalid-auth rejection (REQUIRED)

Per founder directive 2026-05-26: fail-closed must be PROVEN, not
assumed. Deliberately send a wrong secret to the route:

```bash
# Get the public ngrok URL into a variable (replace with yours):
NGROK_URL=https://<your-subdomain>.ngrok.app

# POST with an obviously-wrong secret in the expected header:
curl -s -o /dev/null -w "HTTP %{http_code}\n" \
  -X POST "$NGROK_URL/sendblue/inbound" \
  -H "${SENDBLUE_WEBHOOK_SECRET_HEADER:-sb-signing-secret}: WRONG-SECRET-VALUE" \
  -H "content-type: application/json" \
  -d '{"from_number":"+15555550100","content":"unauthorized","message_handle":"sb-unauth-test"}'
```

Expected output: `HTTP 401`.

In T1:

```
fomo.sendblue.inbound_received   ... secret_header_present: true
fomo.sendblue.signature_invalid  ... error_code: secret_mismatch, secret_header_name: <configured>
```

**Sanity:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT COUNT(*) AS rejected
FROM audit_log
WHERE action='fomo.sendblue.signature_invalid'
  AND detail->>'error_code' IN ('secret_mismatch', 'missing_header');
"
```

`rejected` ≥ 1.

Then prove the parser was NOT called: this curl's
`provider_message_id` (`sb-unauth-test`) should NOT appear in
`inbound_replies`:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT * FROM inbound_replies WHERE provider_message_id='sb-unauth-test';
"
```

Expected: 0 rows.

---

## 11. (Optional) Smoke scenario 6 — START clears stop

**Reply to the FOMO iMessage with the exact text:** `START`

In T1:

```
fomo.sendblue.start_recorded  ... stop_active: false
memory.upserted               ... kind: stop_active, detail: { active: false, ... }
```

**Sanity:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail FROM memory_signals WHERE kind='stop_active' AND user_id='founder';
"
```

Should now show `{"active": false, ...}`.

(After START, a NEW approved alert would send normally. You don't
have to actually trigger one; the memory_signal flip is sufficient
evidence for 3F.2 substrate scope.)

---

## 12. Wait for the worker caps, then stop

In T1, wait until you see both:

```
fomo.outbound.cycle_cap_reached  cycles_run: 30, cycle_cap: 30
fomo.poll.cycle_cap_reached      cycles_run: 30, cycle_cap: 30
```

Then Ctrl-C the dev server.

---

## 13. Run evidence

```bash
pnpm --filter @brevio/fomo run smoke-evidence:3f2 2>&1 | tee /tmp/fomo-3f2-evidence.log
```

The verdict line at the bottom must read `VERDICT: PASS` for this
report to commit. Paste the full stdout into §7 of the smoke report.

Required-PASS gate criteria:

- `inbound_replies` ≥ 1
- `fomo.sendblue.reply_parsed` with `intent_source: classifier` ≥ 1 (soft intent path)
- `fomo.sendblue.stop_recorded` ≥ 1 (deterministic STOP)
- `fomo.sendblue.reply_duplicate` ≥ 1 (idempotency)
- `fomo.sendblue.signature_invalid` ≥ 1 with `error_code: secret_mismatch | missing_header` (invalid-auth rejection)
- `state_transitions` row `sent → replied` ≥ 1
- `state_transitions` row `replied → snoozed | ignored` ≥ 1
- `feedback_events` from inbound ≥ 1
- `memory_signals.stop_active` exists
- `fomo.send.stop_enforced` ≥ 1 (STOP blocked future send)
- **Leak-canary scan green** — no full phone / no webhook secret / no API keys / no long base64-hex blobs anywhere

Recommended-WARN (gate-passable):
- `fomo.sendblue.start_recorded` ≥ 1 (only if you ran scenario 6)
- `fomo.outbound.cycle_cap_reached` line present in log

---

## 14. Report

1. Copy [`docs/SMOKE_REPORT_TEMPLATE_3F2.md`](SMOKE_REPORT_TEMPLATE_3F2.md)
   to `docs/SMOKE_REPORT_3F2.md`.
2. Fill in every section. **§5 Auth Observation is load-bearing
   and founder-recorded** — fill in the observed header name + the
   observed auth scheme + whether the header value equaled the
   configured secret literally + whether a runtime patch was
   required.
3. If `VERDICT: PASS`: commit + push to this branch, merge the PR.
4. If `VERDICT: FAIL`: log the failure; do not merge.

---

## What "PASS" means in 3F.2

Per the founder-confirmed scope (2026-05-26):

> **Real founder reply reaches Brevio through SendBlue. Auth
> verification succeeds for valid SendBlue requests. Invalid auth
> is rejected. Duplicate webhook is idempotent. STOP is
> deterministic (no LLM). START clears stop. One soft reply intent
> is parsed. State / feedback / memory writes happen. STOP
> enforcement blocks future sends. No raw reply text, Gmail body,
> headers, attachment names, phone numbers, or secrets leak into
> audit/memory. `docs/SMOKE_REPORT_3F2.md` has `VERDICT: PASS`.**
>
> **AND** the runtime auth implementation matches the auth scheme
> observed from real SendBlue webhooks (§4). If they differ, the
> runtime is patched first.

After 3F.2 PASS lands on `main`, Phase 3G (full v0.1 demo) is
unblocked.

---

## Cleanup (recommended)

After the smoke, flip kill switches back off:

```bash
# Edit apps/fomo/.env.3b3.local:
FOMO_SENDBLUE_INBOUND_ENABLED=false
FOMO_SEND_ENABLED=false
# (Optional) clear stop_active back to false via START so future
# manual outbound testing isn't blocked.
```

Re-source + restart. Boot log should show:

```
fomo.sendblue.inbound.disabled  ... /sendblue/inbound route NOT mounted
fomo.send.disabled              ... outbound sender worker dormant
```

---

## Manual curl for idempotency (alternative to ngrok Replay)

If ngrok inspector doesn't have a working Replay button, you can
re-send a SendBlue payload manually. From ngrok inspector, copy:

1. The exact request body of the `tomorrow` POST (JSON).
2. The value of the secret header on that request.
3. The value of `message_handle` from the body (note this for the
   `inbound_replies` UNIQUE-constraint match).

Then:

```bash
NGROK_URL=https://<your-subdomain>.ngrok.app
BODY='<the exact JSON body from ngrok inspector>'
SECRET_HEADER_NAME='<the header name from ngrok inspector>'
SECRET_VALUE='<the literal secret>'

curl -s -o /dev/null -w "HTTP %{http_code}\n" \
  -X POST "$NGROK_URL/sendblue/inbound" \
  -H "$SECRET_HEADER_NAME: $SECRET_VALUE" \
  -H "content-type: application/json" \
  -d "$BODY"
```

Expected: `HTTP 200` and a `fomo.sendblue.reply_duplicate` audit row.
