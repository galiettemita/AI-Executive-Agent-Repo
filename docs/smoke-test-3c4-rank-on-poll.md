# Phase 3C.4 — Founder Real Gmail + Real Ranker Smoke Test (Runbook)

> Founder-only smoke gate. Branch: `phase3c4-rank-on-poll-smoke-test`.
> Required deliverable: `docs/SMOKE_REPORT_3C4.md` with `VERDICT: PASS`
> committed to this branch before merge. **Phase 3D Slack adapter does
> not start until 3C.4 PASS is on `main`.**

This is the first time the full FOMO chain fires for real:

```
real Gmail → polling worker → gmail.read dispatch → real OpenAI ranker
→ rank_results row → audit
```

3B.3 proved real Gmail substrate alone. 3C.2 proved real OpenAI on 20
synthetic fixtures. 3C.4 proves the seam between them works against
real founder email content.

---

## 0. Prerequisites

You should already have these from prior gates. If anything is missing,
go back and finish that gate first.

- [ ] **3B.3 PASS report merged** to `main`: `docs/SMOKE_REPORT_3B3.md`
- [ ] **3C.2 PASS report merged** to `main`: `docs/OPENAI_SMOKE_REPORT_3C2.md`
- [ ] Neon Postgres project created and reachable from your laptop
- [ ] Google Cloud OAuth client + consent screen configured (3B.3)
- [ ] OpenAI API key with billing configured (3C.2)
- [ ] Working `apps/fomo/.env.3b3.local` from the 3B.3 run

> **Cost note.** This run makes real OpenAI calls. Bound the inbox
> activity to 1–3 test emails so the bounded `FOMO_GMAIL_POLLING_MAX_CYCLES`
> cap caps total spend. 3C.2 measured gpt-5-mini at well under 1¢ per
> classification, so 1–3 messages × 2 cycles ≈ a couple of pennies.

---

## 1. Env vars — extend `.env.3b3.local`

Open `apps/fomo/.env.3b3.local` (created during 3B.3) and add the
ranker-specific vars. The file is gitignored; values stay on your laptop.

```bash
# Required (already set during 3B.3):
DATABASE_URL=postgres://...                # Neon pooled connection string
BREVIO_TOKEN_KEK=...                       # 32 bytes base64
BREVIO_OAUTH_STATE_KEY=...                 # 32 bytes base64
BREVIO_SESSION_SIGNING_KEY=...             # 32 bytes base64
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
BREVIO_OAUTH_REDIRECT_URI_GOOGLE=http://localhost:8080/oauth/google/callback
FOMO_GMAIL_POLLING_ENABLED=true
FOMO_GMAIL_POLLING_MAX_CYCLES=3            # initial cap; raise for §6 idempotency exercise
# Optional:
FOMO_GMAIL_POLLING_INTERVAL_MS=10000       # tighter than the 60s default makes the smoke faster

# NEW in 3C.4 — required:
FOMO_RANKER_ENABLED=true                   # flips the ranker on (default off)
OPENAI_API_KEY=sk-...                      # same key the 3C.2 smoke eval used
# Optional override; leave UNSET to use gpt-5-mini (3C.2-validated):
# FOMO_OPENAI_MODEL=gpt-5-mini

# MUST stay UNSET or false (preflight will fail otherwise):
# FOMO_SEND_ENABLED=
# FOMO_AUTO_SEND_ENABLED=
# FOMO_FRIEND_BETA_ENABLED=
# BREVIO_DEV_MODE=
```

Source it into your shell:

```bash
set -a; source apps/fomo/.env.3b3.local; set +a
```

Sanity-check:

```bash
echo "ranker=$FOMO_RANKER_ENABLED  model=${FOMO_OPENAI_MODEL:-gpt-5-mini}"
echo "OPENAI_API_KEY length: ${#OPENAI_API_KEY}"
echo "DATABASE_URL=${DATABASE_URL:0:24}..."
```

---

## 2. Preflight

```bash
pnpm --filter @brevio/fomo run preflight:3c4
```

