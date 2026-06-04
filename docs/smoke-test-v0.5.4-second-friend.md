# Phase v0.5.4 — Second-Friend Cross-Tenant Smoke

> Founder + ONE SECOND real briefed iPhone friend (Friend B) complete the
> full Brevio FOMO flow end-to-end. v0.5.2 proved Brevio worked once
> (with Morris). v0.5.3 hardened the substrate. v0.5.4 is the
> **cross-tenant proof**: the system is not a one-off founder-supervised
> miracle, AND Morris's + founder's state remain bit-for-bit untouched.
>
> **The goal is NOT new functionality. The goal is cross-tenant proof
> with a second real human.** v0.5.4 PASS does NOT auto-unlock v1.0 or
> broad beta. It unlocks the next 6-question gate.

**Locked scope reminders (from `project_v05-4-second-friend-scope`):**
- ONE new real friend ("Friend B"). Briefed BEFORE invite mint.
- No third friend, no public self-serve, no auto-send, no SendBlue dedicated-line, no periodic reconciliation worker, no dashboard, no calendar/MCP/browser automation.
- 16 PASS criteria: 12 carry-forward from v0.5.2 + 4 NEW cross-tenant checks.
- If Morris's or founder's `stop_active` row is updated during the smoke window — that is a FAIL even if Friend B's chain succeeded.

---

## 0. Prerequisites (founder verifies before any code runs)

- [ ] `docs/SMOKE_REPORT_v0.5.2.md` AND `docs/SMOKE_REPORT_v0.5.3.md` on `main` with `VERDICT: PASS`
- [ ] **SendBlue account-tier check (carry-forward from v0.5.2 wall).** Free Sandbox / AI Agent tier supports inbound-first verification. Friend B will text the Brevio number once after onboarding — same as Morris.
- [ ] **Friend B briefed out-of-band.** Briefing covered all five topics: Gmail readonly, founder review surface, STOP semantics, beta status, expected volume. (Same script that worked for Morris.)
- [ ] Friend B agreed verbally to participate
- [ ] Friend B's phone is iMessage-capable iPhone (NOT Android)
- [ ] Friend B's Gmail account is actively used (not a dead inbox)
- [ ] Friend B's Gmail is **added to Google Cloud Console "Test users" list** (Testing-mode 403 wall — recurs every new Gmail)
- [ ] Friend B is reachable during the smoke window so they can confirm receipt + STOP
- [ ] Morris is **NOT being notified** of this smoke. He is unaware of v0.5.4; his account exists, his STOP from v0.5.2 is still in effect, and we want it to remain untouched.

If ANY of the above is "no" — stop here. Do not mint a token.

### §0.A Baseline snapshot (cross-tenant invariant)

**This is the load-bearing v0.5.4 pre-smoke step.** Capture Morris's + founder's `memory_signals` state BEFORE the smoke begins. Criteria 13/14/15 of the evidence script will diff against this baseline.

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Capture stop_active baseline for ALL users
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.4-baseline-stop-active.txt

# Capture sendblue_contact_status baseline for ALL users
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'sendblue_contact_status'
ORDER BY user_id;
" | tee /tmp/v0.5.4-baseline-contact-status.txt

# Capture Morris's user_id for the evidence env
psql "$DATABASE_URL" -P pager=off -tA -c "
SELECT id FROM users WHERE email='morrismita.101@gmail.com' AND is_founder=false LIMIT 1;
" | tee /tmp/v0.5.4-morris-user-id.txt
```

Paste these snapshots into `docs/SMOKE_REPORT_v0.5.4.md` §6 (cross-tenant baseline diff) BEFORE running the smoke. The evidence script will use `FOMO_V0_5_4_MORRIS_USER_ID` to identify Morris's rows.

Set `FOMO_V0_5_4_BASELINE_CONFIRMED=true` in your env only after these files are captured.

## 1. Env additions

Add to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_4_FRIEND_BRIEFED=true
FOMO_V0_5_4_FRIEND_NAME=<friend-b-first-name>          # report header only; NEVER audited
FOMO_V0_5_4_BASELINE_CONFIRMED=true                     # set AFTER §0.A capture
FOMO_V0_5_4_MORRIS_USER_ID=<paste-from-/tmp/v0.5.4-morris-user-id.txt>
FOMO_V0_5_4_LEAK_CANARIES=brevio-canary-cccc,brevio-canary-dddd   # different canaries than v0.5.2
FOMO_V0_5_4_WINDOW_HOURS=24
FOMO_GMAIL_POLLING_MAX_CYCLES=300                       # same minima as v0.5.2/v0.5.3
FOMO_OUTBOUND_MAX_CYCLES=300
```

Carry-forward (unchanged from prior smokes):
- `FOMO_FRIEND_BETA_ENABLED=true`
- `BREVIO_PHONE_HASH_KEY=<32-byte base64>`
- `FOMO_FRIEND_BETA_BASE_URL=https://<your-ngrok>.ngrok-free.dev` (HTTPS REQUIRED)

