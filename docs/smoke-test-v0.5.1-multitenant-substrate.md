# Phase v0.5.1 — Multi-tenant Substrate (founder synthetic smoke)

> Eight-step substrate enabling friend onboarding. NO real friend involved
> in this smoke — a SECOND synthetic user with a DISTINCT synthetic phone
> proves the per-user routing, friend-safe card, and STOP isolation. The
> real friend smoke is v0.5.2.

**Locked scope reminders:**
- v0.5.1 substrate only — no real friend until v0.5.2
- No public self-serve, no auto-send, no broad dashboard, no new capabilities
- Two distinct synthetic phones — never reuse the founder's phone for the friend

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_3G1.md` on `main` with `VERDICT: PASS`
- [ ] You're on branch `phase-v0.5.1-multitenant-substrate` with all 8 steps shipped + tested

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a
```

## 1. Env additions

Add to your `.env.3b3.local`:

```
FOMO_FRIEND_BETA_ENABLED=true
BREVIO_PHONE_HASH_KEY=<32-byte base64; openssl rand -base64 32>
FOMO_FRIEND_BETA_BASE_URL=https://<your-ngrok-subdomain>.ngrok-free.dev
```

The Google Cloud OAuth client must list BOTH callback URLs:
- `https://<ngrok>/oauth/google/callback` (existing founder OAuth)
- `https://<ngrok>/onboard/callback` (new — friend onboarding)

## 2. Apply migrations 0005 + 0006 to Neon

```bash
pnpm --filter @brevio/fomo run migrate:neon
```

Verify the new columns exist:

```bash
psql "$DATABASE_URL" -c "\d users" | grep -E "phone_e164|is_founder"
psql "$DATABASE_URL" -c "\d invite_tokens" | grep -E "intended_phone_(hash|encrypted)"
```

## 3. Preflight + boot

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.1
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.1.log
```

Wait for the boot log to include:

```
fomo.onboard.enabled    onboard_route_mounted: true, privacy_copy_bytes: 1500+
fomo.server.listening   onboard_route_mounted: true, ...
```

If `fomo.onboard.disabled` shows instead, FOMO_FRIEND_BETA_ENABLED isn't `true` in the dev server's env. Re-source and restart.

## 4. Issue a synthetic friend invite

In a second terminal (env loaded):

```bash
pnpm --filter @brevio/fomo run issue-friend-token --phone +15550100002
```

The script prints exactly one plaintext token + the `/onboard?token=...` URL. Copy the URL.

**Confirm safe-only audit:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail FROM audit_log
WHERE action='fomo.onboard.invite_issued'
ORDER BY occurred_at DESC LIMIT 1;
"
```

Expected detail keys: `invite_id`, `token_hash_prefix` (8 chars), `intended_phone_slug` (last 4 = "0002"), `expires_at`, `ttl_hours`. **NEVER** the plaintext token or full phone.

## 5. Complete /onboard as the synthetic friend

Open the printed URL in a NEW browser tab (incognito recommended — keeps the founder's OAuth session out of the way).

You'll see the consent page rendering the privacy copy. Click **Connect with Google** and complete Google OAuth using your alt Gmail account (NOT the founder Gmail).

After OAuth, you should land on the "You're connected" page.

**Verify the friend's users row:**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, email, is_founder, phone_e164_hash IS NOT NULL AS has_phone_hash
FROM users
WHERE is_founder = false
ORDER BY created_at DESC LIMIT 3;
"
```

Expected: a row with the alt Gmail email, `is_founder=false`, `has_phone_hash=true`.

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT consumed_at, consumed_user_id
FROM invite_tokens
ORDER BY id DESC LIMIT 1;
"
```

Expected: `consumed_at` is set, `consumed_user_id` matches the friend's `users.id`.

## 6. Friend's email surfaces as a friend-safe Slack card

Send a FOMO-worthy email FROM the alt Gmail TO itself (so the polling worker picks it up):

```
Subject: Re: Greenoaks investor intro — Thursday meeting?
Body: <same as v0.1 demo body>
```

Wait one polling cycle (~10s). Watch the founder Slack channel:

- The card MUST NOT contain a **Snippet** section
- The card MUST NOT contain `message_id` in the footer
- The card MUST show "friend-owned (user redacted)" in the context line
- Sender + subject + ranker reason + label + score ARE shown
- Approve / Reject buttons present

This is the friend-safe rendering proof (Step 5).

## 7. Per-friend STOP isolation (synthetic)

Simulate a friend STOP via curl (don't actually text from your real phone — that would consume the SendBlue minutes + couple to live SendBlue infrastructure):

```bash
curl -X POST "https://<ngrok>/sendblue/inbound" \
  -H "sb-signing-secret: $SENDBLUE_WEBHOOK_SECRET" \
  -H "content-type: application/json" \
  -d '{"messageId":"smoke-v0.5.1-friend-stop","content":"STOP","number":"+15550100002"}'
```

Verify per-user isolation:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, detail, source, updated_at
FROM memory_signals
WHERE kind='stop_active'
ORDER BY updated_at DESC LIMIT 5;
"
```

Expected: a row with `user_id` = friend's UUID (NOT the founder's user_id), `detail.active = true`, `source = user_confirmed`. The founder's row (if it exists) is UNTOUCHED.

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, occurred_at
FROM audit_log
WHERE action='fomo.sendblue.stop_recorded'
ORDER BY occurred_at DESC LIMIT 5;
"
```

The top row should have `actor_user_id = friend's UUID`, NOT the founder.

## 8. Founder flow still works (regression)

Trigger an alert from the founder Gmail (same Greenoaks email pattern as v0.1):

- Watch the founder Slack channel for the FULL card (snippet present + user/message_id in footer — the v0.1 shape)
- Approve in Slack
- iMessage arrives on founder's phone

If your founder phone is still STOP'd from a prior smoke, text START first.

## 9. Clean-stop confirmation

Restart with FOMO_FRIEND_BETA_ENABLED=false:

```bash
lsof -ti :8080 | xargs kill -9 2>/dev/null; sleep 1
FOMO_FRIEND_BETA_ENABLED=false pnpm --filter @brevio/fomo dev > /tmp/fomo-v0.5.1-cleanstop.log 2>&1 &
sleep 8
```

Verify the boot log:

```bash
grep -E "fomo.onboard|onboard_route_mounted" /tmp/fomo-v0.5.1-cleanstop.log
```

Expected: `fomo.onboard.disabled` + `onboard_route_mounted: false`.

Verify `/onboard` returns 404:

```bash
curl -s -o /dev/null -w "/onboard HTTP %{http_code}\n" "http://localhost:8080/onboard"
```

Expected: HTTP 404.

```bash
lsof -ti :8080 | xargs kill -9 2>/dev/null
```

Verify the audit row was emitted:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT occurred_at, detail
FROM audit_log
WHERE action='fomo.onboard.kill_switch_off'
ORDER BY occurred_at DESC LIMIT 1;
"
```

## 10. Run the evidence script

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
```

Expect `VERDICT: PASS` with all required checks green. The evidence script DOES NOT visually verify the friend-safe Slack card or the clean-stop /onboard 404 — those are operator-confirmed in §6 + §9 of this runbook.

## 11. Fill in `docs/SMOKE_REPORT_v0.5.1.md`

Use `docs/SMOKE_REPORT_TEMPLATE_v0.5.1.md`. PASS = every required check above is green + operator confirmations in §6 and §9.

Commit + open PR + merge → v0.5.1 done → v0.5.2 (real friend smoke) is unblocked.
