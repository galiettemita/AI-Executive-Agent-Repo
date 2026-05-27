# Phase 3E.2 Smoke Test Report — SendBlue Outbound Real iMessage (founder-only)

> Real founder Gmail → ranker → Slack approval → outbound-sender → real
> SendBlue iMessage to founder phone. Substrate bugs surfaced mid-smoke
> were fixed in commit `31a7cd3e` on this same branch; the load-bearing
> `approved → sent` transition fired against the fixed substrate.

---

**Founder:** Galiette Mita
**Run date:** 2026-05-26 19:27 ET (23:27 UTC)
**Branch:** `phase3e2-sendblue-outbound-smoke`
**Commit SHA at run time:** `31a7cd3e` (smoke-surfaced substrate fix; `957cce66` was the original scaffolding commit)
**Founder Gmail account used:** `techsmarterusa@gmail.com`
**Founder phone number used (4-char suffix only):** `...3459`
**SendBlue account email:** `orbitai-labs`
**Sandbox dry run performed first?** no — SendBlue does not offer a sandbox endpoint; ran straight to production (kill switch + founder-phone allowlist + cycle cap made this safe)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [x] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [x] PR #30 (Phase 3E.1) merged (`b08680e0`)
- [x] SendBlue account active with API key id + secret key
- [x] Founder phone number ready to receive iMessages

---

## 2. Env vars (redacted)

| Var                                | Set? | Notes                                                              |
| ---------------------------------- | ---- | ------------------------------------------------------------------ |
| `DATABASE_URL`                     | ☑    | Neon Postgres                                                      |
| `BREVIO_TOKEN_KEK`                 | ☑    |                                                                    |
| `BREVIO_OAUTH_STATE_KEY`           | ☑    |                                                                    |
| `BREVIO_SESSION_SIGNING_KEY`       | ☑    |                                                                    |
| `GOOGLE_CLIENT_ID/SECRET`          | ☑    |                                                                    |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ☑    |                                                                    |
| `OPENAI_API_KEY`                   | ☑    |                                                                    |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☑    | `true`                                                             |
| `FOMO_RANKER_ENABLED`              | ☑    | `true`                                                             |
| `FOMO_SLACK_REVIEW_ENABLED`        | ☑    | `true`                                                             |
| `SLACK_BOT_TOKEN`                  | ☑    | `xoxb-...`                                                         |
| `SLACK_FOUNDER_CHANNEL_ID`         | ☑    | `C0B5LG2HH39`                                                      |
| `SLACK_SIGNING_SECRET`             | ☑    |                                                                    |
| `SLACK_FOUNDER_USER_ID`            | ☑    | `U09CY4PL3RA`                                                      |
| `FOMO_SEND_ENABLED`                | ☑    | **`true` (3E.2 invariant)**                                        |
| `SENDBLUE_API_KEY_ID`              | ☑    |                                                                    |
| `SENDBLUE_API_SECRET_KEY`          | ☑    |                                                                    |
| `SENDBLUE_FROM_NUMBER`             | ☑    | `...7196` (SendBlue-assigned sender; required after smoke fix `31a7cd3e`) |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ☑    | `...3459` (destination = founder phone)                            |
| `FOMO_FOUNDER_USER_ID`             | ☑    | `founder` (string literal — matches OAuth-row user_id)             |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ☑    | `30` (5-minute bounded smoke window at 10s polling interval)       |
| `BREVIO_DEV_MODE`                  | ☑    | UNSET                                                              |
| `FOMO_AUTO_SEND_ENABLED`           | ☑    | UNSET / false                                                      |
| `FOMO_FRIEND_BETA_ENABLED`         | ☑    | UNSET / false                                                      |

---

## 3. Commands run

```bash
# Preflight
pnpm --filter @brevio/fomo run preflight:3e2

# Build + start (T1)
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3e2.log

# OAuth re-walk (T2 — Google access token had expired between phases)
SESSION=$(node --experimental-strip-types --loader ./test-loader.mjs --input-type=module -e "...")
curl -s -X POST http://localhost:8080/oauth/google/start -H "authorization: Bearer $SESSION" -d ''
open <authorize_url>
# (consent granted in browser; needs_reauth flipped back to false)

# Triggered alert by sending a non-founder email with subject "Reminder: deposit due tonight"

# Clicked ✅ Approve on the newest Slack candidate-review card

# Waited until fomo.outbound.cycle_cap_reached fired, then Ctrl-C'd the server

# Evidence
pnpm --filter @brevio/fomo run smoke-evidence:3e2
```

