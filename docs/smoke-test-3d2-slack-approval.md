# Phase 3D.2 — Founder Slack Approval Capture Smoke Test (Runbook)

> Founder-only smoke gate. Branch: `phase3d2-slack-approval-capture`.
> Required deliverable: `docs/SMOKE_REPORT_3D2.md` with `VERDICT: PASS`
> committed to this branch before merge. **Phase 3E SendBlue does NOT
> start until 3D.2 PASS is on `main`.**

This is the first time the **inbound** Slack flow fires for real:

```
real Slack workspace → founder clicks Approve/Reject button → signed
HTTP POST → /slack/interactivity → state transition queued_for_review
→ approved | rejected → feedback event → chat.update of the card
```

3D.1 shipped outbound posting. 3D.2 closes the loop and proves the
founder trust checkpoint actually works end-to-end against a real
Slack workspace.

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_3B3.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` is on `main` with `VERDICT: PASS`
- [ ] PR #28 (Phase 3D.1) is merged to `main`
- [ ] Working `apps/fomo/.env.3b3.local` from the 3B.3 / 3C.4 / 3D.1 runs
- [ ] Neon Postgres + Gmail OAuth + OpenAI billing all still configured
- [ ] You have **ngrok** or **cloudflared** installed (or any other tunneling tool)

> **Cost note.** This run uses real OpenAI calls (for the rank that
> produces the alert) and real Slack API calls. Bound the inbox
> activity to 1–2 test emails. Total spend ≈ a couple of pennies.

---

## 1. Create the Slack app (one-time setup)

If you already have a Slack app from 3D.1, you'll need to add
Interactivity to it. Otherwise:

1. Open https://api.slack.com/apps and click **Create New App** → **From scratch**.
2. Name: `FOMO Founder Review` (or whatever). Workspace: yours.
3. **OAuth & Permissions** → Scopes → Bot Token Scopes → add:
   - `chat:write`
4. **Install to Workspace**. Copy the **Bot User OAuth Token**
   (starts `xoxb-`) — this is `SLACK_BOT_TOKEN`.
5. **Basic Information** → Signing Secret → copy → this is
   `SLACK_SIGNING_SECRET`.
6. Create a channel for the founder review (e.g. `#fomo-review`).
   Invite the bot: in Slack, `/invite @FOMO Founder Review`. Copy the
   channel id from the channel URL (`C0123ABCDEF`) →
   `SLACK_FOUNDER_CHANNEL_ID`.
7. Get your own Slack user id (Profile → ⋯ → Copy member ID) → this is
   `SLACK_FOUNDER_USER_ID`. Recommended but optional.

**Don't enable Interactivity yet** — you need the tunnel URL first
(step 3 below).

---

## 2. Extend `.env.3b3.local` with the 3D.2 vars

```bash
# NEW in 3D.2:
SLACK_SIGNING_SECRET=...        # from Slack app Basic Information
SLACK_FOUNDER_USER_ID=U01XYZ... # optional but recommended

# Optional (informational only — the server doesn't read it):
SLACK_INTERACTIVITY_PUBLIC_URL=https://your-ngrok-subdomain.ngrok.app/slack/interactivity

# Already set from 3D.1:
# SLACK_BOT_TOKEN=xoxb-...
# SLACK_FOUNDER_CHANNEL_ID=C0123ABCDEF
# FOMO_SLACK_REVIEW_ENABLED=true
```

Source it:

```bash
unset SLACK_SIGNING_SECRET SLACK_FOUNDER_USER_ID
set -a; source apps/fomo/.env.3b3.local; set +a

echo "signing_secret_len=${#SLACK_SIGNING_SECRET}  bot=${SLACK_BOT_TOKEN:0:6}...  channel=$SLACK_FOUNDER_CHANNEL_ID  user=${SLACK_FOUNDER_USER_ID:-<unset>}"
```

Then preflight:

```bash
pnpm --filter @brevio/fomo run preflight:3d2
```

Must end with `✓ Preflight passed.` (a few WARN lines are OK — most
common is `SLACK_FOUNDER_USER_ID not set`).

---

## 3. Start the tunnel + start the server

In **terminal T1**, start the server:

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3d2.log
```

Look for:

```
fomo.slack.review.enabled  ... interactivity_route_mounted: true
fomo.server.listening      ... slack_interactivity_route_mounted: true
```

In **terminal T2**, start the tunnel:

```bash
# ngrok:
ngrok http 8080

# OR cloudflared:
cloudflared tunnel --url http://localhost:8080
```

Copy the **HTTPS** public URL it prints (e.g. `https://abc123.ngrok.app`).

---

## 4. Configure Slack app Interactivity

Back in https://api.slack.com/apps → your app → **Interactivity & Shortcuts**:

1. Toggle **Interactivity** to **On**.
2. **Request URL:** paste `https://<your-ngrok-subdomain>.ngrok.app/slack/interactivity`
   (the public tunnel URL + the route path).
3. Click **Save Changes** at the bottom.

> Slack will hit your endpoint with a test payload immediately when
> you save. You should see `fomo.slack.interaction_received` + likely
> `fomo.slack.payload_invalid` (Slack's test pings aren't full
> block_actions) in `/tmp/fomo-3d2.log`. That's expected — it proves
> the tunnel is reaching your laptop.

---

## 5. Trigger an alert

