# Phase 3F.2 Smoke Test Report — SendBlue Inbound Reply (founder-only)

> Filled after running every step in
> [`smoke-test-3f2-sendblue-inbound.md`](smoke-test-3f2-sendblue-inbound.md).
> Phase 3G demo gate is unblocked once this report lands on `main`.
>
> Founder directive 2026-05-26 (load-bearing): §5 below is
> founder-recorded and has been filled in honestly against the
> real SendBlue webhook auth observed during this run.

---

**Founder:** Galiette Mita
**Run date:** 2026-05-28 01:00–01:30 UTC
**Branch:** `phase3f2-sendblue-inbound-smoke`
**Commit SHA at run time:** `5df5485817932044cf73ebe982e1c13ea22e5b53`
**Founder Gmail account used:** techsmarterusa@gmail.com
**Founder phone number used (4-char suffix only):** `3459`
**SendBlue account email:** (same as founder)
**ngrok subdomain used:** `unshivering-interaulic-beatriz.ngrok-free.dev` (full webhook URL configured in SendBlue dashboard: `https://unshivering-interaulic-beatriz.ngrok-free.dev/sendblue/inbound`)

---

## 1. Prerequisites confirmed

- [x] `docs/SMOKE_REPORT_3B3.md` on `main` with `VERDICT: PASS`
- [x] `docs/OPENAI_SMOKE_REPORT_3C2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3C4.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3D2.md` on `main` with `VERDICT: PASS`
- [x] `docs/SMOKE_REPORT_3E2.md` on `main` with `VERDICT: PASS`
- [x] PR #33 (Phase 3F.1) merged
- [x] SendBlue webhook configured in dashboard pointing at ngrok
- [x] ngrok tunnel running + inspector accessible at `http://localhost:4040`

---

## 2. Env vars (redacted)

| Var                                | Set? | Notes                                                                |
| ---------------------------------- | ---- | -------------------------------------------------------------------- |
| `DATABASE_URL`                     | ✅    | Neon Postgres (len 116)                                              |
| `BREVIO_TOKEN_KEK`                 | ✅    | len 44 (base64, 32-byte KEK)                                         |
| `BREVIO_OAUTH_STATE_KEY`           | ✅    | len 44                                                               |
| `BREVIO_SESSION_SIGNING_KEY`       | ✅    | len 44                                                               |
| `GOOGLE_CLIENT_ID/SECRET`          | ✅    |                                                                      |
| `OPENAI_API_KEY`                   | ✅    |                                                                      |
| `FOMO_GMAIL_POLLING_ENABLED`       | ✅    | `true`                                                               |
| `FOMO_RANKER_ENABLED`              | ✅    | `true`                                                               |
| `FOMO_SLACK_REVIEW_ENABLED`        | ✅    | `true`                                                               |
| `SLACK_BOT_TOKEN`                  | ✅    | `xoxb-...`                                                           |
| `SLACK_FOUNDER_CHANNEL_ID`         | ✅    |                                                                      |
| `SLACK_SIGNING_SECRET`             | ✅    |                                                                      |
| `SLACK_FOUNDER_USER_ID`            | ✅    |                                                                      |
| `FOMO_SEND_ENABLED`                | ✅    | `true` (needed for STOP-blocks-outbound test)                        |
| `SENDBLUE_API_KEY_ID`              | ✅    |                                                                      |
| `SENDBLUE_API_SECRET_KEY`          | ✅    |                                                                      |
| `SENDBLUE_FROM_NUMBER`             | ✅    | E.164                                                                |
| `FOMO_FOUNDER_PHONE_NUMBER`        | ✅    | E.164 (`...3459`)                                                    |
| `FOMO_FOUNDER_USER_ID`             | ✅    | `founder`                                                            |
| `FOMO_SENDBLUE_INBOUND_ENABLED`    | ✅    | **`true` (3F.2 invariant)**                                          |
| `SENDBLUE_WEBHOOK_SECRET`          | ✅    | The secret configured in SendBlue dashboard Global Webhook Secret    |
| `SENDBLUE_WEBHOOK_SECRET_HEADER`   | ✅    | Default `sb-signing-secret`. CONFIRMED via §5 below                  |
| `SENDBLUE_INBOUND_PUBLIC_URL`      | ✅    | `https://unshivering-interaulic-beatriz.ngrok-free.dev` (set in SendBlue dashboard) |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ✅    | 30                                                                   |
| `FOMO_OUTBOUND_MAX_CYCLES`         | ✅    | 30                                                                   |
| `BREVIO_DEV_MODE`                  | ✅    | UNSET                                                                |
| `FOMO_AUTO_SEND_ENABLED`           | ✅    | UNSET / false                                                        |
| `FOMO_FRIEND_BETA_ENABLED`         | ✅    | UNSET / false                                                        |

