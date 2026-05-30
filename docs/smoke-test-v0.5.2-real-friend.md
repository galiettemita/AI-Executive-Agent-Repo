# Phase v0.5.2 — Real Friend Beta Smoke

> Founder + ONE real briefed iPhone friend complete the full Brevio FOMO
> flow end-to-end. v0.5.1 substrate is proven; v0.5.2 is the experience
> proof.

**Locked scope reminders (from `project_v05-2-real-friend-scope`):**
- ONE real friend. ONE. Not two.
- Briefing happens BEFORE the invite is minted — out-of-band conversation, not the privacy copy.
- Friend has an iMessage-capable iPhone. Android/SMS is a separate future smoke.
- v0.5.2 PASS does **not** auto-unlock v1.0. It unlocks the next 6-question gate.

---

## 0. Prerequisites (founder verifies before any code runs)

- [ ] `docs/SMOKE_REPORT_v0.5.1.md` on `main` with `VERDICT: PASS` (substrate proven)
- [ ] The friend has been **briefed out-of-band**. Briefing covered:
  - What Brevio reads (Gmail subjects + senders + body for the ranker call only, body NOT persisted)
  - What you (the founder) see (Slack card with sender + subject + ranker reason, NO body)
  - How STOP works (text STOP from their iMessage thread, full revocation)
  - Beta status (you are the only reviewer, no auto-send, they can pull out any time)
  - Approximate volume (one iMessage at most every N hours; how rare a FOMO-worthy email is)
- [ ] Friend has agreed verbally to participate
- [ ] Friend's phone is **iMessage-capable iPhone** (not Android — SendBlue's behavior differs)
- [ ] Friend's Gmail account is one they actively use (not a dead inbox)
- [ ] Friend is reachable during the smoke window so they can confirm receipt + STOP

If ANY of the above is "no" — stop here. Do not mint a token.

## 1. Env additions

Add to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_2_FRIEND_BRIEFED=true
# Optional: leak-canary substrings — set BEFORE issuing the invite + sending
# the test email. The evidence script scans audit_log + memory_signals +
# alert_state_transitions for these and fails if any leak.
FOMO_V0_5_2_LEAK_CANARIES=brevio-canary-aaaa,brevio-canary-bbbb
# Optional: smoke window in hours (default 24). The evidence script scopes
# all DB queries to this window so older smoke artifacts don't pollute the
# evidence.
FOMO_V0_5_2_WINDOW_HOURS=24
# Raise both worker cycle caps — v0.5.1 smoke hit cycle_cap_reached
# mid-run; for a real-friend smoke coordinating out-of-band this MUST
# be high enough to outlast the founder + friend exchange.
FOMO_GMAIL_POLLING_MAX_CYCLES=300
FOMO_OUTBOUND_MAX_CYCLES=300
```

Carry-forward from v0.5.1 (already set if you ran v0.5.1):
- `FOMO_FRIEND_BETA_ENABLED=true`
- `BREVIO_PHONE_HASH_KEY=<32-byte base64>`
- `FOMO_FRIEND_BETA_BASE_URL=https://<your-ngrok>.ngrok-free.dev` (HTTPS REQUIRED — friend is on a different device)

The Google Cloud OAuth client must list:
- `https://<your-ngrok>.ngrok-free.dev/oauth/google/callback`
- `https://<your-ngrok>.ngrok-free.dev/onboard/callback`

(`http://localhost:8080/*` callbacks from v0.5.1 are insufficient — friend is not on your machine.)

## 2. Preflight + boot

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

pnpm --filter @brevio/fomo run preflight:v0.5.2
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.2.log
```

Wait for the boot log to include:
```
fomo.onboard.enabled    onboard_route_mounted: true, privacy_copy_bytes: 3000+
fomo.server.listening   onboard_route_mounted: true, ...
```

In a separate terminal, start ngrok pointed at `localhost:8080`:
```bash
ngrok http --domain=<your-static-subdomain>.ngrok-free.dev 8080
```

Verify the tunnel is up — check `https://<your-ngrok>.ngrok-free.dev/onboard?token=any-fake-token` returns the "Invite link not valid" page (not a 404 or connection refused). That proves the tunnel + route are wired.

## 3. Issue the friend invite (briefing gate fires here)

In a second terminal (env loaded):

```bash
pnpm --filter @brevio/fomo run issue-friend-token \
  -- \
  --phone +1<friend-real-e164-no-dashes> \
  --confirm-briefed yes-friend-was-briefed
```

