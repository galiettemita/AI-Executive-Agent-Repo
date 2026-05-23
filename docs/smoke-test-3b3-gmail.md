# Phase 3B.3 — Gmail Real Smoke Test (Runbook)

> **Goal.** Prove the Gmail OAuth + polling path works end-to-end against
> **one real founder Gmail account** with `gmail.readonly` scope only.
> Persistence is real Neon Postgres. Polling is bounded to N cycles by
> `FOMO_GMAIL_POLLING_MAX_CYCLES` so the worker auto-stops.
>
> This runbook is the only artifact that proves the substrate Phase 3B.1
> and 3B.2 shipped actually works against the real Google. After it
> passes, Phase 3C may begin.

---

## 0. Non-goals (what this PR/run does **not** do)

- ❌ no ranker
- ❌ no real model calls
- ❌ no Slack
- ❌ no SendBlue
- ❌ no friend beta
- ❌ no auto-send
- ❌ no Gmail send/modify/delete scopes
- ❌ no broad Gmail scopes (`gmail.modify`, `gmail.metadata`, etc.)
- ❌ only ONE founder Gmail account

Anything not on the green-path checklist below is out of scope.

---

## 1. Google Cloud Console setup (one-time)

Required so the OAuth handshake can complete.

1. Open <https://console.cloud.google.com/> and create (or pick) a project.
   Name it `brevio-fomo-smoke` or similar.
2. **Enable the Gmail API** for that project:
   `APIs & Services → Library → search "Gmail API" → Enable`.
3. **Configure the OAuth consent screen**:
   - `APIs & Services → OAuth consent screen`
   - User type: **External** (so a personal Gmail can grant), then **Create**.
   - App name: `Brevio FOMO (founder smoke test)`
   - User support email: the founder's email
   - **Scopes (this is the critical step):** click `Add or remove scopes`,
     search for `gmail.readonly`, check ONLY
     `https://www.googleapis.com/auth/gmail.readonly`. Do NOT add
     `gmail.metadata`, `gmail.modify`, `gmail.send`, `gmail.compose`,
     `auth.userinfo.*`, or anything else.
   - **Test users:** add the founder's Gmail address (otherwise OAuth
     denies "access blocked" because the app is not verified).
4. **Create the OAuth 2.0 Client ID**:
   - `APIs & Services → Credentials → Create Credentials → OAuth client ID`
   - Application type: **Web application**
   - Authorized redirect URIs: add the value you will set as
     `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`. For a local run this is
     `http://localhost:8080/oauth/google/callback`. Must be **exact** —
     trailing slash matters.
   - Copy the Client ID and Client Secret. You'll set them as env vars.

> **Verification.** On the consent screen page, the only Gmail scope
> listed under "Scopes for Google APIs" should be `.../gmail.readonly`.
> Screenshot this for the report.

---

## 2. Provision Neon Postgres

We are NOT running this smoke test against in-memory stores. We want to
prove the real persistence path (encrypted token storage + cursor
persistence + audit log + tool invocations).

1. Create a free Neon project at <https://console.neon.tech/>.
2. Copy the **pooled** connection string (it looks like
   `postgres://USER:PASS@HOST/DB?sslmode=require`). Set it as
   `DATABASE_URL`.
3. Apply the Drizzle migrations to the new Neon DB. From the repo root:

   ```bash
   # Either run the SQL files directly via psql:
   for f in apps/fomo/src/db/migrations/*.sql; do
     psql "$DATABASE_URL" -f "$f"
   done

   # Or use Drizzle's apply command if you've configured it locally.
   ```

   Verify tables exist:

   ```bash
   psql "$DATABASE_URL" -c "\dt"
   ```

   You should see: `audit_log`, `oauth_tokens`, `gmail_cursors`,
   `tool_invocations`, `feedback_events`, `memory_signals`,
   `alert_state_transitions`, `cost_records`, `consent`, `users`.

---

## 3. Generate keys

Three 32-byte keys are required. Generate locally:

```bash
# 32-byte base64 keys (one each)
node -e "console.log(require('crypto').randomBytes(32).toString('base64'))"
```

Run the command three times to get three distinct values for:

- `BREVIO_TOKEN_KEK` — encrypts OAuth tokens at rest
- `BREVIO_OAUTH_STATE_KEY` — HMAC for OAuth `state` parameter
- `BREVIO_SESSION_SIGNING_KEY` — HMAC for session tokens

