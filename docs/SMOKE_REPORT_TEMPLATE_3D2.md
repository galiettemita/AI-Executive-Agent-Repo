# Phase 3D.2 Smoke Test Report — Slack Approval Capture (real Slack + signed inbound)

> Fill this template after running every step in
> [`smoke-test-3d2-slack-approval.md`](smoke-test-3d2-slack-approval.md).
> Commit as `docs/SMOKE_REPORT_3D2.md` once `VERDICT: PASS`. **Phase 3E
> SendBlue does NOT start until this report lands on `main`.**

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3d2-slack-approval-capture`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Founder Gmail account used:** _<email>_
**Slack workspace:** _<workspace name>_
**Slack app name:** _<app name>_
**Slack founder channel id:** _<C0123...>_
**Slack founder user id:** _<U0123... or "<unset, best-effort>">_
**Tunneling tool:** _<ngrok / cloudflared / other>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [ ] PR #28 (Phase 3D.1) merged
- [ ] Slack app created with `chat:write` bot scope + Interactivity enabled
- [ ] Tunnel running and Slack app Interactivity Request URL points at it

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

Confirm each was set during the run. Do NOT paste secret values.

| Var                                | Set? | Notes                                          |
| ---------------------------------- | ---- | ---------------------------------------------- |
| `DATABASE_URL`                     | ☐    | Neon Postgres                                  |
| `BREVIO_TOKEN_KEK`                 | ☐    |                                                |
| `BREVIO_OAUTH_STATE_KEY`           | ☐    |                                                |
| `BREVIO_SESSION_SIGNING_KEY`       | ☐    |                                                |
| `GOOGLE_CLIENT_ID/SECRET`          | ☐    |                                                |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ☐    |                                                |
| `OPENAI_API_KEY`                   | ☐    |                                                |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☐    | `true`                                         |
| `FOMO_RANKER_ENABLED`              | ☐    | `true`                                         |
| `FOMO_SLACK_REVIEW_ENABLED`        | ☐    | `true`                                         |
| `SLACK_BOT_TOKEN`                  | ☐    | `xoxb-...`                                     |
| `SLACK_FOUNDER_CHANNEL_ID`         | ☐    | `C0123...`                                     |
| `SLACK_SIGNING_SECRET`             | ☐    | from Slack app Basic Information               |
| `SLACK_FOUNDER_USER_ID`            | ☐    | `U0123...` (recommended) or `<unset>`          |
| `SLACK_INTERACTIVITY_PUBLIC_URL`   | ☐    | (informational; the tunnel URL you configured) |
| `BREVIO_DEV_MODE`                  | ☐    | UNSET                                          |
| `FOMO_SEND_ENABLED`                | ☐    | UNSET / false                                  |
| `FOMO_AUTO_SEND_ENABLED`           | ☐    | UNSET / false                                  |
| `FOMO_FRIEND_BETA_ENABLED`         | ☐    | UNSET / false                                  |

---

## 3. Commands run

Paste the actual commands. Order matters.

```bash
# Preflight
pnpm --filter @brevio/fomo run preflight:3d2

# Build + start (T1)
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3d2.log

# Tunnel (T2)
ngrok http 8080   # OR cloudflared tunnel --url http://localhost:8080

# (Slack app: configure Interactivity Request URL to <public>/slack/interactivity, Save)

# (Sent yourself an important-looking test email; watched the polling
#  worker rank + post to Slack)

# (Clicked Approve on the Slack card)

# (Clicked Approve AGAIN on the same card to exercise idempotency)

# Evidence
pnpm --filter @brevio/fomo run smoke-evidence:3d2
```

---

## 4. Boot-time confirmation

Paste the three boot log lines (verbatim):

```
fomo.slack.review.enabled    ... interactivity_route_mounted: true, founder_user_restricted: true|false
fomo.poll.enabled            ... ranker_enabled: true, slack_review_enabled: true
fomo.server.listening        ... slack_interactivity_route_mounted: true
```

- [ ] `interactivity_route_mounted: true`
- [ ] `founder_user_restricted` matches whether `SLACK_FOUNDER_USER_ID` was set

---

## 5. Cycle + interactivity log evidence (server stdout)

Paste the relevant log lines, in order:

```
{ ... fomo.poll.cycle ... messages_ranked: 1 ... }
{ ... fomo.rank.completed ... label: "important" ... }
{ ... alert.created ... alert_id: <NEW-UUID> ... }
{ ... fomo.slack.posted ... slack_ts: <...> ... }

{ ... fomo.slack.interaction_received ... }     ← founder clicks Approve
{ ... fomo.slack.approval_captured ... alert_id: <NEW-UUID>, decision_code: queued_for_review→approved ... }

{ ... fomo.slack.interaction_received ... }     ← founder clicks Approve again (idempotency)
{ ... fomo.slack.approval_duplicate ... alert_id: <NEW-UUID>, current_state: approved ... }
```

---

## 6. `pnpm smoke-evidence:3d2` output (LOAD-BEARING)

Paste the full stdout verbatim. The verdict line at the bottom must
read `VERDICT: PASS` for this report to commit.

```
…
```

Required-PASS checks (the gate criteria):

- alerts table populated
- alert reached `queued_for_review`
- alert reached `approved` OR `rejected` ← **3D.2 LOAD-BEARING**
- feedback_events `founder_approved` / `founder_rejected` ≥ 1
- `fomo.slack.interaction_received` ≥ 1
- `fomo.slack.approval_captured` ≥ 1
- **Leak-canary scan clean** (no body / no raw payload / no full user_id)

Recommended-WARN (gate-passable):
- `fomo.slack.approval_duplicate` ≥ 1 (idempotency exercised)

---

## 7. Founder observations

| Observation | Note |
|---|---|
| Did the Slack card render correctly with Approve + Reject buttons? | _<yes / surprising>_ |
| Did the card update after Approve to show "Approved by ... at ..."? | _<yes / didn't fire — chat.update is best-effort>_ |
| Did the second click (idempotency) feel correct (no double action)? | _<yes / no>_ |
| Any unexpected `fomo.slack.signature_invalid` rows during the run? | _<yes / no — paste error_code if yes>_ |

Overall impression: _<one sentence — "trust checkpoint works as
designed" / "X felt off">_

---

## 8. Clean-stop confirmation

After the smoke window, restart with the switch off:

```bash
FOMO_SLACK_REVIEW_ENABLED=false pnpm --filter @brevio/fomo run dev
```

You should see:

```
fomo.slack.review.disabled  ... /slack/interactivity route NOT mounted
fomo.server.listening       ... slack_interactivity_route_mounted: false
```

- [ ] `/slack/interactivity` returns 404 with the switch off (confirmed via `curl -X POST http://localhost:8080/slack/interactivity` → 404)
- [ ] No `fomo.slack.*` audit rows written after the restart

---

## 9. Verdict

☐ **PASS** — every required check in §6 green, idempotency exercised, no leaks, clean stop confirmed. Phase 3E SendBlue may begin.

☐ **FAIL** — list failures below; Phase 3E blocked until a re-run reaches PASS.

Failures / followups:

- _…_

---

## 10. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
