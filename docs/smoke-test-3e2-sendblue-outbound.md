# Phase 3E.2 — Founder SendBlue Outbound Real-iMessage Smoke Test (Runbook)

> Founder-only smoke gate. Branch: `phase3e2-sendblue-outbound-smoke`.
> Required deliverable: `docs/SMOKE_REPORT_3E2.md` with `VERDICT: PASS`
> committed to this branch before merge. **Phase 3F SendBlue inbound
> does NOT start until 3E.2 PASS is on `main`.**

This is the first time the **outbound send** flow fires for real:

```
real Gmail → polling worker → real ranker → label=important →
  alert created → real Slack post → founder clicks Approve →
  alert state transitions queued_for_review → approved →
  outbound-sender worker picks up approved alert →
  re-reads Gmail → renders deterministic founder text →
  signed POST to real SendBlue → ONE real iMessage to founder phone →
  alert state transitions approved → sent
```

3E.1 shipped the substrate (`SendBlueClient`, `sendBlueSendExecutor`,
`outbound-sender` worker, founder-phone allowlist, three-outcome
state-machine handling). 3E.2 proves the substrate works against a
real SendBlue account and a real phone.

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_3B3.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3D2.md` is on `main` with `VERDICT: PASS`
- [ ] PR #30 (Phase 3E.1 SendBlue substrate) is merged to `main`
- [ ] Working `apps/fomo/.env.3b3.local` from the 3B.3 / 3C.4 / 3D.2 runs
- [ ] Neon Postgres + Gmail OAuth + OpenAI billing + Slack workspace all still configured
- [ ] You have **ngrok** or **cloudflared** installed (for the Slack approval click upstream)
- [ ] **You have a SendBlue account** with an API key id + API secret key from the SendBlue dashboard
- [ ] **You have a real US phone number** ready to receive iMessages (yours)

> **Cost note.** This run uses real OpenAI calls (for the rank that
> produces the alert), real Slack API calls, and **one real SendBlue
> message-send call** (the load-bearing event). Bound the inbox
> activity to 1-2 test emails. SendBlue's per-message cost is a few
> cents. Total spend ≈ a couple of dollars.

---

## 1. Set up SendBlue account (one-time)

If you already have an account, skip to step 2.

1. Sign up at https://sendblue.co. You'll need a phone number for the
   sending side — SendBlue assigns one as part of onboarding.
2. **Dashboard → API Keys.** Create an API key. Copy the **API Key ID**
   (this is `SENDBLUE_API_KEY_ID`) and the **API Secret Key** (this is
   `SENDBLUE_API_SECRET_KEY`). The secret will not be shown again.
3. (Recommended) SendBlue offers a **sandbox / test mode** if available
   in your account — check their docs. If a sandbox path exists,
   walk through steps 2-9 once against the sandbox first to validate
   auth + state-transitions WITHOUT delivering a real iMessage, then
   flip to production for the load-bearing real-delivery run.
4. Verify your destination phone number — SendBlue may require the
   sender to verify the recipient is a real number you control.

> **§4.8 reminder.** Per the API-first / browser-fallback /
> approval-required rule, this whole flow is API-first (SendBlue is the
> official API), human-approved (Slack founder review), and bounded
> (founder-phone allowlist + cycle cap). No browser fallback exists or
> is needed.

---

## 2. Extend `.env.3b3.local` with the 3E.2 vars

```bash
# NEW in 3E.2:
FOMO_SEND_ENABLED=true
SENDBLUE_API_KEY_ID=...                  # from SendBlue dashboard
SENDBLUE_API_SECRET_KEY=...              # from SendBlue dashboard
SENDBLUE_FROM_NUMBER=+1XXXYYYZZZZ        # SendBlue-assigned SENDER phone (NOT your phone)
                                          # — surfaced by smoke run: SendBlue rejects
                                          # every send with HTTP 400 without this