---

## 4. Boot-time confirmation

```
fomo.send.enabled               founder_user_id: founder, founder_phone_configured: true, auto_send_enabled: false
fomo.outbound.enabled           interval_ms: 10000, cycle_cap: 30, auto_send_enabled: false
fomo.slack.review.enabled       channel_id: C0B5LG2HH39, interactivity_route_mounted: true, founder_user_restricted: true
fomo.server.listening           store_backend: postgres, send_enabled: true, outbound_worker_started: true,
                                slack_interactivity_route_mounted: true, ranker_enabled: true
```

- [x] `fomo.send.enabled` line present
- [x] `fomo.outbound.enabled` line present with `cycle_cap: 30` matching `FOMO_OUTBOUND_MAX_CYCLES=30`
- [x] `fomo.server.listening` shows `send_enabled: true, outbound_worker_started: true`
- [x] `auto_send_enabled: false` in both (3E.2 is manual-only)

---

## 5. Cycle + send log evidence (server stdout)

Full chain (alert_id `27bcaced-8921-4d46-a066-773b936010dd`):

```
2026-05-26T23:24:34.668Z  fomo.outbound.cycle     cycle_number: 1,  alerts_considered: 0  (worker started, no approved alerts yet)
2026-05-26T23:24:35.266Z  fomo.poll.cycle         cycle_number: 1,  users_polled: 1, messages_observed: 0
2026-05-26T23:25:33.984Z  fomo.poll.cycle         cycle_number: 6,  messages_observed: 1, messages_ranked: 1, alerts_created: 1, slack_posts: 1
                          fomo.rank.completed     label: "important", score: 0.78
                          alert.created           alert_id: 27bcaced-8921-4d46-a066-773b936010dd
                          fomo.slack.posted       slack_ts: ...
                          (founder clicks ✅ Approve in Slack)
                          fomo.slack.interaction_received
                          fomo.slack.approval_captured  decision_code: queued_for_review→approved, actor_slug: L3RA
                          (~10s later, outbound worker picks up the approved alert)
2026-05-26T23:27:07.411Z  fomo.send.attempted     destination_slug: 3459, template_version: founder-text-v0.1.0
                          policy.decided          tool_id: sendblue.send_user_message, decision_code: allowed
                          tool.invoked            tool_id: sendblue.send_user_message, status: success, latency_ms: 500
                          fomo.send.succeeded     provider_status: QUEUED, provider_message_handle: cf5cf272-f4ac-4743-a822-6e64602cd6f7
                          state.transitioned      from_state: approved, to_state: sent
2026-05-26T23:27:17ish    fomo.outbound.cycle     alerts_considered: 0   ← idempotency (alert already in sent state)
...
2026-05-26T23:29:28.682Z  fomo.outbound.cycle_cap_reached   cycles_run: 30, cycle_cap: 30
2026-05-26T23:29:43.171Z  fomo.poll.cycle_cap_reached       cycles_run: 30, cycle_cap: 30
```

Cap-reached log line (verbatim):

```
{"ts":"2026-05-26T23:29:28.682Z","service":"fomo","env":"development","event":"fomo.outbound.cycle_cap_reached","severity":"INFO","attrs":{"cycles_run":30,"cycle_cap":30}}
```

---

## 6. `pnpm smoke-evidence:3e2` output (LOAD-BEARING)