The Google Cloud OAuth client still lists `/oauth/google/callback` AND `/onboard/callback` at the public HTTPS URL. Friend B's Gmail is added to the test-user list.

## 2. Preflight + boot

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.4
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.4.log
```

Wait for boot log:
```
fomo.onboard.enabled    onboard_route_mounted: true
fomo.server.listening   ...
```

In a separate terminal, start ngrok pointed at `localhost:8080`. Verify `https://<your-ngrok>.ngrok-free.dev/onboard?token=any-fake-token` returns the "Invite link not valid" HTML page (404 is expected — it proves the route is mounted, not connection-refused).

## 3. Issue Friend B's invite (briefing gate fires here)

```bash
pnpm --filter @brevio/fomo run issue-friend-token \
  -- \
  --phone +1<friend-b-real-e164-no-dashes> \
  --confirm-briefed yes-friend-was-briefed
```

Same gate as v0.5.2 — refuses without `--confirm-briefed` for a real phone, refuses if `FOMO_FRIEND_BETA_ENABLED` is not `true`. Prints the invite URL ONCE.

Verify the briefing audit:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT jsonb_pretty(detail) AS detail FROM audit_log
WHERE action='fomo.onboard.invite_issued'
ORDER BY occurred_at DESC LIMIT 1;
"
```

Expected: `briefed_confirmed: true`, `phone_class: 'real'`, `intended_phone_slug` = Friend B's last 4. NEVER the raw E.164, NEVER the token plaintext.

## 4. Send the invite URL to Friend B

Use your normal channel (iMessage, Signal, etc.). One short message:

> Here's the Brevio invite link we talked about. Open this on your iPhone in any browser:
> `https://<your-ngrok>.ngrok-free.dev/onboard?token=<token>`
> One-time link, expires in 24h. Take your time reading the privacy copy at the bottom; reply here if anything's unclear before you click "Connect with Google."

## 5. Friend B completes /onboard

Friend B opens URL on their iPhone. Same flow Morris saw. Click "Connect with Google" → OAuth with Friend B's own Gmail → "You're connected."

Verify Friend B's `users` row (must be NEW, distinct from Morris):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, email, is_founder, phone_e164_hash IS NOT NULL AS has_phone_hash, created_at
FROM users
WHERE is_founder = false AND phone_e164_hash IS NOT NULL
ORDER BY created_at DESC LIMIT 5;
"
```

Expected: a NEW row with Friend B's Gmail, `is_founder=false`, `has_phone_hash=true`. Morris's row is also present (older `created_at`); both visible side by side.

Verify SendBlue contact auto-registration (v0.5.3 hardening item #1):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, jsonb_pretty(detail) AS detail, updated_at
FROM memory_signals
WHERE kind='sendblue_contact_status'
ORDER BY updated_at DESC LIMIT 5;
"
```

Expected: Friend B's row freshly written with `registered: true` (or `registered: false` with `error_reason`). Morris's row is older — `updated_at` predates the smoke window. **If Morris's row is overwritten, that is a cross-tenant write FAIL.**

## 6. Friend B texts the Brevio number once

Tell Friend B (out-of-band, same message you sent Morris):

> One more step — text "hi" to <SENDBLUE_FROM_NUMBER> from your iPhone. SendBlue needs to verify the number once before they'll let me send to you. Should take 5 seconds.

This is the SendBlue Free Sandbox verification gate. Without it, the outbound `fomo.send.contact_not_registered` may fire even though `/onboard/callback` reported registered=true.

## 7. Friend B's email surfaces as a friend-safe Slack card

If Friend B's inbox doesn't naturally have a FOMO-worthy email arriving, send them one (or have a third party send one). Use canary substrings that match `FOMO_V0_5_4_LEAK_CANARIES` (set above to `brevio-canary-cccc,brevio-canary-dddd` — different from v0.5.2's `aaaa/bbbb` so old artifacts don't pollute).

```
From: <a real address you control or a third party uses>
To: <Friend B's Gmail>
Subject: Re: Greenoaks partner intro — Thursday meeting?
Body:
Hi — quick confirm for Thursday 11am ET. Adam wanted me to loop you in
directly. Need your availability for a 30-min follow-up before EOW.
Internal reference: brevio-canary-cccc // brevio-canary-dddd
```

Wait one polling cycle (~10s). Watch the founder Slack channel:

| Visual check | Required |
|---|---|
| Card has **NO Snippet section** | ✅ |
| Card has **NO `message_id`** text | ✅ |
| Card has **NO `model_name` / `prompt_version`** in the footer | ✅ |
| Footer reads **"friend-owned (user redacted)"** | ✅ |
| Sender, Subject, Ranker `Why`, label, score all visible | ✅ |
| Approve / Reject buttons present | ✅ |
| Card payload contains ZERO substring from Friend B's email body | ✅ |

## 8. Founder approves in Slack

