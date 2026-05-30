// Phase v0.5.2 preflight — real-friend beta smoke.
//
// Validates every env var required for the founder-with-real-friend
// smoke test BEFORE the server boots. Inherits v0.5.1 + all prior gates
// and layers v0.5.2-specific gates on top:
//
//   * FOMO_FRIEND_BETA_ENABLED=true (carries forward; the dispatcher
//     gates /onboard on this)
//   * FOMO_V0_5_2_FRIEND_BRIEFED=true — founder asserts they briefed
//     the friend OUT-OF-BAND before this smoke begins. Required.
//   * Founder phone is NOT in NANPA-reserved fictional range (5550100..0199)
//   * Founder phone is configured for SendBlue inbound routing
//   * Both worker cycle caps explicit and ≥ 60 (so the smoke doesn't
//     hit cycle_cap_reached mid-coordination with a real friend)
//
// Forbidden during v0.5.2 (founder directive — beta-narrow):
//   * FOMO_AUTO_SEND_ENABLED=true — manual approval only
//
// The runbook (docs/smoke-test-v0.5.2-real-friend.md) covers the
// out-of-band operational requirements preflight cannot check:
//   * Friend has consented out-of-band
//   * Friend's phone is iMessage-capable (not Android/SMS)
//   * Friend's Gmail has — or will receive — a FOMO-worthy test email
//   * Friend's intended_phone is NOT in NANPA-reserved fictional range
//     (enforced mechanically by issue-friend-token instead, since
//     preflight doesn't see the invite at mint time)
//
// No network. No DB. Pure config inspection.

import { loadKillSwitches } from '../src/core/kill-switches.js';

type Severity = 'error' | 'warn';

interface Check {
  readonly name: string;
  readonly severity: Severity;
  readonly message: string;
}

const issues: Check[] = [];

function require_(name: string, message: string): void {
  if (!(process.env[name] ?? '').trim()) {
    issues.push({ name, severity: 'error', message });
  }
}

function requireMin(name: string, minBytes: number, message: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({ name, severity: 'error', message: `${message} (missing)` });
    return;
  }
  let decoded: Buffer;
  try {
    decoded = raw.startsWith('hex:') ? Buffer.from(raw.slice(4), 'hex') : Buffer.from(raw, 'base64');
  } catch {
    issues.push({ name, severity: 'error', message: `${message} — not a valid base64 or hex: value` });
    return;
  }
  if (decoded.length < minBytes) {
    issues.push({
      name,
      severity: 'error',
      message: `${message} — decoded length ${decoded.length} bytes, need ${minBytes}`
    });
  }
}

function expectEquals(name: string, expected: string, message: string): void {
  const v = (process.env[name] ?? '').trim();
  if (v !== expected) {
    issues.push({
      name,
      severity: 'error',
      message: `${message} (expected '${expected}', got '${v || '<unset>'}')`
    });
  }
}

console.log('Phase v0.5.2 preflight — real-friend beta smoke\n');

/* ---------------------------------------------------------------------- */
/* Substrate (DATABASE_URL + crypto keys — carry-forward from v0.5.1)     */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin(
  'BREVIO_PHONE_HASH_KEY',
  32,
  'BREVIO_PHONE_HASH_KEY must be a 32-byte key (HMAC over normalized E.164; separate from BREVIO_TOKEN_KEK)'
);

/* ---------------------------------------------------------------------- */
/* v0.5.1 substrate invariants (still required for v0.5.2)                */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'Phase v0.5.2 still requires the substrate kill switch ON. /onboard mounts only when this is "true".'
);

/* ---------------------------------------------------------------------- */
/* v0.5.2-specific invariants (founder directive 2026-05-30)              */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_2_FRIEND_BRIEFED',
  'true',
  'v0.5.2 hard gate (correction #2 — no surprise OAuth): the founder must brief the friend out-of-band BEFORE this smoke starts. ' +
    'Set FOMO_V0_5_2_FRIEND_BRIEFED=true after you have walked the friend through the privacy copy, the Gmail-readonly scope, the STOP semantics, ' +
    'and the beta status. This is an assertion, not a value the system can check — set it honestly.'
);

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message:
      'FOMO_FRIEND_BETA_BASE_URL required. The /onboard friend OAuth flow uses this to build its redirect_uri. ' +
      'Set to your ngrok https URL (e.g. https://<your-ngrok>.ngrok-free.dev). For v0.5.2 the friend is on a DIFFERENT device — localhost is INSUFFICIENT.'
  });
} else if (!/^https:\/\//.test(friendBaseUrl)) {
  // v0.5.2 specifically: real friend on their own device → MUST be
  // HTTPS. localhost / plain http would only work if the friend was on
  // the founder's machine, which contradicts "real friend on real device."
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message: `FOMO_FRIEND_BETA_BASE_URL='${friendBaseUrl.slice(0, 60)}...' must start with https:// for v0.5.2. ` +
      'A real friend on their own device cannot reach http://localhost on the founder machine. Use the ngrok URL (or a real domain when available).'
  });
}