Send yourself an email that the ranker will label `important` (subject
like `"Reminder: deposit due tonight"` is a known-important shape
from the 3C.4 founder run). From a non-founder email account, send 1
message to the founder Gmail.

Wait for the polling worker to fire:

```
fomo.poll.cycle ... messages_ranked: 1
fomo.rank.completed ... label: "important"
alert.created     alert_id: <NEW-UUID>
fomo.slack.posted slack_ts: ...
```

Open Slack and confirm the candidate-review card appeared in your
founder channel with **Approve** and **Reject** buttons.

---

## 6. Click Approve (or Reject)

Click **✅ Approve** on the card. Within ~1 second, look in `/tmp/fomo-3d2.log`:

```
fomo.slack.interaction_received
fomo.slack.approval_captured  alert_id=<NEW-UUID>, decision_code=queued_for_review→approved
```

The Slack card should update in place to show:

> **FOMO — Alert ✅ approved**
> ✅ Approved by <@U01...> at <ISO timestamp>

**Quick sanity check** via psql:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT alert_id, from_state, to_state, reason, at
FROM alert_state_transitions
WHERE to_state IN ('approved', 'rejected')
ORDER BY at DESC LIMIT 5;

SELECT id, user_id, kind, alert_id, occurred_at
FROM feedback_events
WHERE kind IN ('founder_approved', 'founder_rejected')
ORDER BY occurred_at DESC LIMIT 5;
"
```

You should see one transition row + one feedback event row.

---

## 7. Idempotency exercise (REQUIRED for PASS)

The founder directive: "first terminal decision wins. Duplicate
approve/reject must be idempotent." Prove it:

**Click the SAME button (Approve) again** on the already-resolved card.

Watch for:

```
fomo.slack.interaction_received
fomo.slack.approval_duplicate  alert_id=<NEW-UUID>, current_state=approved
```

No new state transition, no new feedback event, no chat.update. The
button click was acknowledged but the alert stays approved.

> If the card has already been updated to its resolution shape (no
> buttons), Slack may show the buttons as disabled. You can still
> POST to `/slack/interactivity` directly with curl using a fresh
> signed payload to prove idempotency, OR trigger a SECOND alert and
> approve+approve that one twice.

---

## 8. Optional: trigger a Reject

If you sent a second test email that the ranker also labeled
important, click **❌ Reject** on its card and verify:

```
fomo.slack.approval_captured  decision_code=queued_for_review→rejected
```

(Both Approve and Reject paths are tested if both run.)

---

## 9. Run evidence

Stop the server (Ctrl-C T1). Run the evidence script:

```bash
pnpm --filter @brevio/fomo run smoke-evidence:3d2
```

Required-PASS gate criteria (per the founder directive):

- `alerts` ≥ 1 row (3D.1 carry-forward)
- transitions to `queued_for_review` ≥ 1 (3D.1 carry-forward)
- **transitions to `approved` OR `rejected` ≥ 1** (3D.2 LOAD-BEARING)
- `feedback_events` with `founder_approved` or `founder_rejected` ≥ 1
- `audit_log fomo.slack.interaction_received` ≥ 1 (inbound observed)
- `audit_log fomo.slack.approval_captured` ≥ 1 (success path)
- **Leak-canary scan green** (no body / no raw payload / no full user_id / no signing-secret-shape strings)

Recommended-WARN:
- `audit_log fomo.slack.approval_duplicate` ≥ 1 (idempotency proof against live Postgres)

Capture the full stdout — it's the load-bearing §6 of the report.

---

## 10. Report

1. Copy [`docs/SMOKE_REPORT_TEMPLATE_3D2.md`](SMOKE_REPORT_TEMPLATE_3D2.md)
   to `docs/SMOKE_REPORT_3D2.md`.
2. Fill in every section.
3. If `VERDICT: PASS`: commit + push to this branch, merge the PR.
4. If `VERDICT: FAIL`: log the failure; do not merge. Common causes:
   - Slack signing secret wrong → `fomo.slack.signature_invalid` in audit
   - Wrong channel id → `fomo.slack.approval_unauthorized` with `error_code: wrong_channel`
   - Wrong user id (when SLACK_FOUNDER_USER_ID is set) → `wrong_user`
   - Tunnel offline → no `fomo.slack.interaction_received` at all
   - Kill switch off → `fomo.slack.signature_invalid` with `error_code: kill_switch_off`

---

## What "PASS" means in 3D.2

Per the founder-confirmed scope:

> **Real Slack workspace + real Slack app + real button click + real
> Neon persistence.** Every required check in the evidence must be
> green. The alert must reach a terminal `approved` or `rejected`
> state via a signed inbound interactivity POST from your founder
> Slack channel, with the correct user (if `SLACK_FOUNDER_USER_ID`
> is set), and the duplicate-click must be idempotent.

After 3D.2 PASS lands on main, Phase 3E (SendBlue Outbound) is unblocked.

---

## Cleanup (recommended)

After the smoke test, flip kill switches back off so the dev server
doesn't keep posting to Slack on every poll cycle:

```bash
# Edit apps/fomo/.env.3b3.local and set:
FOMO_SLACK_REVIEW_ENABLED=false
# OR remove the line entirely
```

Re-source and restart the server. The boot log should show:

```
fomo.slack.review.disabled ... /slack/interactivity route NOT mounted
fomo.server.listening      ... slack_interactivity_route_mounted: false
```
