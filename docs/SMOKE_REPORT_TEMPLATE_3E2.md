# Phase 3E.2 Smoke Test Report — SendBlue Outbound Real iMessage (founder-only)

> Fill this template after running every step in
> [`smoke-test-3e2-sendblue-outbound.md`](smoke-test-3e2-sendblue-outbound.md).
> Commit as `docs/SMOKE_REPORT_3E2.md` once `VERDICT: PASS`. **Phase 3F
> SendBlue inbound does NOT start until this report lands on `main`.**

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3e2-sendblue-outbound-smoke`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Founder Gmail account used:** _<email>_
**Founder phone number used (4-char suffix only, e.g. ...1234):** _<last 4 digits>_
**SendBlue account email:** _<email>_
**Sandbox dry run performed first?** _<yes / no — sandbox unavailable / no — accepted production risk>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [ ] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [ ] PR #30 (Phase 3E.1) merged
- [ ] SendBlue account active with API key id + secret key
- [ ] Founder phone number ready to receive iMessages

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

Confirm each was set during the run. Do NOT paste secret values.

| Var                                | Set? | Notes                                                              |
| ---------------------------------- | ---- | ------------------------------------------------------------------ |
| `DATABASE_URL`                     | ☐    | Neon Postgres                                                      |
| `BREVIO_TOKEN_KEK`                 | ☐    |                                                                    |
| `BREVIO_OAUTH_STATE_KEY`           | ☐    |                                                                    |
| `BREVIO_SESSION_SIGNING_KEY`       | ☐    |                                                                    |
| `GOOGLE_CLIENT_ID/SECRET`          | ☐    |                                                                    |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ☐    |                                                                    |
| `OPENAI_API_KEY`                   | ☐    |                                                                    |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☐    | `true`                                                             |
| `FOMO_RANKER_ENABLED`              | ☐    | `true`                                                             |
| `FOMO_SLACK_REVIEW_ENABLED`        | ☐    | `true`                                                             |
| `SLACK_BOT_TOKEN`                  | ☐    | `xoxb-...`                                                         |
| `SLACK_FOUNDER_CHANNEL_ID`         | ☐    | `C0123...`                                                         |
| `SLACK_SIGNING_SECRET`             | ☐    |                                                                    |
| `SLACK_FOUNDER_USER_ID`            | ☐    | `U0123...`                                                         |
| `FOMO_SEND_ENABLED`                | ☐    | **`true` (3E.2 invariant)**                                        |
| `SENDBLUE_API_KEY_ID`              | ☐    |                                                                    |
| `SENDBLUE_API_SECRET_KEY`          | ☐    |                                                                    |
| `SENDBLUE_FROM_NUMBER`             | ☐    | SendBlue-assigned sender phone, E.164 (last 4 digits only)         |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ☐    | YOUR destination phone, E.164 (last 4 digits only above)           |
| `FOMO_FOUNDER_USER_ID`             | ☐    | The user_id whose alerts are allowed to text                        |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ☐    | Recommended: 1-3                                                   |
| `BREVIO_DEV_MODE`                  | ☐    | UNSET                                                              |
| `FOMO_AUTO_SEND_ENABLED`           | ☐    | UNSET / false                                                      |
| `FOMO_FRIEND_BETA_ENABLED`         | ☐    | UNSET / false                                                      |

---

## 3. Commands run

Paste the actual commands. Order matters.

```bash
# Preflight
pnpm --filter @brevio/fomo run preflight:3e2

# Build + start (T1)
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3e2.log

# Tunnel (T2) — same as 3D.2
ngrok http 8080   # OR cloudflared tunnel --url http://localhost:8080

# (Slack app Interactivity Request URL already configured from 3D.2 run)

# (Sent yourself an important-looking test email; watched the polling
#  worker rank + post to Slack)

# (Clicked Approve on the Slack card)

# (Waited up to FOMO_GMAIL_POLLING_INTERVAL_MS for the outbound worker
#  to pick up the approved alert and send the real iMessage)

# (Waited one more cycle to prove idempotency — no second iMessage)

# Evidence
pnpm --filter @brevio/fomo run smoke-evidence:3e2
```

---

## 4. Boot-time confirmation

Paste the boot log lines (verbatim, redacting any sensitive value):

```
fomo.send.enabled               ... founder_user_id: <uuid>, founder_phone_configured: true, auto_send_enabled: false
fomo.outbound.enabled           ... interval_ms: 60000, cycle_cap: <N>, auto_send_enabled: false
fomo.slack.review.enabled       ... interactivity_route_mounted: true
fomo.server.listening           ... send_enabled: true, outbound_worker_started: true
```

- [ ] `fomo.send.enabled` line present
- [ ] `fomo.outbound.enabled` line present with `cycle_cap` matching `FOMO_OUTBOUND_MAX_CYCLES`
- [ ] `fomo.server.listening` line shows `send_enabled: true, outbound_worker_started: true`
- [ ] `auto_send_enabled: false` in both `fomo.send.enabled` and `fomo.outbound.enabled` (3E.2 is manual-only)

---

## 5. Cycle + send log evidence (server stdout)

Paste the relevant log lines, in order:

```
{ ... fomo.poll.cycle ... messages_ranked: 1 ... }
{ ... fomo.rank.completed ... label: "important" ... }
{ ... alert.created ... alert_id: <NEW-UUID> ... }
{ ... fomo.slack.posted ... slack_ts: <...> ... }

