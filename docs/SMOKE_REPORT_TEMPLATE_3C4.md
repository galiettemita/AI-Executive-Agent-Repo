# Phase 3C.4 Smoke Test Report — Rank-on-poll (real Gmail + real OpenAI)

> Fill this template after running every step in
> [`smoke-test-3c4-rank-on-poll.md`](smoke-test-3c4-rank-on-poll.md).
> Commit as `docs/SMOKE_REPORT_3C4.md` (drop the `_TEMPLATE_`) once
> `VERDICT: PASS`. Phase 3D Slack adapter does NOT start until this
> report lands on `main`.

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3c4-rank-on-poll-smoke-test`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Founder Gmail account used:** _<email>_
**OpenAI model used:** _<gpt-5-mini or other; record exactly what FOMO_OPENAI_MODEL resolved to>_

---

## 1. Prerequisites confirmed

- [ ] `docs/SMOKE_REPORT_3B3.md` is on `main` with `VERDICT: PASS`
- [ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` is on `main` with `VERDICT: PASS`
- [ ] Neon Postgres + Google OAuth + OpenAI billing already configured

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

Confirm each was set during the run. Do NOT paste secret values.

| Var                                | Set? | Notes                                  |
| ---------------------------------- | ---- | -------------------------------------- |
| `DATABASE_URL`                     | ☐    | Neon Postgres                          |
| `BREVIO_TOKEN_KEK`                 | ☐    |                                        |
| `BREVIO_OAUTH_STATE_KEY`           | ☐    |                                        |
| `BREVIO_SESSION_SIGNING_KEY`       | ☐    |                                        |
| `GOOGLE_CLIENT_ID`                 | ☐    |                                        |
| `GOOGLE_CLIENT_SECRET`             | ☐    |                                        |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` | ☐    | _<exact value>_                        |
| `FOMO_GMAIL_POLLING_ENABLED`       | ☐    | must be `true`                         |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ☐    | initial value: _<N>_                   |
| `FOMO_GMAIL_POLLING_INTERVAL_MS`   | ☐    | optional; _<ms>_                       |
| `FOMO_RANKER_ENABLED`              | ☐    | must be `true`                         |
| `OPENAI_API_KEY`                   | ☐    | same key as 3C.2                       |
| `FOMO_OPENAI_MODEL`                | ☐    | unset OR `gpt-5-mini`                  |
| `BREVIO_DEV_MODE`                  | ☐    | must be UNSET                          |
| `FOMO_SEND_ENABLED`                | ☐    | must be UNSET / false                  |
| `FOMO_AUTO_SEND_ENABLED`           | ☐    | must be UNSET / false                  |
| `FOMO_FRIEND_BETA_ENABLED`         | ☐    | must be UNSET / false                  |

---

## 3. Commands run

Paste the actual commands. Order matters.

```bash
# Preflight
pnpm --filter @brevio/fomo run preflight:3c4
# (output: ☐ pass / ☐ fail)

# Migrations
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0002_rank_results.sql

# Build + start (cycle 1)
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev

# (sent yourself N test emails, watched fomo.rank.completed log lines)

# Stopped server. Lowered cursor for idempotency exercise:
psql "$DATABASE_URL" -c "UPDATE gmail_cursors SET history_id = (history_id::bigint - 100)::text WHERE user_id = 'founder';"

# Restarted with raised cap (cycle 2):
FOMO_GMAIL_POLLING_MAX_CYCLES=6 pnpm --filter @brevio/fomo run dev

# (observed fomo.rank.already_ranked log lines)

# Evidence
pnpm --filter @brevio/fomo run smoke-evidence:3c4
```

---

## 4. Boot-time confirmation

Paste the three boot log lines from cycle 1's `pnpm dev` (verbatim):

```
fomo.ranker.enabled  ...
fomo.poll.enabled    ...
fomo.server.listening ...
```

`ranker_enabled: true` must appear in two of the three. If `ranker_enabled: false`
appears, the kill switch wasn't sourced into the shell — STOP and fix.

---

## 5. Polling-cycle evidence (server stdout)

Paste the relevant lines from server stdout across BOTH cycles.

### Cycle 1 (ranks happen)

```
fomo.poll.cycle  ... messages_ranked: N, messages_rank_already: 0, ...
fomo.rank.completed (×N) ...
fomo.poll.cycle_cap_reached ...
```

### Cycle 2 (idempotency)

```
fomo.poll.cycle  ... messages_ranked: 0, messages_rank_already: N, ...
fomo.rank.already_ranked (×N) ...
fomo.poll.cycle_cap_reached ...
```

---

## 6. `pnpm smoke-evidence:3c4` output (LOAD-BEARING)

Paste the full stdout verbatim:

```
…
```

The verdict line at the bottom must read `VERDICT: PASS` for this report to commit.

---

## 7. Founder eyeball (informational, not gate-criterion)

For each row that landed in `rank_results`, write one line: "reasonable"
or "surprising — <reason>". This is the founder's calibration signal
for whether to invest in label-quality tuning next; it does NOT gate.

| message_id (last 6) | label         | score | reason (truncated)                          | Founder note                  |
| ------------------- | ------------- | ----- | ------------------------------------------- | ----------------------------- |
| _<...>_             | important     |       |                                             | _<reasonable / surprising>_   |
| _<...>_             | not_important |       |                                             | _<reasonable / surprising>_   |

Overall calibration impression: _<one sentence — "the labels look
right" / "a few felt off, will tune in 3C.5" / etc.>_

---

## 8. Clean-stop confirmation

Paste the boot log from the final restart with `FOMO_RANKER_ENABLED=false`
and `FOMO_GMAIL_POLLING_ENABLED=false`:

```
fomo.ranker.disabled ...
fomo.poll.disabled   ...
fomo.server.listening ... ranker_enabled:false polling_enabled:false
```

- [ ] No `fomo.poll.cycle` log lines appeared during 2× the configured interval
- [ ] No `fomo.rank.completed` log lines appeared

---

## 9. Verdict

☐ **PASS** — every required check in §6 green, idempotency exercised, no leaks, clean stop confirmed. Phase 3D Slack adapter may begin.

☐ **FAIL** — list failures below; Phase 3D blocked until a re-run reaches PASS.

Failures / followups:

- _…_

---

## 10. Sign-off

- Founder signature: _<name>_
- Date: _<YYYY-MM-DD>_
