# Phase 3C.4 Smoke Test Report — Rank-on-poll (real Gmail + real OpenAI)

> Fill this template after running every step in
> [`smoke-test-3c4-rank-on-poll.md`](smoke-test-3c4-rank-on-poll.md).
> Commit as `docs/SMOKE_REPORT_3C4.md` (drop the `_TEMPLATE_`) once
> `VERDICT: PASS`. Phase 3D Slack adapter does NOT start until this
> report lands on `main`.

---

**Founder:** Galiette Mita
**Run date:** 2026-05-24 11:44 EST
**Branch:** `phase3c4-rank-on-poll-smoke-test` (smoke executed against `main` at `931f0135` after PR #27 merged)
**Commit SHA at run time:** `931f013595e5d30f6024b8782e44a15642c92322`
**Founder Gmail account used:** galiettemita@icloud.com
**OpenAI model used:** gpt-5-mini (the 3C.2-validated default)

---

## 1. Prerequisites confirmed

- [ ✓] `docs/SMOKE_REPORT_3B3.md` is on `main` with `VERDICT: PASS`
- [✓ ] `docs/OPENAI_SMOKE_REPORT_3C2.md` is on `main` with `VERDICT: PASS`
- [✓ ] Neon Postgres + Google OAuth + OpenAI billing already configured

If any unchecked → STOP, this gate is premature.

---

## 2. Env vars (redacted)

Confirm each was set during the run. Do NOT paste secret values.

| Var                                | Set? | Notes                                  |
| ---------------------------------- | ---- | -------------------------------------- |
| `DATABASE_URL`                     |  ✓   | Neon Postgres                          |
| `BREVIO_TOKEN_KEK`                 | ✓    |                                        |
| `BREVIO_OAUTH_STATE_KEY`           | ✓    |                                        |
| `BREVIO_SESSION_SIGNING_KEY`       | ✓    |                                        |
| `GOOGLE_CLIENT_ID`                 | ✓    |                                        |
| `GOOGLE_CLIENT_SECRET`             | ✓    |                                        |
| `BREVIO_OAUTH_REDIRECT_URI_GOOGLE` |✓    | http://localhost:8080/oauth/google/callback                    |
| `FOMO_GMAIL_POLLING_ENABLED`       |✓   | must be `true`                         |
| `FOMO_GMAIL_POLLING_MAX_CYCLES`    | ✓   | initial value: 3                  |
| `FOMO_GMAIL_POLLING_INTERVAL_MS`   |✓   | optional; 10000                      |
| `FOMO_RANKER_ENABLED`              |✓  | must be `true`                         |
| `OPENAI_API_KEY`                   | ✓    | same key as 3C.2                       |
| `FOMO_OPENAI_MODEL`                | ✓   | unset OR `gpt-5-mini`                  |
| `BREVIO_DEV_MODE`                  |✓   | must be UNSET                          |
| `FOMO_SEND_ENABLED`                |✓   | must be UNSET / false                  |
| `FOMO_AUTO_SEND_ENABLED`           |✓   | must be UNSET / false                  |
| `FOMO_FRIEND_BETA_ENABLED`         | ✓    | must be UNSET / false                  |

---

## 3. Commands run

```bash
# 1. Preflight (passed after .env.3b3.local was extended with FOMO_RANKER_ENABLED=true + OPENAI_API_KEY)
pnpm --filter @brevio/fomo run preflight:3c4

# 2. Re-auth Google OAuth (the 3B.3 token had expired overnight; needs_reauth had been set to true)
#    Mint a fresh session token + curl /oauth/google/start + complete the browser consent flow.

# 3. Build + start session 1 (cap=15)
pnpm --filter @brevio/fomo run build
FOMO_GMAIL_POLLING_MAX_CYCLES=15 pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3c4.log

# 4. Sent test emails from a non-founder Gmail account to the founder Gmail.
#    Observed fomo.poll.cycle with messages_ranked > 0 across cycles 1, 7, 8.
#    Session 1 hit cycle_cap_reached at cycle 15.

# 5. Rewound cursor to start-of-session value to force the idempotency seam:
psql "$DATABASE_URL" -c "UPDATE gmail_cursors SET history_id='2001671' WHERE user_id='founder';"

# 6. Killed any orphan node + restarted session 2 (cap=8)
lsof -ti :8080 | xargs -r kill -9
FOMO_GMAIL_POLLING_MAX_CYCLES=8 pnpm --filter @brevio/fomo run dev 2>&1 | tee -a /tmp/fomo-3c4.log

# 7. Session 2 cycle 1 observed 15 messages, ranked 10 NEW, audited 5 fomo.rank.already_ranked.
#    Ctrl-C'd the server.

# 8. Evidence
pnpm --filter @brevio/fomo run smoke-evidence:3c4
```

---

## 4. Boot-time confirmation

The three boot log lines from `pnpm dev` (cap=15 session that produced the initial ranks), from `/tmp/fomo-3c4.log`:

```
{"ts":"2026-05-25T04:08:50.755Z","service":"fomo","env":"development","event":"fomo.ranker.enabled","severity":"INFO","attrs":{"model":"gpt-5-mini","prompt_version_loaded":true}}
{"ts":"2026-05-25T04:08:50.757Z","service":"fomo","env":"development","event":"fomo.poll.enabled","severity":"INFO","attrs":{"interval_ms":10000,"cycle_cap":15,"ranker_enabled":true}}
{"ts":"2026-05-25T04:08:50.767Z","service":"fomo","env":"development","event":"fomo.server.listening","severity":"INFO","attrs":{"port":8080,"store_backend":"postgres","oauth_google_wired":true,"polling_enabled":true,"ranker_enabled":true}}
```

A second server session (cap=8) was started at `04:18:13` after the first hit `cycle_cap_reached` at cycle 15; that session ran the idempotency cycle. Its boot lines (same shape) confirm the substrate restarts cleanly:

```
{"ts":"2026-05-25T04:18:13.780Z","event":"fomo.ranker.enabled","attrs":{"model":"gpt-5-mini","prompt_version_loaded":true}}
{"ts":"2026-05-25T04:18:13.781Z","event":"fomo.poll.enabled","attrs":{"interval_ms":10000,"cycle_cap":8,"ranker_enabled":true}}
{"ts":"2026-05-25T04:18:13.791Z","event":"fomo.server.listening","attrs":{"port":8080,"store_backend":"postgres","oauth_google_wired":true,"polling_enabled":true,"ranker_enabled":true}}
```

✓ `ranker_enabled: true` appears in `fomo.poll.enabled` and `fomo.server.listening` in both sessions. Kill switch sourced correctly.

---

## 5. Polling-cycle evidence (server stdout)

### Cycle 1 (ranks happen — first real founder Gmail rank)

The first session (cap=15) ranked the initial 5 test emails across cycles 1, 7, and 8:

```
{"ts":"2026-05-25T04:09:09.873Z","event":"fomo.poll.cycle","attrs":{"cycle_number":1,"cycle_cap":15,"users_polled":1,"messages_observed":3,"messages_dispatched":3,"messages_ranked":3,"messages_rank_already":0,"messages_rank_failed":0}}
{"ts":"2026-05-25T04:10:17.175Z","event":"fomo.poll.cycle","attrs":{"cycle_number":7,"cycle_cap":15,"messages_observed":1,"messages_dispatched":1,"messages_ranked":1}}
{"ts":"2026-05-25T04:10:37.532Z","event":"fomo.poll.cycle","attrs":{"cycle_number":8,"cycle_cap":15,"messages_observed":1,"messages_dispatched":1,"messages_ranked":1}}
```

5 successful ranks. Cap-reached after cycle 15 (the worker correctly bounded itself):

```
{"ts":"2026-05-25T04:11:50.030Z","event":"fomo.poll.cycle_cap_reached","attrs":{"cycles_run":15,"cycle_cap":15}}
```

### Cycle 2 (idempotency proof — ON CONFLICT DO NOTHING fired against live Postgres)

After cap-reached, restarted with cap=8 and rewound `gmail_cursors.history_id` from current → `2001671` (start-of-session value). The next polling cycle (cycle_number=1 of the second session) re-observed 15 messages, of which 5 were already in `rank_results`:

```
{"ts":"2026-05-25T04:19:42.318Z","event":"fomo.poll.cycle","attrs":{"cycle_number":1,"cycle_cap":8,"users_polled":1,"messages_observed":15,"messages_dispatched":15,"messages_failed":0,"messages_ranked":10,"messages_rank_already":5,"messages_rank_failed":0}}
```

`messages_rank_already: 5` is the load-bearing line. The 5 duplicate dispatches each ran through the ranker, each `rank_results.write()` returned `{ inserted: false }`, and each audited as `fomo.rank.already_ranked`. The `(user_id, message_id)` unique constraint is firing as designed against live Neon Postgres.

---

## 6. `pnpm smoke-evidence:3c4` output (LOAD-BEARING)

Paste the full stdout verbatim:




> @brevio/fomo@0.1.0 smoke-evidence:3c4 /Users/galiettemita/Downloads/Executive AI Agent/backend/apps/fomo
> node --experimental-strip-types --loader ./test-loader.mjs scripts/smoke-evidence-3c4.ts

(node:73626) ExperimentalWarning: `--experimental-loader` may be removed in the future; instead use `register()`:
--import 'data:text/javascript,import { register } from "node:module"; import { pathToFileURL } from "node:url"; register("./test-loader.mjs", pathToFileURL("./"));'
(Use `node --trace-warnings ...` to show where the warning was created)
Phase 3C.4 evidence — querying Neon Postgres substrate

(node:73626) Warning: SECURITY WARNING: The SSL modes 'prefer', 'require', and 'verify-ca' are treated as aliases for 'verify-full'.
In the next major version (pg-connection-string v3.0.0 and pg v9.0.0), these modes will adopt standard libpq semantics, which have weaker security guarantees.

To prepare for this change:
- If you want the current behavior, explicitly use 'sslmode=verify-full'
- If you want libpq compatibility now, use 'uselibpqcompat=true&sslmode=require'

See https://www.postgresql.org/docs/current/libpq-ssl.html for libpq SSL mode definitions.
oauth_tokens (provider='google'): 1 row(s)
  user_id=founder scopes=["https://www.googleapis.com/auth/gmail.readonly"] needs_reauth=false

gmail_cursors: 1 row(s)
  user_id=founder history_id=2003576 updated_at=2026-05-25T04:20:29.076Z

audit_log action='gmail.poll.cycle': 20 entry(ies)
  id=129 at=2026-05-25T04:20:29.172Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=128 at=2026-05-25T04:20:18.772Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=127 at=2026-05-25T04:20:08.378Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":1,"users_api_error":0,"messages_observed":1,"users_unauthorized":0,"messages_dispatched":1,"messages_rank_failed":0,"messages_rank_already":0}
  id=123 at=2026-05-25T04:19:52.702Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=122 at=2026-05-25T04:19:42.354Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":10,"users_api_error":0,"messages_observed":15,"users_unauthorized":0,"messages_dispatched":15,"messages_rank_failed":0,"messages_rank_already":5}
  id=76 at=2026-05-25T04:11:50.060Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=75 at=2026-05-25T04:11:39.700Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=74 at=2026-05-25T04:11:29.340Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=73 at=2026-05-25T04:11:18.989Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=72 at=2026-05-25T04:11:08.629Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=71 at=2026-05-25T04:10:58.281Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=70 at=2026-05-25T04:10:47.933Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=69 at=2026-05-25T04:10:37.562Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":1,"users_api_error":0,"messages_observed":1,"users_unauthorized":0,"messages_dispatched":1,"messages_rank_failed":0,"messages_rank_already":0}
  id=65 at=2026-05-25T04:10:17.205Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":1,"users_api_error":0,"messages_observed":1,"users_unauthorized":0,"messages_dispatched":1,"messages_rank_failed":0,"messages_rank_already":0}
  id=61 at=2026-05-25T04:10:01.709Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=60 at=2026-05-25T04:09:51.349Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=59 at=2026-05-25T04:09:40.973Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=58 at=2026-05-25T04:09:30.617Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=57 at=2026-05-25T04:09:20.257Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":0,"users_api_error":0,"messages_observed":0,"users_unauthorized":0,"messages_dispatched":0,"messages_rank_failed":0,"messages_rank_already":0}
  id=56 at=2026-05-25T04:09:09.901Z result=success detail={"users_total":1,"users_polled":1,"users_skipped":0,"messages_failed":0,"messages_ranked":3,"users_api_error":0,"messages_observed":3,"users_unauthorized":0,"messages_dispatched":3,"messages_rank_failed":0,"messages_rank_already":0}

audit_log gmail.read dispatch (regression): policy.decided=23 tool.invoked=23

audit_log action='fomo.rank.completed': 16 entry(ies)
  id=126 at=2026-05-25T04:20:08.329Z detail={"label":"not_important","score":0.96,"latency_ms":4805,"message_id":"19e5d5c73b193ef4","model_name":"gpt-5-mini","input_tokens":389,"invocation_id":"gmail-poll-gm77b2-1","output_tokens":241,"prompt_version":"ranker-v0.1.0","estimated_cost_usd":0.00057925}
  id=106 at=2026-05-25T04:19:15.805Z detail={"label":"not_important","score":0.9,"latency_ms":5899,"message_id":"19e5cdcb4d5f755c","model_name":"gpt-5-mini","input_tokens":379,"invocation_id":"gmail-poll-2eykm8-10","output_tokens":370,"prompt_version":"ranker-v0.1.0","estimated_cost_usd":0.00083475}
  id=103 at=2026-05-25T04:19:09.441Z detail={"label":"not_important","score":0.95,"latency_ms":4668,"message_id":"19e5bae67f01053f","model_name":"gpt-5-mini","input_tokens":467,"invocation_id":"gmail-poll-2eykm8-9","output_tokens":240,"prompt_version":"ranker-v0.1.0","estimated_cost_usd":0.0005967500000000001}
  id=100 at=2026-05-25T04:19:04.321Z detail={"label":"not_important","score":0.95,"latency_ms":4856,"message_id":"19e5b6e5d151e8fd","model_name":"gpt-5-mini","input_tokens":508,"invocation_id":"gmail-poll-2eykm8-8","output_tokens":239,"prompt_version":"ranker-v0.1.0","estimated_cost_usd":0.0006050000000000001}
  id=97 at=2026-05-25T04:18:59.098Z detail={"label":"not_important","score":0.95,"latency_ms":3127,"message_id":"19e5aa02bd414fdc","model_name":"gpt-5-mini","input_tokens":388,"invocation_id":"gmail-poll-2eykm8-7","output_tokens":178,"prompt_version":"ranker-v0.1.0","estimated_cost_usd":0.00045299999999999995}

audit_log action='fomo.rank.already_ranked': 5 entry(ies)

audit_log action='fomo.rank.failed': 0 entry(ies)

tool_invocations tool_id='gmail.read': 23 row(s)
  id=23 invocation_id=gmail-poll-gm77b2-1 policy_decision=allowed status=success latency_ms=256
  id=22 invocation_id=gmail-poll-2eykm8-15 policy_decision=allowed status=success latency_ms=198
  id=21 invocation_id=gmail-poll-2eykm8-14 policy_decision=allowed status=success latency_ms=224
  id=20 invocation_id=gmail-poll-2eykm8-13 policy_decision=allowed status=success latency_ms=204
  id=19 invocation_id=gmail-poll-2eykm8-12 policy_decision=allowed status=success latency_ms=203

rank_results: 16 row(s)
  id=21 user=founder message=19e5d5c73b193ef4 label=not_important score=0.96 model=gpt-5-mini v=ranker-v0.1.0 latency=4805ms tokens=389/241 cost=$0.000579 reason="retail promotional marketing email advertising discounts; not time-sensitive or ..."
  id=15 user=founder message=19e5cdcb4d5f755c label=not_important score=0.9 model=gpt-5-mini v=ranker-v0.1.0 latency=5899ms tokens=379/370 cost=$0.000835 reason="Automated social-platform notification (non-urgent); likely a comment/digest, no..."
  id=14 user=founder message=19e5bae67f01053f label=not_important score=0.95 model=gpt-5-mini v=ranker-v0.1.0 latency=4668ms tokens=467/240 cost=$0.000597 reason="Promotional marketing sale email (time-limited discount), not a personal or urge..."
  id=13 user=founder message=19e5b6e5d151e8fd label=not_important score=0.95 model=gpt-5-mini v=ranker-v0.1.0 latency=4856ms tokens=508/239 cost=$0.000605 reason="Daily news/newsletter about politics; not a personal or time‑sensitive request"
  id=12 user=founder message=19e5aa02bd414fdc label=not_important score=0.95 model=gpt-5-mini v=ranker-v0.1.0 latency=3127ms tokens=388/178 cost=$0.000453 reason="Retail marketing promotional sale (Memorial Day) — promotional, not a personal o..."
  id=11 user=founder message=19e5a7b46dd39b79 label=not_important score=0.96 model=gpt-5-mini v=ranker-v0.1.0 latency=4785ms tokens=414/305 cost=$0.000713 reason="marketing retargeting promotion about a discounted cart item; not a personal or ..."
  id=10 user=founder message=19e5a11fbd0d11e5 label=not_important score=0.93 model=gpt-5-mini v=ranker-v0.1.0 latency=5312ms tokens=538/241 cost=$0.000616 reason="Promotional marketplace email about products (marketing), not a time-sensitive p..."
  id=9 user=founder message=19e5a0d3722eee5a label=not_important score=0.92 model=gpt-5-mini v=ranker-v0.1.0 latency=5539ms tokens=415/235 cost=$0.000574 reason="promotional ecommerce reward notification — marketing email, not time-sensitive"
  id=8 user=founder message=19e588e1da3636c5 label=not_important score=0.02 model=gpt-5-mini v=ranker-v0.1.0 latency=12239ms tokens=386/497 cost=$0.001091 reason="retailer marketing/transactional message about orders/promotions, not a time-sen..."
  id=7 user=founder message=19e58103ce33c4d2 label=not_important score=0.86 model=gpt-5-mini v=ranker-v0.1.0 latency=4391ms tokens=385/237 cost=$0.000570 reason="Short test message from an individual; no time-sensitive or actionable content."

Scanning for leak canaries in audit_log + tool_invocations + rank_results ...
  ✓ no forbidden keys or value patterns found in 500 most recent audit / tool_invocations records or any rank_results.reason

========================================================================
Phase 3C.4 evidence summary
========================================================================
  [✓] OAuth scope is gmail.readonly only (user=founder) — regression check
        scopes=["https://www.googleapis.com/auth/gmail.readonly"]; required=[https://www.googleapis.com/auth/gmail.readonly]
  [✓] Gmail cursor present
        1 cursor row(s); latest history_id=2003576
  [✓] Polling cycle audit written, ranker counters surfaced in detail
        20 cycle(s) recorded; sum messages_ranked across cycles=16
  [✓] gmail.read dispatch fired (regression)
        policy.decided=23 tool.invoked=23
  [✓] fomo.rank.completed audit written (≥1 required for 3C.4 PASS)
        16 successful rank(s) audited
  [✓] fomo.rank.already_ranked audit written (idempotency seam exercised against live Postgres)
        5 duplicate(s) audited — ON CONFLICT DO NOTHING confirmed firing
  [✓] fomo.rank.failed clean (no model errors in smoke window)
        0 ranker failures
  [✓] rank_results populated (≥1 row required for 3C.4 PASS)
        16 row(s); important=0, not_important=10. Founder eyeballs reasonableness in the report; not a gate criterion.
  [✓] No raw email leak in audit / tool_invocations / rank_results.reason
        Scanned 500 recent audit + 23 tool_invocations + 16 rank_results rows; zero hits.

VERDICT: PASS  (0 warning(s); see notes above)
Phase 3D Slack adapter is now unblocked.

The verdict line at the bottom must read `VERDICT: PASS` for this report to commit.

---

## 7. Founder eyeball (informational, not gate-criterion)

`rank_results` ended up with **16 rows total**. The evidence script's summary only iterates the 10 most-recent (display-only loop limit); the older 6 include 2 `important` labels from the initial "Smoke test 1: interview deadline" emails verified earlier in the session. Founder eyeballs below for the 10 shown in §6 + the 2 known earlier `important` rows.

| message_id (suffix) | label         | score | reason (truncated)                                            | Founder note      |
| ------------------- | ------------- | ----- | ------------------------------------------------------------- | ----------------- |
| ...8736759          | important     | 0.80  | Time-sensitive interview-related request (deadline tonight)   | reasonable        |
| ...7835259fd        | important     | 0.85  | Time-sensitive interview/form deadline tonight                | reasonable        |
| ...5c73b193ef4      | not_important | 0.96  | retail promotional marketing                                  | reasonable        |
| ...cdcb4d5f755c     | not_important | 0.90  | automated social-platform notification                        | reasonable        |
| ...bae67f01053f     | not_important | 0.95  | promotional time-limited sale                                 | reasonable        |
| ...b6e5d151e8fd     | not_important | 0.95  | daily news/newsletter (politics)                              | reasonable        |
| ...aa02bd414fdc     | not_important | 0.95  | retail Memorial Day promotional                               | reasonable        |
| ...a7b46dd39b79     | not_important | 0.96  | retargeting cart-abandonment promotion                        | reasonable        |
| ...a11fbd0d11e5     | not_important | 0.93  | marketplace product promotion                                 | reasonable        |
| ...a0d3722eee5a     | not_important | 0.92  | ecommerce reward notification                                 | reasonable        |
| ...88e1da3636c5     | not_important | 0.02  | retailer marketing/transactional                              | **surprising — score is 0.02; label correct but unusually low confidence; worth a look in 3C.5 tuning** |
| ...8103ce33c4d2     | not_important | 0.86  | short personal test message                                   | reasonable        |

**Overall calibration impression:** Labels look right. Two interview-deadline emails correctly surfaced as `important`. All commercial/promotional/newsletter emails correctly suppressed as `not_important`. One row (`...88e1da3636c5`) returned score=0.02 — same label as the others, just low confidence; a candidate for prompt-tuning attention in a later phase but not a 3C.4 concern.

---

## 8. Clean-stop confirmation

The bounded-window safety stop fired correctly during the smoke. From `/tmp/fomo-3c4.log`:

```
{"ts":"2026-05-25T04:11:50.030Z","event":"fomo.poll.cycle_cap_reached","severity":"INFO","attrs":{"cycles_run":15,"cycle_cap":15}}
```

After this event, the polling worker went dormant. The HTTP server continued listening on :8080 (verified via `lsof`), but no further `fomo.poll.cycle` audits were written — verified by the gap in `audit_log` between `04:11:50` (id=76, last cycle of session 1) and `04:19:42` (id=122, first cycle of session 2 after a fresh `pnpm dev`).

This proves the kill-switch substrate stops the worker cleanly without crashing the HTTP layer — same guarantee the runbook's optional "restart with `FOMO_RANKER_ENABLED=false` + `FOMO_GMAIL_POLLING_ENABLED=false`" step asks for. The default-off code path (no FOMO_* flags set → both kill switches off → `fomo.ranker.disabled` + `fomo.poll.disabled` log lines) is the path every `pnpm test` run already exercises in CI; it doesn't need a separate founder restart to validate.

- [x] Worker stopped cleanly when bounded window was reached (`fomo.poll.cycle_cap_reached` at 04:11:50)
- [x] No `fomo.poll.cycle` audit entries written between session 1 stop and session 2 start (verified via `audit_log` gap)
- [x] Default-off kill-switch paths covered by CI (`pnpm test` exercises every boot with both flags unset)

---

## 9. Verdict

**[✓] PASS** — every required check in §6 green:

- ✓ OAuth scope `gmail.readonly` only (regression)
- ✓ Gmail cursor present + advanced
- ✓ `audit_log gmail.poll.cycle`: 20 entries with ranker counters surfaced in detail
- ✓ `audit_log gmail.read` dispatch: policy.decided=23, tool.invoked=23
- ✓ `audit_log fomo.rank.completed`: 16 entries (≥ 1 required)
- ✓ `audit_log fomo.rank.already_ranked`: 5 entries (≥ 1 required — idempotency seam exercised against live Neon Postgres)
- ✓ `audit_log fomo.rank.failed`: 0 (no model errors)
- ✓ `rank_results`: 16 rows populated
- ✓ Leak-canary scan over 500 audit + 23 tool_invocations + 16 `rank_results.reason` → zero hits
- ✓ Bounded-window stop fired correctly (`cycle_cap_reached` at cycle 15)

**[ ] FAIL**

**Phase 3D Slack adapter is unblocked.**

### Bonus observations (not gate criteria, useful context)

- The 3B.3 401 → `needs_reauth` substrate fired naturally during this run when the founder's prior-day Gmail access token expired overnight. The worker correctly marked `needs_reauth=true` and refused to poll the dead token until OAuth was re-completed. That path was optional in 3B.3 and got proven for free here against live Google.
- Cycle latencies for `gpt-5-mini` ranks averaged ~4–5s per message (range 3.1s–12.2s across the 16 ranks). Cost averaged ~$0.0006 per rank; total spend across the smoke window ≈ $0.01.

---

## 10. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-05-25
