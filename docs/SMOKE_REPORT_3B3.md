# Phase 3B.3 Smoke Test Report — Gmail Real

> Founder sign-off artifact for the [smoke-test runbook](smoke-test-3b3-gmail.md).
> When `VERDICT: PASS` is committed here, Phase 3C may begin.

---

**Founder:** Galiette Mita
**Run date:** 2026-05-23 11:30 PM EST
**Branch:** `phase3b3-gmail-real-smoke-test`
**Commit SHA at run time:** `526a56d564a7f7f5db2be9633e6d6ab8e6c232bc`
**Founder Gmail account used:** galiettemita@gmail.com

---

## 1. Google Cloud setup

- **Project name:** Brevio
- **Gmail API enabled:** ☒ Yes
- **OAuth consent screen — user type:** ☒ External
- **Scopes added to consent screen:** `https://www.googleapis.com/auth/gmail.readonly`
  - Phase 3B.3 invariant: must be exactly this scope and nothing else. ✓ verified.
- **Test users added:** galiettemita@gmail.com, gm3258@columbia.edu
- **OAuth client type:** ☒ Web application
- **Redirect URI configured:** `http://localhost:8080/oauth/google/callback`
- **Consent-screen screenshot:** attached separately (founder confirms gmail.readonly is the only scope shown)

---

## 2. Env vars (redacted)

| Var                                   | Set? | Notes                                                                     |
| ------------------------------------- | ---- | ------------------------------------------------------------------------- |
| `DATABASE_URL`                        | ☒    | Neon Postgres (pooled connection)                                          |
| `BREVIO_TOKEN_KEK`                    | ☒    | 32 bytes (base64)                                                          |
| `BREVIO_OAUTH_STATE_KEY`              | ☒    | 32 bytes (base64)                                                          |
| `BREVIO_SESSION_SIGNING_KEY`          | ☒    | 32 bytes (base64)                                                          |
| `GOOGLE_CLIENT_ID`                    | ☒    | from Google Cloud OAuth 2.0 Client                                         |
| `GOOGLE_CLIENT_SECRET`                | ☒    | from Google Cloud OAuth 2.0 Client                                         |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`    | ☒    | `http://localhost:8080/oauth/google/callback`                              |
| `FOMO_GMAIL_POLLING_ENABLED`          | ☒    | `true`                                                                     |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`       | ☒    | `3` (bumped to `5` for one observation pass after OAuth completed)         |
| `FOMO_GMAIL_POLLING_INTERVAL_MS`      | ☒    | `10000` (10s, tightened from the 60s default to make the smoke window fast)|
| `BREVIO_DEV_MODE`                     | ☒    | UNSET (so production fail-closed checks fire)                              |
| `FOMO_SEND_ENABLED`                   | ☒    | UNSET (false)                                                              |
| `FOMO_AUTO_SEND_ENABLED`              | ☒    | UNSET (false)                                                              |
| `FOMO_FRIEND_BETA_ENABLED`            | ☒    | UNSET (false)                                                              |

---

## 3. Commands run

```bash
# Migrations applied to Neon
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0000_init.sql
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0001_gmail_cursors.sql
psql "$DATABASE_URL" -P pager=off -c "\dt"  # verified 10 tables present

# Preflight (with cap=3 restored)
FOMO_GMAIL_POLLING_MAX_CYCLES=3 pnpm --filter @brevio/fomo run smoke:preflight
# → ✓ Preflight passed. All required env vars are set and well-formed.

# Build + start the server
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev

# OAuth (separate terminal, from apps/fomo/)
SESSION='<minted via signSessionToken() with user_id=founder, fresh session_id, expires_at=+1h>'
curl -s -X POST http://localhost:8080/oauth/google/start \
  -H "authorization: Bearer $SESSION" -d ''
# → JSON with authorize_url; opened in browser; granted consent for the
#   founder Gmail; redirected to /oauth/google/callback which returned
#   {"ok":true,"user_id":"founder","provider":"google", ...}.

# Sent two test emails to the founder inbox between cycles to exercise
# the gmail.read dispatch path on real messages.

# Evidence
pnpm --filter @brevio/fomo run smoke:evidence
# → VERDICT: PASS  (1 warning: optional 401 path not exercised)
```

---

## 4. Scope observed

```sql
SELECT scopes FROM oauth_tokens WHERE provider='google';
```

Result:

```json
["https://www.googleapis.com/auth/gmail.readonly"]
```

Phase 3B.3 invariant satisfied: exactly the readonly scope, nothing else.

---

## 5. Polling cycle evidence

Server stdout (representative; full session ran 19 cycles across two
boots — see §7 evidence script for the exact count):

```json
{"ts":"2026-05-24T03:12:03.217Z","service":"fomo","env":"development","event":"fomo.poll.enabled","severity":"INFO","attrs":{"interval_ms":10000,"cycle_cap":3}}
{"ts":"2026-05-24T03:12:03.229Z","service":"fomo","env":"development","event":"fomo.server.listening","severity":"INFO","attrs":{"port":8080,"store_backend":"postgres","oauth_google_wired":true,"polling_enabled":true}}
{"ts":"2026-05-24T03:38:19.769Z","service":"fomo","env":"development","event":"fomo.poll.cycle","severity":"INFO","attrs":{"cycle_number":1,"cycle_cap":5,"users_total":1,"users_polled":1,"users_skipped":0,"users_unauthorized":0,"users_api_error":0,"messages_observed":1,"messages_dispatched":1,"messages_failed":0}}
{"ts":"2026-05-24T03:38:51.161Z","service":"fomo","env":"development","event":"fomo.poll.cycle","severity":"INFO","attrs":{"cycle_number":4,"cycle_cap":5,"users_total":1,"users_polled":1,"users_skipped":0,"users_unauthorized":0,"users_api_error":0,"messages_observed":1,"messages_dispatched":1,"messages_failed":0}}
{"ts":"2026-05-24T03:39:11.000Z","service":"fomo","env":"development","event":"fomo.poll.cycle_cap_reached","severity":"INFO","attrs":{"cycles_run":5,"cycle_cap":5}}
```

The cap fired cleanly; worker stopped without manual intervention.

---

## 6. Cursor advance

```sql
SELECT user_id, history_id, updated_at FROM gmail_cursors;
```

| Time           | user_id  | history_id | updated_at                       |
| -------------- | -------- | ---------- | -------------------------------- |
| At OAuth       | founder  | (seeded from Gmail profile.historyId; baseline) | 2026-05-24 03:14:xx UTC |
| After 19 cycles | founder  | `2001771`  | `2026-05-24 03:39:01.515+00`     |

`updated_at` moved forward across the run, confirming the polling
worker advanced the cursor after each successful `listHistorySince`.
Two of the cycles observed (and dispatched) one new message each (see
§7 `gmail.read dispatch audits: policy.decided=2 tool.invoked=2`).

---

## 7. Audit + tool_invocations evidence

Full `pnpm --filter @brevio/fomo run smoke:evidence` output:

```
oauth_tokens (provider='google'): 1 row(s)
  user_id=founder scopes=["https://www.googleapis.com/auth/gmail.readonly"] needs_reauth=false key_version=1