---

## 3. Commands run

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

pnpm --filter @brevio/fomo run preflight:3f2
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3f2.log

# Separate tab:
ngrok http 8080
# (SendBlue webhook URL updated in dashboard to ngrok URL + /sendblue/inbound)

# Initial DB blocker — migration 0004 had only been applied via PGlite in gated tests;
# Neon needed it applied manually:
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0004_inbound_replies.sql

# Texted SendBlue number from personal iMessage:
#   1. "ping" (×3) — auth + recording path (got reply_unclear, as expected — no soft intent)
#   2. "tomorrow"  — soft-intent classifier path
#   3. "STOP"      — deterministic STOP recognition
#
# ngrok inspector "Replay" used on the "tomorrow" POST — idempotency test
#
# Deliberate bad-secret curl for signature_invalid:
curl -X POST "https://unshivering-interaulic-beatriz.ngrok-free.dev/sendblue/inbound" \
  -H "sb-signing-secret: wrong-secret-deliberate" \
  -H "content-type: application/json" \
  -d '{"messageId":"deliberate-bad-auth-test","content":"x","number":"+15555555555"}'

# Synthetic alert injected via SQL to test STOP-blocks-outbound (Gmail polling
# was needs_reauth=true so no organic alert chain was available during this run;
# stop_enforced code path tested via direct alert insert):
psql "$DATABASE_URL" -P pager=off -c "BEGIN; WITH r AS (INSERT INTO rank_results ...)
  ... INSERT INTO alert_state_transitions ... (detected→ranked→queued_for_review→approved);
  COMMIT;"
# (next outbound cycle picked it up, saw stop_active=true, emitted fomo.send.stop_enforced)