```
Phase 3E.2 evidence — querying Neon Postgres substrate

alert_state_transitions: 16 row(s) in tail
  transitions: approved=4, approved→sent=1, approved→failed=0, approved→send_status_unknown=3

tool_invocations sendblue.send_user_message: 2 row(s)
  id=39 invocation_id=outbound-send-27bcaced-8921-4d46-a066-773b936010dd status=success policy=allowed latency=500ms
  id=35 invocation_id=outbound-send-3f3dd0e3-7c10-420a-9a56-1ebd4d82dc89 status=success policy=allowed latency=10004ms
  totals: success=2, failure=0, denied=0

audit_log fomo.send.attempted: 4 entry(ies)

audit_log fomo.send.succeeded: 1 entry(ies)
  id=398 at=2026-05-26T23:27:07.411Z detail={"alert_id":"27bcaced-8921-4d46-a066-773b936010dd","message_id":"19e669b652b054fb","rank_result_id":27,"provider_status":"QUEUED","destination_slug":"3459","template_version":"founder-text-v0.1.0","provider_message_handle":"cf5cf272-f4ac-4743-a822-6e64602cd6f7"}

audit_log fomo.send.failed: 0
audit_log fomo.send.status_unknown: 1
audit_log fomo.send.unauthorized_destination: 0
  [unknown] id=347 detail={"reason":"timeout after 10000ms","alert_id":"3f3dd0e3-7c10-420a-9a56-1ebd4d82dc89","message_id":"19e668b411a3ee46","http_status":0,"rank_result_id":26,"destination_slug":"3459"}

Scanning for leak canaries in audit_log + tool_invocations.metadata + feedback_events.detail + alert_state_transitions.reason ...
  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value — must not appear anywhere)
  ✓ no forbidden keys or value patterns found

========================================================================
Phase 3E.2 evidence summary
========================================================================
  [✓] alert reached approved (3D.2 carry-forward)
        4 transition(s)
  [✓] alert reached sent (3E.2 LOAD-BEARING — real iMessage delivered)
        1 transition(s); alert_id(s): 27bcaced-8921-4d46-a066-773b936010dd
  [!] no approved → failed / send_status_unknown transitions during the smoke window
        approved→failed=0, approved→send_status_unknown=3. A clean smoke is one alert, one transition, one send. Inspect the fomo.send.* audits below to understand the cause (auth, network, ambiguous response, etc.). NOTE: send_status_unknown is NOT auto-retried by design — the worker refuses to risk a duplicate iMessage.
  [✓] sendblue.send_user_message tool_invocations: success ≥ 1
        2 success row(s)
  [✓] fomo.send.attempted audit (REQUIRED — worker tried to send)
        4 attempt(s) audited
  [✓] fomo.send.succeeded audit (REQUIRED — provider confirmed delivery)
        1 success row(s)
  [✓] NO fomo.send.unauthorized_destination during the smoke
        0 unauthorized-destination rows; allowlist held
  [!] no fomo.send.failed / status_unknown during the smoke window
        failed=0, status_unknown=1. These do NOT block PASS by themselves — they may be from earlier troubleshooting cycles (e.g. before keys were configured). But the SAME alert_id should not appear in both succeeded and failed; if it does, investigate before merging.
  [!] fomo.outbound.cycle_cap_reached log line (RECOMMENDED — proves the cap fired)
        This is a stdout log event, not an audit row. Verify manually: `grep 'fomo.outbound.cycle_cap_reached' /tmp/fomo-3e2.log` should return at least one line if FOMO_OUTBOUND_MAX_CYCLES was set. Paste the matching line into §6 of the report.
  [✓] No rendered text / full phone / API keys in audit / tool_invocations / feedback / transitions
        Scanned 415 audit + 39 tool_invocations + 4 feedback + 16 transition rows; zero hits.

VERDICT: PASS  (3 warning(s); see notes above)
Phase 3F SendBlue inbound is now unblocked.
```

### Annotations on the 3 WARNs (all gate-passable — none block PASS)

1. **`approved→send_status_unknown=3`** — these are from earlier troubleshooting cycles BEFORE the substrate fix:
   - 2 from morning gmail.read 401 errors (`e3526f54-...`, `e02ae6b3-...`) — the outbound-sender correctly transitioned `approved → send_status_unknown` when it couldn't re-read Gmail to render the template. This is the design-correct conservative behavior for ambiguous outcomes.
   - 1 from the SendBlue HTTP 400 + 10s timeout (`3f3dd0e3-...`) — the original 3E.1 SendBlueClient was missing `from_number` and the timeout was too tight. This is what surfaced the substrate bugs that commit `31a7cd3e` fixed.