Expected output: a list of resolved env vars, the resolved kill-switch
view (`ranker_enabled: true`), the hardcoded Gmail scope, and a final
`✓ Preflight passed.` line. **If any `[ERROR]` line appears, fix the
listed var(s) and re-run before proceeding.**

---

## 3. Apply migrations (if not already)

The 3B.3 founder run applied `0000_init.sql` and `0001_gmail_cursors.sql`.
3C.3 added `0002_rank_results.sql`; apply it if you haven't already.

```bash
psql "$DATABASE_URL" -f apps/fomo/src/db/migrations/0002_rank_results.sql
psql "$DATABASE_URL" -P pager=off -c "\dt"
```

You should see **11 tables**, including `rank_results`. If you see only
10, the migration didn't apply.

---

## 4. Build + start the server

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo run dev
```

The boot log should show:

```json
{"event":"fomo.ranker.enabled", ... ,"attrs":{"model":"gpt-5-mini", ...}}
{"event":"fomo.poll.enabled", ... ,"attrs":{"interval_ms":10000,"cycle_cap":3,"ranker_enabled":true}}
{"event":"fomo.server.listening", ... ,"attrs":{"port":8080,"polling_enabled":true,"ranker_enabled":true}}
```

If you see `fomo.ranker.disabled` instead, `FOMO_RANKER_ENABLED` wasn't
exported into the shell that started the server. Re-source the env
file and restart.

---

## 5. Run the cycle 1 (rank live emails)

### 5a. Complete OAuth (if not already)

If your 3B.3 OAuth row is still present and `needs_reauth=false`, skip
to 5b. Otherwise, in a second terminal:

```bash
cd "$(pwd)/apps/fomo"
set -a; source ./.env.3b3.local; set +a

SESSION=$(node --experimental-strip-types --loader ./test-loader.mjs --input-type=module -e "
import { signSessionToken, loadSessionConfig } from './src/security/session.ts';
import { randomUUID } from 'node:crypto';
const cfg = loadSessionConfig();
console.log(signSessionToken(cfg, {
  user_id: 'founder',
  session_id: randomUUID(),
  expires_at: Math.floor(Date.now()/1000) + 3600
}));
")

curl -s -X POST http://localhost:8080/oauth/google/start \
  -H "authorization: Bearer $SESSION" -d '' | python3 -m json.tool
```

Open the `authorize_url` in your browser, confirm only the
`gmail.readonly` scope appears on the consent screen, click Allow.

### 5b. Send yourself 1–2 test emails

From any email account, send the founder Gmail:
1. One that looks **obviously important** (e.g., subject `"Reminder:
   deposit confirmation today"` with a short body about a real deadline).
2. One that looks **obviously promotional** (e.g., subject `"50% OFF
   FLASH SALE — TODAY ONLY"` with a marketing-style body).

Both must arrive **after** your `gmail_cursors.history_id` so they enter
the polling window. Wait 10–20 seconds for Gmail to deliver them.

### 5c. Watch cycles fire

The server should log within `interval_ms` of receipt:

```
{"event":"fomo.poll.cycle", ... "users_total":1, "users_polled":1,
 "messages_observed":2, "messages_dispatched":2,
 "messages_ranked":2, "messages_rank_already":0, "messages_rank_failed":0, ...}
{"event":"fomo.rank.completed", ... <one per ranked message>}
```

After the cap (`MAX_CYCLES=3`) the worker auto-stops:

```
{"event":"fomo.poll.cycle_cap_reached", "attrs":{"cycles_run":3,"cycle_cap":3}}
```

Ctrl-C the server (HTTP keeps listening even after the cap; safe to stop).

---

## 6. Second cycle — exercise idempotency (REQUIRED for 3C.4 PASS)

3C.3's `ON CONFLICT (user_id, message_id) DO NOTHING` semantics need to
fire against real Neon Postgres. To force this:

1. **Lower the cursor** so the same messages re-appear in the next
   history range:

   ```bash
   psql "$DATABASE_URL" -c \
     "UPDATE gmail_cursors SET history_id = (history_id::bigint - 100)::text
      WHERE user_id = 'founder';"
   ```

   (Gmail's `history_id` is a monotonic uint64; rewinding by 100
   reliably overlaps the messages you just sent — Gmail returns a
   superset of new IDs, and the polling worker will hand the SAME
   `message_id`s to the dispatch+ranker path again.)

2. **Raise the cap** so the worker can run a fresh window and restart:

   ```bash
   FOMO_GMAIL_POLLING_MAX_CYCLES=6 pnpm --filter @brevio/fomo run dev
   ```

3. Within 10–20 seconds you should see:

   ```
   {"event":"fomo.poll.cycle", ... "messages_ranked":0, "messages_rank_already":2, ...}
   {"event":"fomo.rank.already_ranked", ... <one per re-seen message>}
   ```

   `messages_rank_already > 0` is the proof the idempotency seam fires
   against live Postgres. (The original `rank_results` rows are
   unchanged; no new OpenAI call was made for the duplicates.)

4. Ctrl-C the server.

> If the cursor rewind doesn't surface the same messages (e.g., Gmail's
> history ring is shorter than 100), send yourself **one more** test
> email and re-run cycle 1 — then immediately re-run this section
> without sending anything new. The second cycle will re-rank the
> message already in `rank_results`.