> **Do not** commit these. Store them in a local `.env` (gitignored) or
> the host's secret manager.

---

## 4. Set the smoke-test env

Create `apps/fomo/.env.3b3` (gitignored) or set in your shell:

```bash
# Persistence
export DATABASE_URL='postgres://...?sslmode=require'

# Crypto
export BREVIO_TOKEN_KEK='...base64-32-bytes...'
export BREVIO_OAUTH_STATE_KEY='...base64-32-bytes...'
export BREVIO_SESSION_SIGNING_KEY='...base64-32-bytes...'

# Google OAuth
export GOOGLE_CLIENT_ID='...apps.googleusercontent.com'
export GOOGLE_CLIENT_SECRET='...'
export BREVIO_OAUTH_REDIRECT_URI_GOOGLE='http://localhost:8080/oauth/google/callback'

# Phase 3B.3: polling explicitly enabled, bounded to N cycles
export FOMO_GMAIL_POLLING_ENABLED='true'
export FOMO_GMAIL_POLLING_MAX_CYCLES='3'

# Optional: tighter interval for the smoke test so 3 cycles take 30s
# instead of 3 minutes. Default is 60_000.
export FOMO_GMAIL_POLLING_INTERVAL_MS='10000'

# Leave BREVIO_DEV_MODE UNSET so the production fail-closed checks fire.
unset BREVIO_DEV_MODE
```

> **Forbidden during the smoke test.** `FOMO_SEND_ENABLED`,
> `FOMO_AUTO_SEND_ENABLED`, `FOMO_FRIEND_BETA_ENABLED` must all be unset
> or `false`. The preflight script fails if any is on.

---

## 5. Preflight

```bash
pnpm --filter @brevio/fomo run smoke:preflight
```

Expected output: a kill-switch dump and `✓ Preflight passed.`

If anything fails, fix the env vars and re-run. Do NOT proceed until
preflight is green.

---