FOMO_FOUNDER_PHONE_NUMBER=+1XXXYYYZZZZ   # YOUR phone, E.164 format (destination)
FOMO_FOUNDER_USER_ID=<your user_id>      # same user_id you completed Google OAuth as
                                          # (in 3B.3 / 3C.4 / 3D.2 runs this was the
                                          # string "founder")

# Bounded smoke window — recommended for the smoke run so the
# outbound-sender auto-stops after a controlled number of cycles even
# if something goes wrong. The worker emits
# `fomo.outbound.cycle_cap_reached` when this fires. The smoke run
# benefits from a higher cap (~30) at a 10s polling interval so the
# founder has ~5 minutes to send the trigger email + click Approve
# in Slack + watch the iMessage arrive without racing the clock.
FOMO_OUTBOUND_MAX_CYCLES=30

# Hard non-goal — must stay UNSET / false:
# FOMO_AUTO_SEND_ENABLED=  (auto-send is not in 3E.2 scope)
# FOMO_FRIEND_BETA_ENABLED= (founder-only smoke)

# Already set from prior phases:
# DATABASE_URL=...
# BREVIO_TOKEN_KEK=...
# BREVIO_OAUTH_STATE_KEY=...
# BREVIO_SESSION_SIGNING_KEY=...
# GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / BREVIO_OAUTH_REDIRECT_URI_GOOGLE
# OPENAI_API_KEY=...
# FOMO_GMAIL_POLLING_ENABLED=true
# FOMO_RANKER_ENABLED=true
# FOMO_SLACK_REVIEW_ENABLED=true
# SLACK_BOT_TOKEN=xoxb-...
# SLACK_FOUNDER_CHANNEL_ID=C...
# SLACK_SIGNING_SECRET=...
# SLACK_FOUNDER_USER_ID=U...
```

Source it:

```bash
unset SENDBLUE_API_KEY_ID SENDBLUE_API_SECRET_KEY FOMO_FOUNDER_PHONE_NUMBER FOMO_FOUNDER_USER_ID FOMO_OUTBOUND_MAX_CYCLES
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity check (NEVER echo the secrets):
echo "send_enabled=$FOMO_SEND_ENABLED  key_id_len=${#SENDBLUE_API_KEY_ID}  secret_len=${#SENDBLUE_API_SECRET_KEY}"
echo "founder_phone_suffix=...${FOMO_FOUNDER_PHONE_NUMBER: -4}  founder_user=$FOMO_FOUNDER_USER_ID  cap=$FOMO_OUTBOUND_MAX_CYCLES"
```

Then preflight:

```bash
pnpm --filter @brevio/fomo run preflight:3e2
```

Must end with `✓ Preflight passed.` Common issues:
- `FOMO_FOUNDER_PHONE_NUMBER` not in E.164 → preflight ERROR
- `FOMO_OUTBOUND_MAX_CYCLES` unset → preflight WARN (proceeds, but cap is highly recommended)

---

## 3. (Optional but recommended) Sandbox dry run

If SendBlue offers a sandbox API endpoint or test mode in your
dashboard:

1. Configure your SendBlue account in sandbox / test mode.
2. Run through steps 4-7 below once.
3. Verify:
   - `fomo.send.attempted` audit row appears
   - State transitions `approved → sent` (assuming sandbox returns a
     "queued"-shaped status)
   - No real iMessage on your phone
4. Flip your SendBlue account out of sandbox.
5. Run through steps 4-7 again for the LOAD-BEARING real-delivery run.

If SendBlue has no sandbox path, skip this step. The kill-switch +
founder-phone allowlist + cycle cap make a direct production run
safe.

---

## 4. Start the tunnel + start the server

In **terminal T1**, start the server:

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3e2.log
```

Look for these boot lines:

```
fomo.send.enabled               ... founder_user_id: <your-uuid>, founder_phone_configured: true, auto_send_enabled: false
fomo.outbound.enabled           ... interval_ms: 60000, cycle_cap: 2, auto_send_enabled: false
fomo.slack.review.enabled       ... interactivity_route_mounted: true
fomo.server.listening           ... send_enabled: true, outbound_worker_started: true
```

In **terminal T2**, start the tunnel (for the Slack approval click
upstream of the send):