Click ✅ Approve on the Friend B card. State transitions:
- `queued_for_review → approved`
- Outbound worker fires
- v0.5.3 contact-gate: confirms Friend B's `sendblue_contact_status.registered=true`
- SendBlue sends iMessage to Friend B's iPhone

Verify the transition + send:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT alert_id, user_id, from_state, to_state, reason, at
FROM alert_state_transitions
WHERE at > now() - interval '15 minutes' AND user_id != 'founder'
ORDER BY at DESC LIMIT 10;
"
```

Expected: `approved → sent` for Friend B's user_id.

## 9. Friend B confirms receipt + texts STOP

Founder asks Friend B (out-of-band): "Did you get the iMessage? If yes, reply STOP."

Friend B replies STOP from the iMessage thread on their iPhone. SendBlue's webhook posts to `/sendblue/inbound`. The route resolves Friend B by phone hash and writes `memory_signals.stop_active=true` for **Friend B's user_id specifically**.

**Cross-tenant verification (this is the v0.5.4 load-bearing query):**

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind='stop_active'
ORDER BY updated_at DESC;
"
```

Expected:
- A row for **Friend B's user_id** with `active=true`, `source=user_confirmed`, `updated_at` within the last few minutes.
- **Morris's row** (if it exists) UNTOUCHED: `updated_at` predates the smoke start (compare against `/tmp/v0.5.4-baseline-stop-active.txt`).
- **Founder's row** (if it exists) UNTOUCHED.

If Morris's or founder's `updated_at` has moved into the smoke window — that's a **cross-tenant write regression**. STOP THE SMOKE. Investigate before continuing.

## 10. Founder regression (concurrent — proves no regression)

During the same smoke window, send yourself (founder Gmail → founder Gmail) a FOMO-worthy email. Wait one cycle. The founder card appears in Slack with the FULL v0.1 shape (Snippet present, footer with `message_id` + `model_name` + `prompt_version`, footer `user: founder`). Click Approve. Real iMessage arrives on YOUR phone.

This proves: Friend B's onboarding + STOP did NOT break the founder's own flow.

## 11. Run ALL FOUR evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
```

ALL FOUR must print `VERDICT: PASS`. v0.5.1 = substrate; v0.5.2 = first-friend specifics still hold; v0.5.3 = hardening still wired; v0.5.4 = cross-tenant proof.

**Known friction (carried from v0.5.3):** v0.5.2 evidence uses a 24h wall-clock window. If your v0.5.4 smoke runs >24h after the last v0.5.2 audit row was written, set `FOMO_V0_5_2_WINDOW_HOURS=$((<hours-since-v0.5.2-merge>+2))` to widen the window. This is a known followup (D in the v0.5.4 scope discussion).

## 12. Cross-tenant baseline diff

After §11 PASS, manually diff the §0.A baseline against the post-smoke state:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.4-post-stop-active.txt

diff /tmp/v0.5.4-baseline-stop-active.txt /tmp/v0.5.4-post-stop-active.txt | tee /tmp/v0.5.4-stop-active.diff
```

Expected diff:
- ONE new row added: Friend B's stop_active=true.
- ZERO modifications to Morris's row.
- ZERO modifications to founder's row.

Paste the diff into SMOKE_REPORT §6.

## 13. Fill in `docs/SMOKE_REPORT_v0.5.4.md`

Use `docs/SMOKE_REPORT_TEMPLATE_v0.5.4.md`. PASS requires all 16 criteria green + all four evidence scripts PASS + §6 cross-tenant diff shows no Morris/founder modifications.

Commit + open PR + (after CI green) merge → v0.5.4 done. The next phase is its own 6-question gate.

## 14. Aftercare for Friend B + check on Morris

After PASS, send Friend B one short message (same shape as Morris's aftercare):

> Brevio's done with the second-friend gate. You can keep texting STOP / START whenever; nothing's running against your inbox until I explicitly unpause. Thanks for the help.

For Morris: he was never told about v0.5.4 because the smoke was specifically designed to leave his state alone. Verify one more time that his `stop_active=true` is still in place (his v0.5.2 aftercare was him opted out):

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, jsonb_pretty(detail), updated_at FROM memory_signals
WHERE user_id='${FOMO_V0_5_4_MORRIS_USER_ID}' AND kind='stop_active';
"
```

If anything looks different from the §0.A baseline — that's the regression v0.5.4 was designed to catch. Investigate before the PR merges.

## 15. What v0.5.4 PASS does NOT promise (next-phase boundaries)

- A third friend — out
- Public self-serve onboarding — out indefinitely
- Auto-send — its own gate
- SendBlue dedicated-line upgrade — out
- Periodic reconciliation worker — still on-demand
- Production scaling sprint — out
- Dashboard — out
- Calendar / Drafting / MCP / browser automation — L2+ surfaces

The next phase is decided AT THE NEXT 6-question gate, with the founder's full attention on whatever Friend B's experience taught them.