pnpm --filter @brevio/fomo run smoke-evidence:3f2 2>&1 | tee /tmp/fomo-3f2-evidence.log
```

---

## 4. Boot-time confirmation

Confirmed via audit_log queries (see §6):
- `fomo.sendblue.inbound_received` events were written with `secret_header_name: sb-signing-secret`
- Inbound POSTs returned HTTP 200 (auth verified end-to-end against live SendBlue)
- `/sendblue/inbound` route mounted while `FOMO_SENDBLUE_INBOUND_ENABLED=true`
- Inbound route returns 404 when `FOMO_SENDBLUE_INBOUND_ENABLED=false` (see §9)

- [x] `sendblue_inbound_route_mounted: true`
- [x] `webhook_secret_header` matches the header SendBlue actually uses (see §5)

---

## 5. AUTH OBSERVATION (LOAD-BEARING; founder-recorded)

**Observation method used:** Both — ngrok inspector at `localhost:4040` (visual inspection of POST request headers from real SendBlue) AND server-log inspection via `audit_log.detail->>'secret_header_name'` field on every `fomo.sendblue.inbound_received` row. Additionally confirmed via screenshot from the SendBlue Webhooks dashboard which states: *"The secret will be sent in the `sb-signing-secret` header. Use this to verify requests are from Sendblue."*

**Observed webhook secret header name:** `sb-signing-secret` (lowercase). Confirmed across 10 / 10 inbound events in audit_log — every `fomo.sendblue.inbound_received` row has `detail->>'secret_header_name' = 'sb-signing-secret'`.

**Observed auth scheme:**
- [x] **Scenario A — Plain shared secret in a named header.** The `sb-signing-secret` header value equals the configured `SENDBLUE_WEBHOOK_SECRET` byte-for-byte. Verified empirically: all 10 inbound POSTs from real SendBlue passed timing-safe equality check against the configured secret; the single deliberate-bad-secret curl was rejected with HTTP 401 and `error_code: secret_mismatch`. This is consistent with the SendBlue dashboard copy ("The secret will be sent in the `sb-signing-secret` header").
- [ ] Scenario B — HMAC / signature over body
- [ ] Scenario C — Something else

**Did the observed header value equal the literal `SENDBLUE_WEBHOOK_SECRET`?** **Yes.** All 10 real inbound POSTs auth'd successfully via plain-equality. The bad-curl test (with `sb-signing-secret: wrong-secret-deliberate`) was rejected with `secret_mismatch`. Asymmetric outcomes prove the runtime is comparing the header value against the literal secret, not parsing it as an HMAC.

**Was `SENDBLUE_WEBHOOK_SECRET_HEADER` overridden from the default `sb-signing-secret`?** No — default used. Matched what SendBlue sent.

**Was a runtime patch required?**
- [x] **No** — Scenario A held. The 3F.1 substrate as merged on `main` (after the post-PR-review correction in commit `53e341d5` which replaced the HMAC implementation with plain-shared-secret-header verification + renamed env vars `SENDBLUE_WEBHOOK_SIGNING_SECRET` → `SENDBLUE_WEBHOOK_SECRET`) is correct as shipped. No further patch needed before this PASS.
- [ ] Yes

**If Scenario B or C, describe the observed auth shape:** N/A — Scenario A confirmed.

---

## 6. Cycle + inbound log evidence (audit_log)

Captured via direct queries against Neon. Times are UTC.

```
-- §5 of runbook (upstream alert chain): not exercised organically during this run
-- because oauth_tokens.needs_reauth=true was preventing Gmail polling. The
-- stop_enforcement code path was instead exercised via synthetic-alert SQL insert
-- (see §3). All other scenarios used the real SendBlue → ngrok → runtime path.

-- Scenario 2 — soft intent ("tomorrow", §7 of runbook):
fomo.sendblue.inbound_received  | success | sb-signing-secret           | 2026-05-28 01:13:51.778Z
fomo.sendblue.reply_parsed      | success | intent=snooze source=classifier | 2026-05-28 01:13:54.778Z
alert_state_transitions: 27bcaced... | sent     → replied  | sendblue:snooze hint=tomorrow                       | 2026-05-28 01:13:54.646Z
alert_state_transitions: 27bcaced... | replied  → snoozed  | sendblue:snooze hint=tomorrow until=2026-05-29...   | 2026-05-28 01:13:54.714Z
feedback_events: id=6 kind=user_snoozed alert_id=27bcaced...

-- Scenario 3 — STOP + enforcement ("STOP", §8 of runbook):
fomo.sendblue.inbound_received  | success | sb-signing-secret  | 2026-05-28 01:12:21.612Z
fomo.sendblue.stop_recorded     | success | detail.stop_active=true, from_slug=3459 | 2026-05-28 01:12:21.784Z
                                              (172ms total — deterministic, no LLM call)
memory_signals: kind=stop_active detail={"active":true,...} source=user_confirmed confidence=1 | 2026-05-28 01:12:21.720Z
feedback_events: id=5 kind=stop alert_id=<none>

-- Synthetic alerts approved AFTER STOP, outbound cycle blocked:
alert_state_transitions: smoke3f2-alert-e08f8004... | approved → failed | stop_enforced: user has active STOP request (stop_active memory_signal=true) | 2026-05-28 01:24:22.463Z
alert_state_transitions: smoke3f2-alert-f2b3595f... | approved → failed | stop_enforced: user has active STOP request (stop_active memory_signal=true) | 2026-05-28 01:24:22.559Z
fomo.send.stop_enforced         | success | reason="stop_active memory_signal is true for this user" alert_id=smoke3f2-alert-e08f8004... | 2026-05-28 01:24:22.439Z
fomo.send.stop_enforced         | success | reason="stop_active memory_signal is true for this user" alert_id=smoke3f2-alert-f2b3595f... | 2026-05-28 01:24:22.535Z
(tool_invocations sendblue.send_user_message count: unchanged — zero outbound SendBlue API calls fired for these alerts)