{ ... fomo.slack.interaction_received ... }     ← founder clicks Approve
{ ... fomo.slack.approval_captured ... alert_id: <NEW-UUID>, decision_code: queued_for_review→approved ... }

{ ... fomo.outbound.cycle ... cycle_number: 1, alerts_considered: 1, alerts_sent: 1 ... }
{ ... fomo.send.attempted ... alert_id: <NEW-UUID>, destination_slug: <last 4>, template_version: founder-text-v0.1.0 ... }
{ ... policy.decided ... tool_id: sendblue.send_user_message, decision_code: allowed ... }
{ ... tool.invoked ... tool_id: sendblue.send_user_message, status: success ... }
{ ... fomo.send.succeeded ... alert_id: <NEW-UUID>, provider_status: <QUEUED|SENT|DELIVERED> ... }
{ ... state.transitioned ... alert_id: <NEW-UUID>, from_state: approved, to_state: sent ... }

{ ... fomo.outbound.cycle ... cycle_number: 2, alerts_considered: 0, alerts_sent: 0 ... }   ← idempotency
{ ... fomo.outbound.cycle_cap_reached  cycles_run: 2, cycle_cap: <N> ... }                  ← cap fired
```

---

## 6. `pnpm smoke-evidence:3e2` output (LOAD-BEARING)

Paste the full stdout verbatim. The verdict line at the bottom must
read `VERDICT: PASS` for this report to commit.

```
…
```

Required-PASS checks (the gate criteria):

- alert reached `approved` (3D.2 carry-forward)
- alert reached `sent` ← **3E.2 LOAD-BEARING**
- `tool_invocations sendblue.send_user_message` with `status=success` ≥ 1
- `fomo.send.attempted` ≥ 1
- `fomo.send.succeeded` ≥ 1
- `fomo.send.unauthorized_destination` == 0 (allowlist held)
- **Leak-canary scan clean** (no rendered text / no full phone / no API-key shapes / no raw payload anywhere)

Recommended-WARN (gate-passable):
- `fomo.outbound.cycle_cap_reached` line present in `/tmp/fomo-3e2.log` (paste verbatim below)
- No `fomo.send.failed` / `fomo.send.status_unknown` for the SAME alert that reached `sent`

---

## 7. Founder observations

| Observation | Note |
|---|---|
| Did exactly ONE iMessage arrive on your phone? | _<yes / no — how many>_ |
| Did the iMessage content match the deterministic template (label + masked sender + subject + snippet)? | _<yes / mismatched — paste exact text>_ |
| Did the iMessage contain anything surprising or sensitive (raw body, full sender email, headers)? | _<no / yes — DETAIL>_ |
| Did the second cycle correctly NOT re-send the same alert? | _<yes / no — SECOND IMESSAGE ARRIVED?>_ |
| Did `fomo.outbound.cycle_cap_reached` fire after `FOMO_OUTBOUND_MAX_CYCLES` cycles? | _<yes / no>_ |
| Any unexpected `fomo.send.failed` / `fomo.send.status_unknown` / `fomo.send.unauthorized_destination` rows? | _<no / yes — paste detail>_ |

Overall impression: _<one sentence — "real iMessage delivery works
end-to-end" / "X felt off">_

---

## 8. Clean-stop confirmation

After the smoke window, restart with the send switch off:

```bash
FOMO_SEND_ENABLED=false pnpm --filter @brevio/fomo run dev
```

You should see:

```
fomo.send.disabled              ... outbound sender worker dormant
fomo.outbound.disabled          ... outbound-sender worker dormant
fomo.server.listening           ... send_enabled: false, outbound_worker_started: false
```

- [ ] An approved alert (if any remains) does NOT get re-sent with the switch off
- [ ] No `fomo.send.*` audit rows written after the restart

---

## 9. Verdict

☐ **PASS** — every required check in §6 green, exactly one iMessage delivered, idempotency held, no leaks, clean stop confirmed. Phase 3F SendBlue inbound may begin.

☐ **FAIL** — list failures below; Phase 3F blocked until a re-run reaches PASS.

Failures / followups:

- _…_

---

## 10. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