---

## 7. Evidence

```bash
pnpm --filter @brevio/fomo run smoke-evidence:3c4
```

Reads from the live Neon DB. Prints per-check PASS/FAIL/WARN plus a final
verdict. Exits 1 on any FAIL.

Required-PASS checks (the gate criteria):

- OAuth scope is `gmail.readonly` only (regression)
- Gmail cursor present
- `audit_log gmail.poll.cycle` written with ranker counters in detail
- `audit_log gmail.read` dispatch fired
- `audit_log fomo.rank.completed` ≥ 1
- `audit_log fomo.rank.already_ranked` ≥ 1 *(from §6)*
- `rank_results` ≥ 1 row
- **Leak-canary scan over `audit_log` + `tool_invocations` +
  `rank_results.reason` → zero hits**

Capture the full stdout — you'll paste it into the report.

---

## 8. Stop and clean up

```bash
# (Ctrl-C the server if still running.)
# Restart WITHOUT the ranker to confirm clean shutdown of the substrate:
FOMO_RANKER_ENABLED=false FOMO_GMAIL_POLLING_ENABLED=false pnpm --filter @brevio/fomo run dev
```

You should see:

```
{"event":"fomo.ranker.disabled", ...}
{"event":"fomo.poll.disabled", ...}
{"event":"fomo.server.listening", ... "polling_enabled":false, "ranker_enabled":false}
```

Ctrl-C. (This confirms the kill switches default-off behavior — no
orphan ranker calls, no orphan polling.)

---

## 9. Report

1. Copy [`docs/SMOKE_REPORT_TEMPLATE_3C4.md`](SMOKE_REPORT_TEMPLATE_3C4.md)
   to `docs/SMOKE_REPORT_3C4.md`.
2. Fill in every section. **The evidence-script stdout is the
   load-bearing artifact — paste it verbatim in §6.**
3. If `VERDICT: PASS`: commit to this branch, push, merge the PR.
4. If `VERDICT: FAIL`: do **not** merge. The PASS-gate principle: every
   required check must be green before 3D may begin. Log the failure
   details in the report, ping the founder.

---

## What "PASS" means in 3C.4

Per the founder-confirmed gate definition:

> **Seam-works only.** PASS means the integration produced the artifacts
> the next phase needs: rank_results rows exist, audit events fire with
> correct fields, no leaks, no crashes, idempotency holds against live
> Postgres, OAuth scope is still readonly. PASS does **not** require any
> specific label-quality threshold. Founder writes a one-line "looked
> reasonable / surprising" judgment per row in the report; that
> judgment is informational and does not gate.

Label-quality tuning is a separate phase. 3C.4 proves the *seam works*.