```bash
ngrok http 8080
# OR
cloudflared tunnel --url http://localhost:8080
```

Paste the public URL + `/slack/interactivity` into Slack app →
Interactivity & Shortcuts → Request URL → Save. (Same as 3D.2.)

---

## 5. Trigger an alert

Send yourself an email the ranker will label `important` (subject
like `Reminder: deposit due tonight` — known-important shape from
3C.4 / 3D.2 founder runs). From a non-founder email account, send 1
message to the founder Gmail.

Watch `/tmp/fomo-3e2.log` for:

```
fomo.poll.cycle               ... messages_ranked: 1
fomo.rank.completed           ... label: "important"
alert.created                 ... alert_id: <NEW-UUID>
fomo.slack.posted             ... slack_ts: ...
```

Open Slack and confirm the candidate-review card appeared in your
founder channel with **Approve** and **Reject** buttons.

---

## 6. Click Approve (LOAD-BEARING)

Click **✅ Approve** on the card. Within ~1 second, look in
`/tmp/fomo-3e2.log`:

```
fomo.slack.interaction_received
fomo.slack.approval_captured   alert_id=<NEW-UUID>, decision_code=queued_for_review→approved
```

Now wait for the outbound-sender worker's next tick (up to
`FOMO_GMAIL_POLLING_INTERVAL_MS`, default 60s). You should see:

```
fomo.outbound.cycle           ... cycle_number: 1, alerts_considered: 1, alerts_sent: 1
fomo.send.attempted           ... alert_id=<NEW-UUID>, destination_slug=<last 4 of your phone>, template_version=founder-text-v0.1.0
policy.decided                ... tool_id: sendblue.send_user_message, decision_code: allowed
tool.invoked                  ... tool_id: sendblue.send_user_message, status: success
fomo.send.succeeded           ... alert_id=<NEW-UUID>, provider_status=QUEUED|SENT|DELIVERED
state.transitioned            ... alert_id=<NEW-UUID>, from_state=approved, to_state=sent
```

**Check your phone.** You should have received ONE iMessage that
looks like:

```
FOMO · IMPORTANT (0.92)
<sender display name> <s***@masked-domain>
Reminder: deposit due tonight
<≤120-char body snippet, egress-redacted>
```

If the iMessage arrived → 3E.2's load-bearing event happened.

**Quick sanity check** via psql:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT alert_id, from_state, to_state, reason, at
FROM alert_state_transitions
WHERE to_state IN ('sent', 'failed', 'send_status_unknown')
ORDER BY at DESC LIMIT 5;