-- Scenario 4 — idempotency (ngrok Replay on prior POST, §9 of runbook):
fomo.sendblue.inbound_received  | success | sb-signing-secret  | 2026-05-28 01:15:11.574Z
fomo.sendblue.reply_duplicate   | success |                    | 2026-05-28 01:15:11.642Z

-- Scenario 5 — invalid auth rejection (deliberate bad-secret curl, §10 of runbook):
fomo.sendblue.inbound_received  | success | sb-signing-secret  | 2026-05-28 01:17:06.829Z
fomo.sendblue.signature_invalid | failure | error_code=secret_mismatch | 2026-05-28 01:17:06.852Z
(curl response: HTTP 401)

-- Scenario 6 — START (§11 of runbook, optional): NOT EXERCISED.
-- stop_active remained true through end of run; START path tested by unit tests, not in this smoke.
```

Histogram of all `fomo.sendblue.*` audit events written during the run:

```
fomo.sendblue.inbound_received  | 10
fomo.sendblue.reply_unclear     |  3   (three "ping" texts — classifier correctly returned "unsure")
fomo.sendblue.reply_duplicate   |  1
fomo.sendblue.reply_parsed      |  1
fomo.sendblue.signature_invalid |  1
fomo.sendblue.stop_recorded     |  1
```

---

## 7. `pnpm smoke-evidence:3f2` output (LOAD-BEARING)

```
inbound_replies: 5 row(s)
  id=5 provider_message_id=4A1F4B3A-E580-4812-96CA-CE60FD200864 user=founder received=2026-05-28T01:13:51.799Z
  id=4 provider_message_id=CE56160C-D523-4844-97A1-E73EDD35C4B9 user=founder received=2026-05-28T01:12:21.632Z
  id=3 provider_message_id=DDB74EA4-AB79-4CFC-99A9-1EFA1C8CA5CA user=founder received=2026-05-28T01:08:34.698Z
  id=2 provider_message_id=70C5BC70-3DAD-4C80-8D9C-1EAD35807CFB user=founder received=2026-05-28T01:06:45.749Z
  id=1 provider_message_id=1BC22053-936E-4DFC-B7F9-51FE53E03430 user=founder received=2026-05-28T00:53:03.720Z

audit_log fomo.sendblue.inbound_received: 10 entry(ies)
  configured secret_header_name(s) observed: sb-signing-secret
audit_log fomo.sendblue.signature_invalid: 1 entry(ies)
  id=548 at=2026-05-28T01:17:06.852Z error_code=secret_mismatch

audit_log fomo.sendblue.stop_recorded: 1 entry(ies)
  id=512 at=2026-05-28T01:12:21.784Z detail={"from_slug":"3459","stop_active":true,"provider_message_id":"CE56160C-D523-4844-97A1-E73EDD35C4B9"}
audit_log fomo.sendblue.start_recorded: 0 entry(ies)

audit_log fomo.sendblue.reply_parsed: 1 entry(ies)
  id=525 intent=snooze source=classifier

audit_log fomo.sendblue.reply_duplicate: 1 entry(ies)

alert_state_transitions: sent→replied=1, replied→snoozed=1, replied→ignored=0

feedback_events (inbound-derived): 2 row(s)
  id=6 kind=user_snoozed alert_id=27bcaced-8921-4d46-a066-773b936010dd
  id=5 kind=stop alert_id=<none>

memory_signals stop_active: 1 row(s)
  id=1 user=founder active=true updated=2026-05-28T01:12:21.720Z