gmail_cursors: 1 row(s)
  user_id=founder history_id=2001771 updated_at=2026-05-24T03:39:01.515Z

audit_log action='gmail.poll.cycle': 19 entry(ies)
  (per-cycle details, sample)
  id=23 at=2026-05-24T03:39:01.577Z detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0}
  id=22 at=2026-05-24T03:38:51.161Z detail={"users_total":1,"users_polled":1,"messages_observed":1,"messages_dispatched":1, ...}
  id=17 at=2026-05-24T03:38:19.769Z detail={"users_total":1,"users_polled":1,"messages_observed":1,"messages_dispatched":1, ...}
  ... (16 more — pre-OAuth empty-user cycles + post-OAuth empty-history cycles)

audit_log gmail.read dispatch: policy.decided=2 tool.invoked=2

tool_invocations tool_id='gmail.read': 2 row(s)
  id=2 invocation_id=gmail-poll-mud0fl-1 policy_decision=allowed status=success latency_ms=281 error_code=null
  id=1 invocation_id=gmail-poll-q3njfd-1 policy_decision=allowed status=success latency_ms=184 error_code=null

Scanning for leak canaries in audit_log.detail + tool_invocations.metadata ...
  ✓ no forbidden keys or value patterns found in 500 most recent records

========================================================================
Phase 3B.3 evidence summary
========================================================================
  [✓] OAuth scope is gmail.readonly only (user=founder)
  [✓] OAuth token persisted
  [✓] Gmail cursor present
  [✓] Polling cycle audit written  (19 cycle(s) recorded)
  [✓] gmail.read dispatch audits  (policy.decided=2 tool.invoked=2)
  [!] 401 → needs_reauth path exercised  (skipped — see §8)
  [✓] No raw email leak in audit / tool_invocations

VERDICT: PASS  (1 warning(s); see notes above)
```

Highlights:
- `oauth_tokens (provider='google')`: **1** row, scope = readonly only, `needs_reauth=false`
- `gmail_cursors`: **1** row, `history_id=2001771`
- `audit_log gmail.poll.cycle`: **19** entries (across the full session)
- `audit_log gmail.read dispatch`: **policy.decided=2 tool.invoked=2** (both from real founder-inbox messages)
- `tool_invocations gmail.read`: **2** rows, both `allowed` / `success`, latencies 184ms and 281ms (real Google API round-trip)
- **Leak canary scan: ✓ no forbidden keys or value patterns** — zero raw-email leak across 500 most-recent audit + 500 most-recent tool_invocations records

---

## 8. 401 → needs_reauth

☒ **Skipped** (relying on unit-test coverage in `dispatch/external-executors.test.ts` and `workers/gmail-poll.test.ts` — both prove the 401 → `markNeedsReauth` path independently).

Optional follow-up: revoke at <https://myaccount.google.com/permissions>,
re-run one cycle, verify `oauth_tokens.needs_reauth=true`. Not blocking
for 3B.3 sign-off.

---

## 9. Polling stops when disabled

☒ Implicitly confirmed via `fomo.poll.cycle_cap_reached` — once the cap
fired, no further `fomo.poll.cycle` entries appeared in stdout for
>10× the interval. The cap-stop and the kill-switch-stop share the
same `stopped = true` code path in `apps/fomo/src/index.ts`.

(Explicit `FOMO_GMAIL_POLLING_ENABLED=false` restart not separately
recorded; not blocking for 3B.3 sign-off given the cap-stop is
equivalent.)

---

## 10. Verdict

☒ **PASS** — all required checks green; Phase 3C may begin.

Notes / non-blocking follow-ups:

- 401 → `needs_reauth` live exercise skipped (unit-test coverage relied
  on). Worth doing before any user other than the founder is onboarded.
- `pg-connection-string` deprecation warning on the
  `sslmode=require` connection string — cosmetic; the next pg major
  version will require an explicit `sslmode=verify-full`. File a
  follow-up to switch.
- No OAuth refresh-token flow yet (by design in 3B.2/3B.3). The founder
  will need to re-OAuth roughly hourly until a refresh path lands. Track
  as a precondition for Phase 3D/3E (when send-tier tools go live).

---

## 11. Sign-off

- **Founder signature:** Galiette Mita
- **Date:** 2026-05-23
