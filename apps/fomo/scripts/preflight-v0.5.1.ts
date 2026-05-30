// Phase v0.5.1 preflight — multi-tenant substrate smoke.
//
// Validates every env var required for the founder synthetic-two-user
// smoke test BEFORE the server boots. Inherits 3G.1 + all prior gates
// and layers the v0.5.1 friend-beta requirements on top:
//
//   * FOMO_FRIEND_BETA_ENABLED=true (the dispatcher's gate; without
//     it /onboard returns null → 404)
//   * BREVIO_PHONE_HASH_KEY (32-byte HMAC key; separate from
//     BREVIO_TOKEN_KEK per separation-of-duties)
//   * BREVIO_OAUTH_STATE_KEY + GOOGLE_CLIENT_ID/SECRET +
//     BREVIO_OAUTH_REDIRECT_URI_GOOGLE (friend onboarding reuses
//     the existing Google OAuth config)
//   * FOMO_FRIEND_BETA_BASE_URL (informational; the URL the founder
//     shares with the friend, built from the dev server hostname)
//
// Forbidden during v0.5.1 (founder directive — substrate only):
//   * FOMO_AUTO_SEND_ENABLED=true — manual approval only
//   * Any v0.5+ "no real friend yet" assertion is operational, not
//     enforceable here. The runbook calls it out.
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

console.log('Phase v0.5.1 preflight — multi-tenant substrate smoke\n');

/* ---------------------------------------------------------------------- */
/* Substrate (DATABASE_URL + crypto keys)                                 */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

/* ---------------------------------------------------------------------- */
/* v0.5.1 invariants                                                      */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'Phase v0.5.1 invariant: friend beta MUST be on for the smoke. /onboard mounts only when this is "true".'
);

requireMin(
  'BREVIO_PHONE_HASH_KEY',
  32,
  'BREVIO_PHONE_HASH_KEY must be a 32-byte key (used for HMAC over normalized E.164 — separate from BREVIO_TOKEN_KEK per separation-of-duties)'
);

/* ---------------------------------------------------------------------- */
/* Google OAuth (friend onboarding reuses the existing config)            */
/* ---------------------------------------------------------------------- */

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (friend OAuth uses the existing Google client).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required. NOTE: the /onboard friend flow uses /onboard/callback, NOT /oauth/google/callback. ' +
    'Make sure your Google Cloud OAuth client has BOTH callback paths registered as valid redirect URIs.'
);

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'warn',
    message:
      'FOMO_FRIEND_BETA_BASE_URL not set. Informational only — issue-friend-token uses this to build the URL it prints. ' +
      'Without it the script prints `<YOUR_BASE_URL>/onboard?token=...` and the founder builds the URL by hand.'
  });
} else if (!/^https?:\/\//.test(friendBaseUrl)) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'warn',
    message: `FOMO_FRIEND_BETA_BASE_URL='${friendBaseUrl.slice(0, 40)}...' should start with https:// or http://`
  });
}

/* ---------------------------------------------------------------------- */
/* Carry-overs (Slack + Send + Inbound — the friend's chain still needs   */
/* every prior phase's surface to function)                               */
/* ---------------------------------------------------------------------- */

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker + reply parser).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on for the friend smoke (their email needs to surface).');
expectEquals('FOMO_SLACK_REVIEW_ENABLED', 'true', 'Slack review must be on — friend-safe card is the proof of Step 5.');
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');
require_('FOMO_FOUNDER_PHONE_NUMBER', 'FOMO_FOUNDER_PHONE_NUMBER required.');

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.1                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);

if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase v0.5.1 non-goal: founder review required. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message:
      'BREVIO_DEV_MODE=true bypasses production fail-closed checks. Phone hash + token KEK will use per-process random keys — ' +
      'invite tokens issued before restart will be unrecoverable. Acceptable for a single-session smoke; UNSET for any longer-lived test.'
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
  console.log('  Next steps (see docs/smoke-test-v0.5.1-multitenant-substrate.md):');
  console.log('    1. Apply migration 0006 to Neon (pnpm migrate:neon)');
  console.log('    2. Start dev server: pnpm --filter @brevio/fomo run dev');
  console.log('    3. Confirm fomo.onboard.enabled appears in the boot log');
  console.log('    4. Issue a synthetic friend invite (alt phone): pnpm issue-friend-token --phone +15550100002');
  console.log('    5. Open the printed URL → complete OAuth as alt Gmail');
  console.log('    6. Send an email from the friend Gmail; observe a friend-safe Slack card');
  console.log('    7. Synthetic STOP via curl with the friend phone → assert per-user isolation');
  console.log('    8. pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1');
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