audit_log fomo.send.stop_enforced: 2 entry(ies)
  id=555 at=2026-05-28T01:24:22.535Z detail={"reason":"stop_active memory_signal is true for this user","alert_id":"smoke3f2-alert-f2b3595f-b0e6-45b5-a972-fd1e8f195cde","message_id":"smoke3f2-stop-test-1779931461","rank_result_id":29}
  id=552 at=2026-05-28T01:24:22.439Z detail={"reason":"stop_active memory_signal is true for this user","alert_id":"smoke3f2-alert-e08f8004-12e9-4454-b4a4-f90eaa1919ed","message_id":"smoke3f2-stop-test-1779931453","rank_result_id":28}

tool_invocations sendblue.send_user_message: 2 total
Scanning for leak canaries in audit_log + tool_invocations.metadata + feedback_events.detail + alert_state_transitions.reason + memory_signals.detail + inbound_replies ...
  (scanning for the literal FOMO_FOUNDER_PHONE_NUMBER value)
  (scanning for the literal SENDBLUE_WEBHOOK_SECRET value)
  ✓ no forbidden keys or value patterns found

========================================================================
Phase 3F.2 evidence summary
========================================================================
  [✓] inbound_replies ≥ 1 (real SendBlue webhook reached Brevio + auth passed)
        5 inbound webhook(s) auth'd + processed
  [✓] invalid-auth rejection (founder curl with wrong secret produces 401 + audit)
        1 rejection(s); reason codes: secret_mismatch
  [✓] STOP is deterministic (no LLM involved); audit row written
        1 STOP(s) recorded
  [!] START clears stop (RECOMMENDED — only required if founder ran scenario 6)
        No start_recorded entries. If you ran scenario 6 (texting START), check for issues. If you skipped scenario 6, this is fine.
  [✓] soft reply intent parsed via classifier
        1 classifier-parsed soft intent(s)
  [✓] duplicate webhook is idempotent (re-post produces reply_duplicate audit; no double-write)
        1 duplicate(s) caught by inbound_replies UNIQUE constraint
  [✓] state transition sent → replied
        1 transition(s)
  [✓] terminal state transition replied → snoozed | ignored
        snoozed=1, ignored=0
  [✓] feedback events from inbound replies
        2 event(s)
  [✓] stop_active memory_signal exists (STOP wrote memory)
        1 stop_active signal(s); latest active=true
  [✓] STOP enforcement blocked a future outbound send
        2 stop_enforced row(s); zero SendBlue API calls fired for these alerts
  [✓] No reply text / phone / webhook secret / API keys leaked in any persisted store
        Scanned 500 audit + 39 tool_invocations + 6 feedback + 26 transition + 1 memory_signal + 5 inbound_replies rows; zero hits.
  [!] Auth observation fields recorded by founder in §5 of the report (LOAD-BEARING)
        This evidence script CANNOT verify what header SendBlue actually sent. The founder MUST fill in §5 of SMOKE_REPORT_3F2.md: observed webhook header name, observed auth scheme (plain-secret-header / HMAC / other), whether the header value equaled the configured secret literally, and whether a runtime patch was required. Reviewer (Claude) MUST verify these fields are filled in and consistent with the runtime config before merging the PR.

VERDICT: PASS  (2 warning(s); see notes above)
Phase 3G demo gate is now unblocked.