## 6. Build + start the server

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev
```

Watch the stdout. You should see structured JSON logs:

- `fomo.poll.enabled` with `interval_ms` and `cycle_cap`
- `fomo.server.listening` with `port`, `store_backend: "postgres"`,
  `oauth_google_wired: true`, `polling_enabled: true`

The polling worker fires its first cycle immediately — but with no
OAuth token yet, the cycle reports `users_total: 0`.

---

## 7. Connect founder Gmail through OAuth

`/oauth/google/start` is **session-authenticated**. For the smoke test
we don't have a session UI yet. The simplest path:

1. **Mint a founder session token** using a small helper. From a Node
   REPL in `apps/fomo/`:

   ```bash
   node --experimental-strip-types --loader ./test-loader.mjs --input-type=module -e "
   import { signSessionToken, loadSessionConfig } from './src/security/session.ts';
   const cfg = loadSessionConfig();
   const token = signSessionToken(cfg, { user_id: 'founder', role: 'founder' });
   console.log(token);
   "
   ```

   Copy the resulting JWT-like token.

2. **POST to `/oauth/google/start` with the session token as a cookie or
   Bearer header.** Example:

   ```bash
   SESSION='...token from step 1...'
   curl -s -X POST http://localhost:8080/oauth/google/start \
     -H "authorization: Bearer $SESSION" \
     -d '' | tee /tmp/oauth-start.json
   ```

   The response contains `authorize_url`. Open that URL in the founder's
   browser.

3. In the browser:
   - Sign in as the founder Gmail.
   - On the consent screen, **verify only "View your email messages and
     settings (gmail.readonly)" is requested**. Screenshot this.
   - Click Allow.

4. Google redirects to `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` with `code`
   and `state` query params. The server's `/oauth/google/callback`
   handles the exchange and writes the token + cursor.

   Watch the server stdout for the callback log. Watch Postgres:

   ```bash
   psql "$DATABASE_URL" -c "SELECT user_id, provider, scopes, needs_reauth FROM oauth_tokens;"
   psql "$DATABASE_URL" -c "SELECT user_id, history_id FROM gmail_cursors;"
   ```

   Both rows should exist.

---

## 8. Watch the polling cycles fire

With the token + cursor in place, the next polling tick (within
`FOMO_GMAIL_POLLING_INTERVAL_MS` of step 7) will:

- Call Gmail's `users.me.history` against the cursor's `history_id`.
- Dispatch `gmail.read` for any new message IDs Gmail returns.
- Advance the cursor.

Each cycle emits a `fomo.poll.cycle` log line with counts. After
`FOMO_GMAIL_POLLING_MAX_CYCLES` cycles, the worker logs
`fomo.poll.cycle_cap_reached` and stops. No more Gmail traffic from
the worker.

> **Tip.** Send yourself a test email between cycles to ensure at least
> one `gmail.read` dispatch fires. Without a new message, `messages_observed`
> stays 0 — still a valid pass, but lighter evidence.

---

## 9. Verify the 401 → needs_reauth path

This is the only smoke-test step that requires deliberate breakage.
Skip if you're comfortable trusting the unit-test coverage.

1. Open <https://myaccount.google.com/permissions> in the founder's
   browser.
2. Find "Brevio FOMO (founder smoke test)" and revoke its access.
3. Trigger another polling cycle:
   - Set `FOMO_GMAIL_POLLING_MAX_CYCLES=4` and restart, OR
   - Wait for the existing interval if cycles remain.
4. On the next cycle, the worker hits 401, marks `needs_reauth=true`.
   Verify:

   ```bash
   psql "$DATABASE_URL" -c "SELECT user_id, provider, needs_reauth FROM oauth_tokens;"
   ```

   `needs_reauth` should be `true`.

---

## 10. Stop polling

Two ways:

- **Implicit (preferred):** the cycle cap auto-stops. The process keeps
  serving HTTP but the polling worker no longer fires.
- **Explicit:** `Ctrl-C` the process. The shutdown handler awaits any
  in-flight cycle before closing the HTTP server and Pg pool.

Restart with `FOMO_GMAIL_POLLING_ENABLED=false` (or unset) to confirm
the worker does NOT install its interval. Server stdout should show
`fomo.poll.disabled`.

---

## 11. Run the evidence script

```bash
pnpm --filter @brevio/fomo run smoke:evidence
```

The script reads:

- `oauth_tokens` — exactly one Google row, scope = `gmail.readonly` only.
- `gmail_cursors` — one row with a non-zero `history_id`.
- `audit_log` — `gmail.poll.cycle` entries (one per cycle the worker
  ran), `policy.decided` + `tool.invoked` pairs for any `gmail.read`
  dispatches.
- `tool_invocations` — rows with `tool_id='gmail.read'`.
- Scans the 500 most recent audit + tool_invocations rows for forbidden
  shapes (`body_plain`, `body_html`, `headers`, `attachments`, raw
  header dumps, long base64 blobs).

Exit 0 + `VERDICT: PASS` means the substrate held. Exit 1 + `VERDICT:
FAIL` means something leaked or didn't fire — read the per-finding
detail.

Copy the script's stdout into [SMOKE_REPORT_TEMPLATE_3B3.md](SMOKE_REPORT_TEMPLATE_3B3.md).

---

## 12. Report

Fill out [SMOKE_REPORT_TEMPLATE_3B3.md](SMOKE_REPORT_TEMPLATE_3B3.md)
and commit it to the branch as `docs/SMOKE_REPORT_3B3.md` (drop the
`_TEMPLATE_` part). That report is the founder's sign-off artifact for
the merge.

After the report is committed and verdict = PASS, Phase 3B closes here
and Phase 3C may begin.

---

## Appendix: what each check proves

| Required check                              | Evidence source                                                          |
| ------------------------------------------- | ------------------------------------------------------------------------ |
| Google OAuth env vars configured            | preflight passes                                                         |
| Founder connects Gmail through OAuth        | `oauth_tokens` row exists                                                |
| OAuth requests only `gmail.readonly`        | consent-screen screenshot + `oauth_tokens.scopes` JSON                   |
| `FOMO_GMAIL_POLLING_ENABLED=true` only here | preflight resolves it; server logs `fomo.poll.enabled`                   |
| Polling runs once or for a short window     | `fomo.poll.cycle_cap_reached` log + `gmail.poll.cycle` audit rows count  |
| Real Gmail message metadata/read exercised  | `policy.decided` + `tool.invoked` for `gmail.read` in audit; or `messages_observed > 0` in cycle audit |
| Cursor advances                             | `gmail_cursors.history_id` changes between cycles (re-query if needed)   |
| Audit + tool invocation records write       | counts in evidence script output                                         |
| No raw body/html/header/attachment leak     | evidence script's "No raw email leak" finding = PASS                     |
| 401 → `needs_reauth` marked                 | `oauth_tokens.needs_reauth = true` after step 9                          |
| Polling stops when disabled                 | restart with flag off; server logs `fomo.poll.disabled`                  |