SELECT tool_id, status, policy_decision, latency_ms, occurred_at
FROM tool_invocations
WHERE tool_id = 'sendblue.send_user_message'
ORDER BY occurred_at DESC LIMIT 5;
"
```

---

## 7. Idempotency exercise (REQUIRED for PASS)

The state machine itself is the idempotency guard — an alert in
`sent` state cannot transition again, so the next cycle's
`findAlertIdsInState('approved', ...)` query will not return it.

Wait one more cycle (up to 60s). In `/tmp/fomo-3e2.log` you should
see:

```
fomo.outbound.cycle           ... cycle_number: 2, alerts_considered: 0, alerts_sent: 0
fomo.outbound.cycle_cap_reached  cycles_run: 2, cycle_cap: 2
```

- `alerts_considered: 0` proves the worker did NOT re-find the
  already-sent alert.
- `cycle_cap_reached` proves the bounded smoke window terminated
  cleanly.
- **No second iMessage arrived on your phone.**

If you see a second iMessage arrive → STOP. This indicates a serious
bug in the state-machine idempotency layer and must be diagnosed
before PASS.

---

## 8. (Optional) Failure-path exploration

The runbook does not require these. They are listed for the
curious / for re-runs if the first happy path fails:

- **Wrong founder phone:** temporarily set `FOMO_FOUNDER_USER_ID`
  to a different value than the user_id that approved the alert.
  Restart server. The outbound worker should audit
  `fomo.send.unauthorized_destination` and transition the alert to
  `failed` WITHOUT calling SendBlue.
- **Kill switch off:** set `FOMO_SEND_ENABLED=false`. Restart
  server. The outbound-sender worker is NOT started; an approved
  alert sits in `approved` indefinitely. (Don't restore mid-test —
  flipping back to true should resume cleanly because the alert
  state machine is intact.)

These do NOT need to land in the smoke report unless you exercise
them.

---

## 9. Run evidence

Stop the server (Ctrl-C T1). Run the evidence script:

```bash
pnpm --filter @brevio/fomo run smoke-evidence:3e2
```

Required-PASS gate criteria (per the founder directive):

- `alert_state_transitions`: ≥1 row `queued_for_review → approved` (3D.2 carry-forward)
- `alert_state_transitions`: ≥1 row `approved → sent` (**3E.2 LOAD-BEARING**)
- `tool_invocations`: ≥1 `sendblue.send_user_message` with `status=success`
- `audit_log fomo.send.attempted` ≥ 1
- `audit_log fomo.send.succeeded` ≥ 1
- `audit_log fomo.send.unauthorized_destination` == 0 (allowlist held)
- **Leak-canary scan green:** no rendered text, no full phone, no
  API-key shapes, no raw payload anywhere in audit / tool_invocations
  / feedback_events / alert_state_transitions

Recommended-WARN (gate-passable):
- Single happy path: no `fomo.send.failed` / `fomo.send.status_unknown`
  for the same alert
- `fomo.outbound.cycle_cap_reached` line present in `/tmp/fomo-3e2.log`
  (the evidence script reminds you to grep — paste the matched line
  into §6 of the report)

Capture the full stdout — it's the load-bearing §6 of the report.

---

## 10. Report

1. Copy [`docs/SMOKE_REPORT_TEMPLATE_3E2.md`](SMOKE_REPORT_TEMPLATE_3E2.md)
   to `docs/SMOKE_REPORT_3E2.md`.
2. Fill in every section.
3. If `VERDICT: PASS`: commit + push to this branch, merge the PR.
4. If `VERDICT: FAIL`: log the failure; do not merge. Common causes:
   - SendBlue API key wrong / expired → `fomo.send.failed` with HTTP 401/403
   - Founder phone not in E.164 → preflight ERROR
   - `FOMO_FOUNDER_USER_ID` doesn't match the approving user_id →
     `fomo.send.unauthorized_destination`
   - SendBlue returned non-success status → `fomo.send.failed` (real failure) or `fomo.send.status_unknown` (ambiguous; alert reaches `send_status_unknown`, NOT auto-retried)
   - Network / timeout → `fomo.send.status_unknown`
   - Cycle cap too aggressive (set to 0) → outbound worker never ticks

---

## What "PASS" means in 3E.2

Per the founder-confirmed scope (2026-05-25):

> **Real SendBlue account works. Real founder-only phone receives
> exactly one message. Kill switch blocks sends. Non-founder phone
> is rejected (allowlist held). Duplicate send is prevented
> (state-machine idempotency). Audit / state / tool_invocation
> evidence exists. No sensitive content leaks. `docs/SMOKE_REPORT_3E2.md`
> has `VERDICT: PASS`.**

After 3E.2 PASS lands on main, Phase 3F (SendBlue inbound reply
parser) is unblocked.

---

## Cleanup (recommended)

After the smoke test, flip the send kill switch back off so the dev
server doesn't keep sending real iMessages on every approved alert:

```bash
# Edit apps/fomo/.env.3b3.local and set:
FOMO_SEND_ENABLED=false
# OR remove the line entirely
unset FOMO_OUTBOUND_MAX_CYCLES  # cap only matters when enabled=true
```

Re-source and restart the server. The boot log should show:

```
fomo.send.disabled              ... FOMO_SEND_ENABLED is not "true"; outbound sender worker dormant
fomo.outbound.disabled          ... outbound-sender worker dormant
fomo.server.listening           ... send_enabled: false, outbound_worker_started: false
```