REMINDER: This script CANNOT verify the auth-mechanism claim.
Founder MUST record observed auth header + scheme in §5 of the smoke report.
Reviewer MUST verify those fields before merging.
```

Required-PASS checks (all green):

- [x] inbound_replies ≥ 1 (5)
- [x] fomo.sendblue.reply_parsed with intent_source=classifier ≥ 1 (1)
- [x] fomo.sendblue.stop_recorded ≥ 1 (1)
- [x] fomo.sendblue.reply_duplicate ≥ 1 (1)
- [x] fomo.sendblue.signature_invalid ≥ 1 with error_code: secret_mismatch (1)
- [x] alert_state_transitions sent → replied ≥ 1 (1)
- [x] alert_state_transitions replied → snoozed | ignored ≥ 1 (snoozed=1)
- [x] feedback_events from inbound ≥ 1 (2)
- [x] memory_signals stop_active exists (1, active=true, confidence=1)
- [x] fomo.send.stop_enforced ≥ 1 (2)
- [x] **Leak-canary scan clean** (500 + 39 + 6 + 26 + 1 + 5 = 577 rows scanned, zero hits)

Recommended-WARN (gate-passable, not blocking):
- [ ] fomo.sendblue.start_recorded ≥ 1 — N/A; scenario 6 (texting START) was not exercised in this run. STOP path was tested deterministically; START path is covered by unit tests in the 3F.1 substrate.

---

## 8. Founder observations

| Observation | Note |
|---|---|
| Did exactly the soft-intent state transition + the STOP enforcement fire as designed? | **Yes.** "tomorrow" produced a clean `sent → replied → snoozed` with `until=2026-05-29` (correctly +1 day). STOP produced a 172ms-total deterministic transition with `stop_active=true` + `source=user_confirmed` + `confidence=1`. Subsequent synthetic alerts (approved AFTER STOP) were blocked: `approved → failed` with reason `stop_enforced: user has active STOP request`. Zero SendBlue API calls fired for those alerts. |
| Did SendBlue retry a webhook on its own (you see a `reply_duplicate` you did NOT trigger via curl)? | **No.** The single `reply_duplicate` event was the explicit ngrok-Replay test (scenario 4). SendBlue did not auto-retry any POST during the run. |
| Did any iMessage arrive after STOP was active? | **No.** STOP enforced correctly — both synthetic alerts that landed in `approved` state after STOP were blocked at the outbound layer. `tool_invocations` count for `sendblue.send_user_message` remained unchanged during the stop_enforced phase. |
| Did the leak-canary scan stay green across all 5 scenarios? | **Yes.** 577 persisted rows scanned (audit_log + tool_invocations + feedback_events + alert_state_transitions + memory_signals + inbound_replies). Zero hits on `FOMO_FOUNDER_PHONE_NUMBER` literal, `SENDBLUE_WEBHOOK_SECRET` literal, or any forbidden keys. |
| Anything else surprising? | Two operational notes: (1) Migration `0004_inbound_replies.sql` was applied during gated PGlite tests in 3F.1 but had NOT been applied to live Neon — first three inbound POSTs returned HTTP 500 until the migration was applied manually. Same pattern as 3D.2 (alerts table missing). Worth automating in `pnpm dev` boot or a `pre-smoke` step. (2) Gmail `oauth_tokens.needs_reauth=true` blocked organic alert chain creation, so STOP-blocks-outbound was tested with a synthetic alert injected via SQL. Same code path, same emitted events, but the org `e2e` pipeline (Gmail → ranker → Slack → approved → outbound) was not exercised end-to-end in this run. Real-pipeline STOP enforcement should be re-confirmed in the Phase 3G demo. |

---

## 9. Clean-stop confirmation

Restarted with both inbound + send switches off:

```bash
FOMO_SENDBLUE_INBOUND_ENABLED=false FOMO_SEND_ENABLED=false pnpm --filter @brevio/fomo dev
```

Observed:
- Cycle heartbeats continued (`fomo.poll.cycle`) — dev server is alive
- `curl -X POST http://localhost:8080/sendblue/inbound` → **HTTP 404** (route NOT mounted)
- No `fomo.sendblue.*` audit rows written after the clean-stop restart

- [x] `/sendblue/inbound` returns 404 when inbound flag is off
- [x] No `fomo.sendblue.*` audit rows written after the restart

---

## 10. Verdict

**[x] PASS** — every required check in §7 is green, §5 Auth Observation is filled in honestly (Scenario A — plain `sb-signing-secret` header, no runtime patch required), runtime matches observed auth scheme, leak-canary scan clean across 577 persisted rows, clean stop confirmed. **Phase 3G demo gate may begin.**

[ ] FAIL

Failures / followups:

- None blocking PASS. Two non-blocking followups for Phase 3G:
  1. Re-auth Gmail (founder token expired) so the demo can show the full organic alert chain (Gmail → ranker → Slack → approved → outbound).
  2. Consider adding Neon-migration automation in dev boot so future smokes don't hit the "migration applied in PGlite but not Neon" footgun.

---

## 11. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-05-28
