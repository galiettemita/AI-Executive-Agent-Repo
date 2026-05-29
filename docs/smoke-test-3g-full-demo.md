# Phase 3G — Full Founder Demo Smoke Test (v0.1 milestone)

> The v0.1 milestone. ONE email flows end-to-end through every piece
> of the substrate against real providers — Gmail → ranker → Slack
> approval → SendBlue outbound iMessage → SendBlue inbound reply →
> state transitions + feedback + memory writes — and the founder
> commits `docs/SMOKE_REPORT_3G.md` with `VERDICT: PASS`. v0.5 friend
> beta is gated behind this report landing on `main`.

**Scope (locked 2026-05-28 via 6-question pre-phase gate):**
- ONE happy-path scenario, no multi-scenario gauntlet
- Founder reply IS in scope (proves the inbound loop on a natural alert)
- Neon migration automation is OUT of scope (deferred to its own small phase)
- Hard boundaries (founder directive, reaffirmed): no friend beta, no auto-send, no group chats, no proactive follow-up messages, no snooze resurface scheduler, no new tools

---

## 0. Prerequisites

Hard prerequisites — verify each on `main` before this run:

- [ ] `docs/SMOKE_REPORT_3F2.md` on `main` with `VERDICT: PASS` (PR #34 merged)
- [ ] All earlier gates' PASS reports on `main`: 3B.3, 3C.2, 3C.4, 3D.2, 3E.2
- [ ] SendBlue dashboard webhook still configured (or you'll re-configure in §3)
- [ ] You're on a branch named `phase3g-full-demo-smoke` (or similar) and the scaffolding has been pushed

**Verify every migration is applied to Neon** (3F.2 footgun: PGlite-applied != Neon-applied):

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

psql "$DATABASE_URL" -P pager=off -c "
  SELECT
    to_regclass('public.oauth_tokens')         AS oauth_tokens,
    to_regclass('public.gmail_cursors')        AS gmail_cursors,
    to_regclass('public.rank_results')         AS rank_results,
    to_regclass('public.alerts')               AS alerts,
    to_regclass('public.inbound_replies')      AS inbound_replies,
    to_regclass('public.audit_log')            AS audit_log,
    to_regclass('public.feedback_events')      AS feedback_events,
    to_regclass('public.memory_signals')       AS memory_signals,
    to_regclass('public.alert_state_transitions') AS alert_state_transitions,
    to_regclass('public.tool_invocations')     AS tool_invocations
  ;"
```

Every column must print a table name, not `null`. If any are null, apply the
corresponding migration from `apps/fomo/src/db/migrations/` before proceeding.

---

## 1. Re-auth Gmail (CRITICAL — 3F.2 left needs_reauth=true)

The 3F.2 smoke run left `oauth_tokens.needs_reauth = true` for `founder`, so
the polling worker has been skipping every cycle. The chain CANNOT fire until
you complete a fresh OAuth flow.

**Confirm the current state:**

```bash
psql "$DATABASE_URL" -P pager=off -c \
  "SELECT user_id, needs_reauth, obtained_at FROM oauth_tokens WHERE user_id='founder';"
```

If `needs_reauth = t`, do the OAuth flow:

1. Start the dev server briefly to expose the OAuth start route:
   ```bash
   pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g-oauth.log
   ```
   Wait until you see `fomo.server.listening`.

2. In a browser, visit:
   ```
   http://localhost:8080/oauth/google/start?user_id=founder
   ```
   Complete the Google consent screen. You'll land back on the callback URL.

3. **Verify the token landed:**
   ```bash
   psql "$DATABASE_URL" -P pager=off -c \
     "SELECT user_id, needs_reauth, obtained_at FROM oauth_tokens WHERE user_id='founder';"
   ```
   Expect `needs_reauth = f` and a fresh `obtained_at`. Stop the dev server
   (`lsof -ti :8080 | xargs kill -9`).

If the OAuth flow fails or the redirect URI doesn't match, fix the
`BREVIO_OAUTH_REDIRECT_URI_GOOGLE` env var to match the OAuth client config
in Google Cloud Console.

---

## 2. Env additions

Same env as 3F.2 (none new). Confirm via preflight:

```bash
pnpm --filter @brevio/fomo run preflight:3g
```

Must exit 0. Address any errors; warnings (e.g. `FOMO_GMAIL_POLLING_MAX_CYCLES`
not set) are acceptable but recommend setting to `60` for ~10-minute window.

---

## 3. ngrok + SendBlue webhook URL

3F.2 left a tunnel running, but ngrok-free subdomains are dynamic — verify or
restart:

```bash
ngrok http 8080
```

Note the `https://<subdomain>.ngrok-free.dev` URL. In the SendBlue dashboard
(Webhooks page), confirm the configured URL is:

```
https://<subdomain>.ngrok-free.dev/sendblue/inbound
```

If it's stale, update it. (The dashboard secret should already be the same as
`SENDBLUE_WEBHOOK_SECRET`. If you reset it in 3F.2 cleanup, restore the value.)

---

## 4. Send yourself the FOMO-worthy test email

After the dev server starts in §5 you'll need a fresh email for the ranker to
score as `important`. Draft one now (don't send yet — wait until §5):

**Recommended subject:** `Re: investor intro — can you meet Thursday?`

**Recommended body (one short paragraph):**
> Hey — Eli (from the seed round) just intro'd me to a partner at Greenoaks who
> wants to chat about your seed. He's only free Thursday 3-5pm PT this week. Are
> you around? If not I'll push to next week but he flagged it as time-sensitive.

This shape (named investor + named partner + time-sensitive + Thursday) reliably
lands `label=important score≥0.85` against the v0.1 ranker prompt (covered by
ranker fixtures `investor_intro_time_sensitive` + similar).

**Do NOT send yet** — start the dev server first so the polling cycle picks up
the email within the bounded window.

---

## 5. Start dev server + send the email + watch the chain

Build first (catches any TS errors before the smoke window starts):

```bash
pnpm --filter @brevio/fomo run build
```

Then start the server with bounded cycle caps so it auto-stops:

```bash
FOMO_GMAIL_POLLING_MAX_CYCLES=60 \
FOMO_OUTBOUND_MAX_CYCLES=60 \
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g.log
```

Wait for the boot lines:
- `fomo.gmail.polling.enabled` (with `interval_ms` + cycle cap)
- `fomo.send.enabled` (with founder phone redacted)
- `fomo.sendblue.inbound.enabled` (with `webhook_secret_header: sb-signing-secret`)
- `fomo.slack.review.enabled`
- `fomo.server.listening`

Now **send the test email** from your founder Gmail to yourself. Within ~10s
the polling worker should pick it up. Expect this sequence in `/tmp/fomo-3g.log`:

```
fomo.poll.cycle ... messages_observed: ≥1, messages_dispatched: ≥1
fomo.rank.completed ... label: "important", score: ≥0.7
alert.created ... alert_id: <UUID>
fomo.slack.posted ... slack_ts: <ts>
```

**Slack:** open the founder channel. You should see the FOMO review card with
the alert summary + Approve / Reject buttons. Click **Approve**.

```
fomo.slack.approval_captured ... alert_id: <UUID>
state.transitioned ... <UUID>: queued_for_review → approved
fomo.outbound.cycle ... alerts_considered: ≥1
fomo.send.attempted ... alert_id: <UUID>
fomo.send.succeeded ... alert_id: <UUID>, provider_status: QUEUED
state.transitioned ... <UUID>: approved → sent
```

**Your phone:** within a few seconds you should receive an iMessage from your
SendBlue number. That's the v0.1 demo's outbound side proven against real
Gmail + real OpenAI + real Slack + real SendBlue, end-to-end.

---

## 6. Reply to the iMessage

From the same iMessage thread on your phone, reply `tomorrow`.

Expect within ~5s in `/tmp/fomo-3g.log`:

```
fomo.sendblue.inbound_received ... secret_header_name: sb-signing-secret
fomo.sendblue.reply_parsed ... intent: snooze, intent_source: classifier, snooze_hint: tomorrow
state.transitioned ... <UUID>: sent → replied
state.transitioned ... <UUID>: replied → snoozed (until=<tomorrow ISO>)
feedback.written ... kind: user_snoozed
```

That's the inbound side proven against real SendBlue end-to-end on a NATURAL
alert (not a synthetic one like 3F.2).

---

## 7. Wait for cap, then stop

Let cycles tick until you see:

```
fomo.poll.cycle_cap_reached     cycles_run: 60, cycle_cap: 60
fomo.outbound.cycle_cap_reached cycles_run: 60, cycle_cap: 60
```

Then Ctrl-C the dev server. (Or `lsof -ti :8080 | xargs kill -9` if Ctrl-C
hangs.)

---

## 8. Run evidence

In a second terminal tab (env loaded):

```bash
pnpm --filter @brevio/fomo run smoke-evidence:3g 2>&1 | tee /tmp/fomo-3g-evidence.log
```

Must print `VERDICT: PASS`. Specifically:

- [ ] Gmail connected (oauth_tokens.needs_reauth=false; gmail_cursors advanced)
- [ ] Ranker works (rank_result for demo alert with label=important)
- [ ] Slack review works (queued_for_review → approved + fomo.slack.approval_captured)
- [ ] SendBlue send works (approved → sent + exactly 1 tool_invocations row)
- [ ] Reply parser works (sent → replied + fomo.sendblue.reply_parsed)
- [ ] Memory / feedback writes (feedback_events tied to alert + memory_signal writes)
- [ ] No duplicate sends (tool_invocations count = 1)
- [ ] No raw body / phone / secret leakage (leak-canary scan clean)

The script auto-discovers the "demo alert" (the most recent natural alert
whose state trail includes both approved→sent AND sent→replied). It prints
the alert_id — confirm it matches the one you saw in the log.

---

## 9. Clean-stop confirmation

Verify each kill switch turns its surface off cleanly:

```bash
FOMO_GMAIL_POLLING_ENABLED=false \
FOMO_RANKER_ENABLED=false \
FOMO_SLACK_REVIEW_ENABLED=false \
FOMO_SEND_ENABLED=false \
FOMO_SENDBLUE_INBOUND_ENABLED=false \
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-3g-cleanstop.log &
sleep 8
```

```bash
curl -s -o /dev/null -w "/sendblue/inbound HTTP %{http_code}\n" -X POST http://localhost:8080/sendblue/inbound
curl -s -o /dev/null -w "/slack/interactivity HTTP %{http_code}\n" -X POST http://localhost:8080/slack/interactivity
```

Expect HTTP 404 for both. Then stop:

```bash
lsof -ti :8080 | xargs kill -9 2>/dev/null
```

And verify no `fomo.sendblue.*` / `fomo.send.*` / `fomo.slack.*` rows landed
during the cleanstop window.

---

## 10. Fill in `docs/SMOKE_REPORT_3G.md`

Use `docs/SMOKE_REPORT_TEMPLATE_3G.md`. Required fields:

- §5 demo-alert state trail (paste from the audit_log query)
- §6 each of the 8 PASS criteria confirmed with the specific evidence row
- §7 full `smoke-evidence:3g` stdout
- §8 founder observations (anything surprising during the run)
- §10 verdict: PASS
- §11 sign-off + v0.1 milestone declaration

Commit. Open PR. Merge to main. v0.1 is done.

---

## What's intentionally NOT in 3G

- Friend beta (gated behind v0.5)
- Auto-send (gated behind its own gate after demo confidence)
- Group chats
- Proactive follow-up messages
- Snooze resurface (a new iMessage at `snooze_until`)
- Calendar / Drafting / Sending / MCP tools / Autonomous (L2+ surfaces; v0.5+)
- Any new tools or policy decisions
- Neon migration automation (separate small phase)
