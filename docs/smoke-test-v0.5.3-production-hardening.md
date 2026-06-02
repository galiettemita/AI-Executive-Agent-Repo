# Phase v0.5.3 — Production Hardening Smoke

> Re-runs v0.5.2's substrate against the four hardening fixes from
> [v0.5.2 SMOKE_REPORT §9](SMOKE_REPORT_v0.5.2.md#9). Founder synthetic
> smoke (alt-Gmail-as-friend, v0.5.1 pattern) — Morris remains opted
> out per his aftercare. Real-friend re-test is a future gate if needed.
>
> **v0.5.3 PASS does NOT auto-unlock v1.0.** Next phase runs its own
> 6Q gate.

---

## 0. Prerequisites

- [ ] `docs/SMOKE_REPORT_v0.5.2.md` on `main` with `VERDICT: PASS` (merge commit `249ba465`)
- [ ] You're on branch `phase-v0.5.3-production-hardening`
- [ ] No new friend involved. Use a second Gmail account you control (same `alt Gmail` pattern as v0.5.1)
- [ ] No SendBlue plan upgrade. Free Sandbox stays. The v0.5.3 fixes work on that tier.

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a
```

## 1. Preflight + boot

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.3
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.3.log
```

Wait for `fomo.server.listening`. Confirm the boot log shows:
- `fomo.onboard.enabled onboard_route_mounted: true`
- `fomo.poll.enabled` (with the OAuth refresh helper wired internally)
- No errors

## 2. Item #1 — SendBlue contact auto-registration

Mint a fresh invite for your alt Gmail with a **555-fictional phone** (so SendBlue's verified-contact gate is exercised AGAINST a test number, NOT against an existing real contact):

```bash
pnpm --filter @brevio/fomo run issue-friend-token -- --phone +15550100099
```

Complete /onboard with the alt Gmail. Once the friend is provisioned, confirm:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, jsonb_pretty(detail) AS detail, updated_at
FROM memory_signals
WHERE kind='sendblue_contact_status'
ORDER BY updated_at DESC LIMIT 3;
"
```

Expected: the friend's user_id has a `sendblue_contact_status` row with either `registered: true` (SendBlue accepted the contact-add) OR `registered: false, error_reason: 'send_disabled' | 'client_error: ...'`.

Then confirm the audit row:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT action, jsonb_pretty(detail) AS detail
FROM audit_log
WHERE action LIKE 'fomo.sendblue.contact_%'
ORDER BY occurred_at DESC LIMIT 3;
"
```

If the gate fires later (you try to approve a friend alert and get `fomo.send.contact_not_registered`), that's the contract — the substrate refused to send because SendBlue had not registered the contact.

## 3. Item #2 — OAuth auto-refresh

Force the founder's access_token to be near-expiry:

```bash
psql "$DATABASE_URL" -P pager=off -c "
UPDATE oauth_tokens
SET expires_at = now() - interval '1 minute', needs_reauth = false
WHERE user_id = 'founder' AND provider = 'google';
"
```

Watch the next polling cycle (~10s). The polling worker should:
1. Detect the expired token
2. Call Google's refresh endpoint via the stored refresh_token
3. Save the new token
4. Audit `fomo.oauth.refreshed`
5. Continue polling normally

Verify:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT action, jsonb_pretty(detail) AS detail, occurred_at
FROM audit_log
WHERE action IN ('fomo.oauth.refreshed', 'fomo.oauth.refresh_failed')
ORDER BY occurred_at DESC LIMIT 3;
"
```

Expected: `fomo.oauth.refreshed` with detail `{provider: 'google', expires_at_iso: '<future>', refresh_token_rotated: ...}`. NEVER the refresh_token plaintext.

## 4. Item #3 — Neon ECONNRESET resilience

The unit tests cover the synthetic-emit path. For the smoke, just confirm:

```bash
grep -E "fomo\.db\.connection_error|EADDRINUSE|unhandled" /tmp/fomo-v0.5.3.log | head -5
```

The server should be running clean. If a real Neon drop happens during the smoke window, you'll see a `fomo.db.connection_error` row in stderr — and the server STAYS UP.

(Optional defensive simulation, not required for PASS: write a small node REPL script that loads the pool and emits 'error' — the server must remain alive.)

## 5. Item #4 — Reconciliation script

```bash
pnpm --filter @brevio/fomo run ops:reconcile-sendblue
```

Expected: prints a summary including `gaps_found: N` where N is the count of SendBlue inbounds not in our audit log. If we've been running the server cleanly, N should be 0. If there are gaps from prior sessions (e.g., the v0.5.2 incident window), they'll be detected and audited as `fomo.sendblue.delivery_gap_detected`.

Verify:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT count(*) FROM audit_log WHERE action='fomo.sendblue.delivery_gap_detected';
"
```

## 6. Founder regression (re-prove v0.5.2 path)

Send yourself a FOMO-worthy founder email; approve in Slack; confirm real iMessage arrives. Same as v0.5.2 §9. The substrate must not have regressed.

## 7. Run all three evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
```

All three must print `VERDICT: PASS`.

## 8. Fill in `docs/SMOKE_REPORT_v0.5.3.md`

Use `docs/SMOKE_REPORT_TEMPLATE_v0.5.3.md`. PASS requires all 7 hardening criteria green + all three evidence scripts PASS.

Commit + open PR + merge → v0.5.3 done. Next phase is its own 6Q gate.