/* ---------------------------------------------------------------------- */
/* Founder phone — must NOT be in NANPA-reserved fictional range          */
/* ---------------------------------------------------------------------- */

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: 'FOMO_FOUNDER_PHONE_NUMBER required for §10 founder regression check.'
  });
} else if (/^\+1555010\d{4}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='+1555010xxxx' is in NANPA-reserved fictional range. v0.5.2 requires a REAL founder phone — the regression check sends a real iMessage.`
  });
}

/* ---------------------------------------------------------------------- */
/* Google OAuth (friend onboarding reuses the existing config)            */
/* ---------------------------------------------------------------------- */

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required.');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required. v0.5.2: confirm BOTH /oauth/google/callback AND /onboard/callback are registered on your Google Cloud OAuth client at the public HTTPS URL the friend will resolve.'
);

/* ---------------------------------------------------------------------- */
/* SendBlue (real iMessage delivery is the v0.5.2 PASS criterion)         */
/* ---------------------------------------------------------------------- */

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required — real iMessage delivery to a real friend phone (canonical name from 3F.2; index.ts:395 reads this).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required (canonical name from 3F.2; index.ts:396 reads this).');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required (the Brevio-side sender number).');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required (inbound STOP needs the signed-header check to pass).');

/* ---------------------------------------------------------------------- */
/* Carry-overs (Slack + send + inbound + ranker — all required)           */
/* ---------------------------------------------------------------------- */

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker + reply parser).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on — friend email must surface as important.');
expectEquals('FOMO_SLACK_REVIEW_ENABLED', 'true', 'Slack review must be on — friend-safe card is the proof.');
expectEquals('FOMO_SEND_ENABLED', 'true', 'FOMO_SEND_ENABLED must be true so the outbound worker fires for the founder approval.');
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true so the friend STOP webhook is received.'
);
expectEquals('FOMO_GMAIL_POLLING_ENABLED', 'true', 'FOMO_GMAIL_POLLING_ENABLED must be true so both users are polled.');
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');

/* ---------------------------------------------------------------------- */
/* Worker cycle caps — v0.5.2-specific minimums                           */
/* ---------------------------------------------------------------------- */
//
// v0.5.1 smoke surfaced that 30 cycles (default) is too short for a
// founder-coordinating-with-a-real-friend smoke; the worker cap fires
// mid-smoke and approvals fail to deliver. v0.5.2 requires explicit
// minimums (≥ 60 each = 10 min) so the founder + friend can take their
// time without polling/outbound stalling.

function checkCycleMin(name: string, min: number, ctx: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({
      name,
      severity: 'error',
      message: `${name} required for v0.5.2 (${ctx}). Set ≥ ${min} explicitly to avoid mid-smoke cycle_cap_reached pauses.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.2 minimum of ${min}. ${ctx}`
    });
  }
}

checkCycleMin(
  'FOMO_GMAIL_POLLING_MAX_CYCLES',
  60,
  'A real-friend smoke needs the polling worker live for at least 10 minutes so the friend has time to send + receive emails without the worker pausing.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  60,
  'A real-friend smoke needs the outbound worker live for at least 10 minutes so founder-approved alerts can fire without the worker pausing.'
);

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.2 (same as v0.5.1)                                   */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);

if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'v0.5.2 hard boundary: founder review required. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true is a hard error in v0.5.2 — real friend onboarding cannot use ephemeral per-process random keys (the invite token would be unrecoverable after restart, and the friend would lose access mid-flow).'
  });
}

console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.filter((i) => i.severity === 'error').length === 0) {
  if (issues.length > 0) {
    console.log(`! ${issues.length} warning(s):\n`);
    for (const w of issues) {
      console.log(`  [WARN]  ${w.name}: ${w.message}`);
    }
    console.log('');
  }
  console.log('✓ Preflight passed.');
  console.log('');
  console.log('  Next steps (see docs/smoke-test-v0.5.2-real-friend.md):');
  console.log('    1. CONFIRM out-of-band: friend has been briefed (privacy copy, scope, STOP, beta status)');
  console.log('    2. CONFIRM out-of-band: friend has an iMessage-capable phone (iPhone preferred)');
  console.log('    3. Start dev server: pnpm --filter @brevio/fomo run dev');
  console.log("    4. Issue REAL friend invite: pnpm issue-friend-token --phone +1<friend's-real-e164> --confirm-briefed yes-friend-was-briefed");
  console.log('    5. Send invite URL to friend via your existing channel (text, email, Signal, etc.)');
  console.log('    6. Friend opens URL on their own device → OAuth as their own Gmail → "You\'re connected"');
  console.log('    7. (Optional) friend sends themselves the safe test email pattern from §6 of the runbook');
  console.log("    8. Founder Slack: confirm friend-safe card; Approve; verify iMessage lands on friend's real phone");
  console.log("    9. Friend texts STOP from their iMessage thread; verify per-friend isolation");
  console.log('   10. pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2');
  process.exit(0);
}

const errors = issues.filter((i) => i.severity === 'error');
const warns = issues.filter((i) => i.severity === 'warn');

console.log(`✖ ${errors.length} required check(s) failed:\n`);
for (const e of errors) {
  console.log(`  [ERROR] ${e.name}: ${e.message}`);
}
console.log('');
if (warns.length > 0) {
  console.log(`! ${warns.length} warning(s):\n`);
  for (const w of warns) {
    console.log(`  [WARN]  ${w.name}: ${w.message}`);
  }
  console.log('');
}
process.exit(1);