The script will:
- REFUSE if `--confirm-briefed yes-friend-was-briefed` is missing for a real phone (correction #2)
- REFUSE if `FOMO_FRIEND_BETA_ENABLED` is not `true`
- Mint a single one-time token bound to the friend's intended E.164
- Print the invite URL ONCE — copy it; the plaintext token is not recoverable from the DB

**Verify the briefing audit:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail FROM audit_log
WHERE action='fomo.onboard.invite_issued'
ORDER BY occurred_at DESC LIMIT 1;
"
```

Expected detail keys: `invite_id`, `token_hash_prefix` (8 chars), `intended_phone_slug` (last 4), `expires_at`, `ttl_hours`, `briefed_confirmed: true`, `phone_class: "real"`. NEVER the plaintext token, NEVER the raw phone.

## 4. Send the invite URL to the friend

Use your normal channel (iMessage, Signal, email — whatever you used for briefing). One short message:

> Here's the Brevio invite link we talked about. Open this on your iPhone in any browser:
> `https://<your-ngrok>.ngrok-free.dev/onboard?token=<token>`
> One-time link, expires in 24h. Take your time reading the privacy copy at the bottom; reply here if anything's unclear before you click "Connect with Google."

## 5. Friend completes /onboard

Friend opens the URL on their iPhone. They see:
- The consent page rendering the privacy copy verbatim
- "Connect with Google" button
- Privacy text covering Gmail readonly, founder review, STOP semantics, beta status

Friend clicks "Connect with Google" → completes Google OAuth with **their own Gmail** → lands on the "You're connected" page.

**Verify the friend's users row:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, email, is_founder, phone_e164_hash IS NOT NULL AS has_phone_hash
FROM users
WHERE is_founder = false AND phone_e164_hash IS NOT NULL
ORDER BY created_at DESC LIMIT 3;
"
```

Expected: a row with the friend's actual Gmail, `is_founder=false`, `has_phone_hash=true`.

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT consumed_at, consumed_user_id
FROM invite_tokens
ORDER BY id DESC LIMIT 1;
"
```

Expected: `consumed_at` is set, `consumed_user_id` matches the friend's `users.id`.

## 6. Friend's email surfaces as a friend-safe Slack card

If the friend's inbox doesn't naturally have a FOMO-worthy email arriving in the next ~10 minutes, send them one yourself (or have a third party send one). Recommended test pattern — a real-sounding investor / partner / deadline email that includes your leak-canary substrings so the §10 evidence scan can verify nothing leaked:

```
From: <a real address you control or a third party uses>
To: <friend's Gmail>
Subject: Re: Greenoaks partner intro — Thursday meeting?
Body:
Hi — quick confirm for Thursday 11am ET. Adam wanted me to loop you in
directly. Need your availability for a 30-min follow-up before EOW.
Internal reference: brevio-canary-aaaa // brevio-canary-bbbb
```

(The canary substrings match what you set in `FOMO_V0_5_2_LEAK_CANARIES`. If they appear anywhere in audit_log / memory_signals / state transitions, §10's evidence script fails.)

Wait one polling cycle (~10s). Watch the founder Slack channel:

| Visual check | Required |
|---|---|
| Card has **NO Snippet section** | ✅ |
| Card has **NO `message_id`** text | ✅ |
| Card has **NO `model_name` / `prompt_version`** in the footer | ✅ |
| Footer reads **"friend-owned (user redacted)"** | ✅ |
| Sender, Subject, Ranker `Why`, label, score all visible | ✅ |
| Approve / Reject buttons present | ✅ |

The card payload must contain ZERO substring from the friend's email body. If the ranker `Why` text quotes the body verbatim, that's a leak — `Why` must be a summary, not a paraphrase that includes body words.

## 7. Founder approves in Slack

Click ✅ Approve on the friend-owned card. State transitions:
- `queued_for_review → approved`
- Outbound worker fires
- SendBlue sends iMessage from the Brevio number to the friend's iPhone

Verify the state transition + send audit:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT alert_id, user_id, from_state, to_state, reason, at
FROM alert_state_transitions
WHERE at > now() - interval '15 minutes' AND user_id != 'founder'
ORDER BY at DESC LIMIT 10;
"
```

Expected: a `approved → sent` transition with `user_id` = friend's UUID.

## 8. Friend confirms receipt + texts STOP

Founder asks friend (out-of-band): "Did you get the iMessage just now? If yes, reply STOP to that thread."

Friend replies STOP from the iMessage thread on their iPhone. SendBlue's webhook posts to `/sendblue/inbound`. The route resolves friend by phone hash and writes `memory_signals.stop_active=true` for THAT user_id.

Verify per-friend isolation:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind='stop_active'
ORDER BY updated_at DESC LIMIT 5;
"
```

Expected: a row with `user_id` = friend's UUID, `active=true`, `source=user_confirmed`. Founder's `stop_active` row (if it exists) is UNTOUCHED.

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT action, actor_user_id, jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action='fomo.sendblue.stop_recorded'
ORDER BY occurred_at DESC LIMIT 5;
"
```

The top row should have `actor_user_id` = friend's UUID, `provider_message_id` is an Apple/SendBlue UUID-shaped id (NOT `smoke-v0.5.*`), `from_slug` is the last 4 of the friend's phone.

## 9. Founder regression (concurrent — proves no regression)

During the same smoke window, send yourself (founder Gmail → founder Gmail) a FOMO-worthy email. Wait one cycle. The founder card appears in Slack with the FULL v0.1 shape (Snippet present, full footer with `message_id` + `model_name` + `prompt_version`, footer `user: founder` NOT "friend-owned"). Click Approve. Real iMessage arrives on YOUR phone.

This proves: the friend's onboarding + STOP did NOT break the founder's own flow.

## 10. Run BOTH evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
```

BOTH must print `VERDICT: PASS`. v0.5.1 proves the substrate is still healthy. v0.5.2 proves the real-friend specifics: briefing recorded, real friend exists, real iMessage delivered, real STOP captured, founder regression intact, leak-canary clean.

## 11. Fill in `docs/SMOKE_REPORT_v0.5.2.md`

Use `docs/SMOKE_REPORT_TEMPLATE_v0.5.2.md`. PASS requires all 12 criteria green + both evidence scripts PASS.

Commit + open PR + (after CI green) merge → v0.5.2 done. The next phase is its own 6-question gate (could be hardening, multi-friend, auto-send, or something else — v0.5.2 PASS does NOT auto-unlock v1.0).

## 12. Aftercare for the friend

After PASS, send the friend one short message:

> Brevio's done with the beta gate. You can keep texting STOP / START whenever; nothing's running against your inbox until I explicitly unpause. Thanks for the help.

Don't leave them onboarded into an indefinite background polling process unless they've explicitly said they want to keep using Brevio post-beta. If they don't: revoke their OAuth token from Google's side and DELETE their `users` + `oauth_tokens` + `gmail_cursors` rows after they confirm.
