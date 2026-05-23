# Phase 3B.3 Smoke Test Report — Gmail Real

> Fill this template after running the steps in [smoke-test-3b3-gmail.md](smoke-test-3b3-gmail.md).
> Commit as `docs/SMOKE_REPORT_3B3.md` (drop the `_TEMPLATE_` part) once
> `VERDICT: PASS`.

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3b3-gmail-real-smoke-test`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Founder Gmail account used:** _<email>_

---

## 1. Google Cloud setup

- **Project name:** _<e.g., brevio-fomo-smoke>_
- **Gmail API enabled:** ☐ yes / ☐ no
- **OAuth consent screen — user type:** ☐ External / ☐ Internal
- **Scopes added to consent screen:** _<paste the list verbatim>_
  - Phase 3B.3 invariant: must be exactly
    `https://www.googleapis.com/auth/gmail.readonly` and nothing else.
- **Test users added:** _<list>_
- **OAuth client type:** ☐ Web application
- **Redirect URI configured:** _<exact value>_
- **Consent-screen screenshot:** _<link or attach>_

---

## 2. Env vars (redacted)

Confirm each was set. Do NOT paste secret values; just mark present.

| Var                                   | Set? | Notes                                   |
| ------------------------------------- | ---- | --------------------------------------- |
| `DATABASE_URL`                        | ☐    | Neon Postgres                           |
| `BREVIO_TOKEN_KEK`                    | ☐    | 32 bytes                                |
| `BREVIO_OAUTH_STATE_KEY`              | ☐    | 32 bytes                                |
| `BREVIO_SESSION_SIGNING_KEY`          | ☐    | 32 bytes                                |
| `GOOGLE_CLIENT_ID`                    | ☐    |                                         |
| `GOOGLE_CLIENT_SECRET`                | ☐    |                                         |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE`    | ☐    | _<value>_                               |
| `FOMO_GMAIL_POLLING_ENABLED`          | ☐    | must be `true`                          |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`       | ☐    | value: _<N>_                            |
| `FOMO_GMAIL_POLLING_INTERVAL_MS`      | ☐    | optional; value: _<ms>_                 |
| `BREVIO_DEV_MODE`                     | ☐    | should be UNSET                         |
| `FOMO_SEND_ENABLED`                   | ☐    | must be UNSET or false                  |
| `FOMO_AUTO_SEND_ENABLED`              | ☐    | must be UNSET or false                  |
| `FOMO_FRIEND_BETA_ENABLED`            | ☐    | must be UNSET or false                  |

---

## 3. Commands run

Paste the actual commands. Order matters.

```bash
# Migrations
…

# Preflight
pnpm --filter @brevio/fomo run smoke:preflight
# (preflight output: ☐ pass / ☐ fail; if fail, do not proceed)

# Build + start
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev

# OAuth start (in another terminal)
SESSION='…'
curl -s -X POST http://localhost:8080/oauth/google/start \
  -H "authorization: Bearer $SESSION" -d ''

# (open authorize_url in browser, complete consent, returned to callback)

# (wait for FOMO_GMAIL_POLLING_MAX_CYCLES cycles)

# Evidence
pnpm --filter @brevio/fomo run smoke:evidence
```

---

## 4. Scope observed

Paste the value of `oauth_tokens.scopes` for the founder's Google row:

```
SELECT scopes FROM oauth_tokens WHERE provider='google';
```

Result:

```json
[ "…" ]
```

Phase 3B.3 invariant: must be exactly
`["https://www.googleapis.com/auth/gmail.readonly"]`. Any other scope
is a FAIL.

---

## 5. Polling cycle evidence

Paste the relevant lines from server stdout. Should include:

- `fomo.poll.enabled` (one)
- `fomo.poll.cycle` (N entries; N = `FOMO_GMAIL_POLLING_MAX_CYCLES`)
- `fomo.poll.cycle_cap_reached` (one)
- `fomo.poll.disabled` (after restart with flag off)

```
…
```

---

## 6. Cursor advance

```bash
# Before any polling:
SELECT user_id, history_id, updated_at FROM gmail_cursors;

# After the smoke window:
SELECT user_id, history_id, updated_at FROM gmail_cursors;
```

| Time      | user_id | history_id | updated_at |
| --------- | ------- | ---------- | ---------- |
| At OAuth  |         |            |            |
| After N cycles |         |            |            |

`updated_at` should have moved forward.

---

## 7. Audit + tool_invocations evidence

Paste the `pnpm smoke:evidence` stdout:

```
…
```

Key lines to highlight:

- `oauth_tokens (provider='google'): N row(s)`
- `gmail_cursors: N row(s)`
- `audit_log action='gmail.poll.cycle': N entry(ies)`
- `audit_log gmail.read dispatch: policy.decided=X tool.invoked=X`
- `tool_invocations tool_id='gmail.read': N row(s)`
- `Scanning for leak canaries ...` → `✓ no forbidden keys` or `✖ N hit(s)`

---

## 8. 401 → needs_reauth

☐ Exercised  /  ☐ Skipped (relying on unit-test coverage)

If exercised:

- Revoked at: _<URL or timestamp>_
- Next cycle outcome: _<paste fomo.poll.cycle log line>_
- `oauth_tokens.needs_reauth` after revoke: ☐ true (PASS) / ☐ false (FAIL)

---

## 9. Polling stops when disabled

After cycle cap or Ctrl-C, restart with `FOMO_GMAIL_POLLING_ENABLED=false`
(or unset):

- Server log line shown: _<paste — should be `fomo.poll.disabled`>_
- No further `fomo.poll.cycle` entries in stdout for at least 2× the
  interval: ☐ confirmed

---

## 10. Verdict

☐ **PASS** — all required checks green; Phase 3C may begin
☐ **FAIL** — list failures below; do not start Phase 3C

Failures / followups:

- _…_

---

## 11. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