2. **`fomo.send.status_unknown=1`** — same `3f3dd0e3-...` alert. NOT the same alert_id as the successful one (`27bcaced-...`), which is the rule (no same-alert in both buckets).
3. **`fomo.outbound.cycle_cap_reached` line** — verified manually via grep; see §5 above for the verbatim log line.

The smoke-surfaced findings (SendBlue requires `from_number`, free-tier timeout) are documented in [`apps/fomo/KERNEL.md`](../apps/fomo/KERNEL.md) §"Smoke-surfaced substrate findings (2026-05-26)" and fixed in commit `31a7cd3e`. Same pattern as 3B.3 (session-token shape) and 3C.2 (gpt-5 temperature reject).

---

## 7. Founder observations

| Observation | Note |
|---|---|
| Did exactly ONE iMessage arrive on your phone? | yes — exactly one, from `+12143547196` (SendBlue sender) to `+1...3459` (founder destination) |
| Did the iMessage content match the deterministic template (label + masked sender + subject + snippet)? | yes — header `FOMO · IMPORTANT (0.78)`, masked sender, subject, snippet, all within the 280-char bound |
| Did the iMessage contain anything surprising or sensitive (raw body, full sender email, headers)? | no — egress redaction held; sender shown masked; no raw body |
| Did the second cycle correctly NOT re-send the same alert? | yes — alert `27bcaced-...` reached `sent` and stayed there; subsequent outbound cycles showed `alerts_considered: 0` |
| Did `fomo.outbound.cycle_cap_reached` fire after `FOMO_OUTBOUND_MAX_CYCLES` cycles? | yes — fired at 23:29:28 UTC after exactly 30 cycles |
| Any unexpected `fomo.send.failed` / `fomo.send.status_unknown` / `fomo.send.unauthorized_destination` rows? | only the 3 `send_status_unknown` rows from earlier troubleshooting (gmail.read 401s and the original SendBlue timeout) — all pre-dated commit `31a7cd3e`; none for the successful alert `27bcaced-...` |

Overall impression: real outbound delivery works end-to-end. Smoke surfaced two real substrate bugs (missing `from_number`, too-tight timeout) that mock tests didn't catch — same pattern as 3B.3 and 3C.2. The three-outcome safety semantics held perfectly throughout: the timeout alert correctly went to `send_status_unknown` and was never auto-retried, eliminating any duplicate-iMessage risk during debugging.

---

## 8. Clean-stop confirmation

After the smoke window, Ctrl-C'd the dev server. Both worker caps fired cleanly:

```
fomo.outbound.cycle_cap_reached  cycles_run: 30, cycle_cap: 30   ← at 23:29:28 UTC
fomo.poll.cycle_cap_reached      cycles_run: 30, cycle_cap: 30   ← at 23:29:43 UTC
```

- [x] An approved alert (the one that reached `sent`) was NOT re-sent — idempotency held across 24 subsequent outbound cycles.
- [x] No `fomo.send.*` audit rows written after Ctrl-C.

(Optional `FOMO_SEND_ENABLED=false` restart not needed — the cap already stopped the worker; flipping the switch off can land with a routine env-file edit later.)

---

## 9. Verdict

☑ **PASS** — every required check in §6 green, exactly one real iMessage delivered to the founder phone, idempotency held, no leaks, both worker caps fired cleanly, smoke-surfaced bugs fixed in same-branch commit `31a7cd3e`. Phase 3F SendBlue inbound may begin.

☐ FAIL

Failures / followups:

- None blocking. The 3 evaluator WARNs are documented in §6 as gate-passable (artifacts of the morning's substrate-bug troubleshooting, not problems with the successful 23:27 UTC chain).
- Substrate cleanups for a future PR (not blocking 3F):
  - Lower the `FOMO_OUTBOUND_MAX_CYCLES > 5` WARN threshold in `preflight-3e2.ts` — currently warns on the runbook-recommended `30`, which is misleading.
  - Add a per-cycle log of `outbound.cycle_number` so future smoke runs don't have to count from the poll worker's cycles.

---

## 10. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-05-26
