// Phase v0.5.4 preflight — second-friend cross-tenant smoke.
//
// Pure config inspection — no DB, no network. Validates that the
// substrate is the v0.5.2 + v0.5.3 shape (so Friend B onboards through
// the same proven path) AND that the v0.5.4-specific gates are explicit:
//
//   * v0.5.2 carry-forward: FOMO_FRIEND_BETA_ENABLED=true, real founder
//     phone, HTTPS ngrok base URL (Friend B is on a different device),
//     ≥ 60 cycle caps on both workers.
//   * v0.5.3 carry-forward: 7 hardening audit actions registered;
//     `sendblue_contact_status` memory-signal kind registered.
//   * v0.5.4-specific:
//     - FOMO_V0_5_4_FRIEND_BRIEFED=true (founder asserts Friend B was
//       briefed out-of-band BEFORE this smoke).
//     - FOMO_V0_5_4_FRIEND_NAME set (first name only — used in the
//       SMOKE_REPORT for traceability; never logged to audit_log).
//     - FOMO_V0_5_4_BASELINE_CONFIRMED=true (founder asserts they ran
//       the runbook §0 baseline snapshot of Morris's + founder's
//       `memory_signals` rows BEFORE this smoke — that snapshot is what
//       criteria 13/14 are diffed against).
//
// Forbidden in v0.5.4 (same as v0.5.2/v0.5.3):
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true (ephemeral keys break friend-bound tokens)
//
// The runbook (docs/smoke-test-v0.5.4-second-friend.md) covers the
// out-of-band requirements preflight cannot check:
//   * Friend B has been briefed
//   * Friend B's Gmail is added to Google Cloud Console test-user list
//   * Morris's onboarded row still present + `stop_active=true` baseline
//   * Founder's `stop_active` baseline noted
//   * Friend B knows they MUST text the Brevio number once after
//     onboarding (SendBlue Free Sandbox verification — same as v0.5.2).

import { loadKillSwitches } from '../src/core/kill-switches.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';

type Severity = 'error' | 'warn';
interface Check { readonly name: string; readonly severity: Severity; readonly message: string }
const issues: Check[] = [];

function require_(name: string, message: string): void {
  if (!(process.env[name] ?? '').trim()) issues.push({ name, severity: 'error', message });
}
function requireMin(name: string, minBytes: number, message: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) { issues.push({ name, severity: 'error', message: `${message} (missing)` }); return; }
  let decoded: Buffer;
  try { decoded = raw.startsWith('hex:') ? Buffer.from(raw.slice(4), 'hex') : Buffer.from(raw, 'base64'); }
  catch { issues.push({ name, severity: 'error', message: `${message} — not a valid base64 or hex: value` }); return; }
  if (decoded.length < minBytes) issues.push({ name, severity: 'error', message: `${message} — decoded length ${decoded.length} bytes, need ${minBytes}` });
}
function expectEquals(name: string, expected: string, message: string): void {
  const v = (process.env[name] ?? '').trim();
  if (v !== expected) issues.push({ name, severity: 'error', message: `${message} (expected '${expected}', got '${v || '<unset>'}')` });
}
function checkCycleMin(name: string, min: number, ctx: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({ name, severity: 'error', message: `${name} required for v0.5.4 (${ctx}). Set ≥ ${min} explicitly.` });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({ name, severity: 'error', message: `${name}=${raw} is below the v0.5.4 minimum of ${min}. ${ctx}` });
  }
}

console.log('Phase v0.5.4 preflight — second-friend cross-tenant smoke\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 + v0.5.2 + v0.5.3)                     */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals('FOMO_FRIEND_BETA_ENABLED', 'true', 'v0.5.4 still requires the substrate kill switch ON. /onboard mounts only when this is "true".');

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required.');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required. Confirm /oauth/google/callback AND /onboard/callback are registered at the public HTTPS URL.');

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (contact auto-registration + reconciliation).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker + reply parser).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on.');
expectEquals('FOMO_SLACK_REVIEW_ENABLED', 'true', 'Slack review must be on.');
expectEquals('FOMO_SEND_ENABLED', 'true', 'FOMO_SEND_ENABLED must be true for outbound to fire on founder approval.');
expectEquals('FOMO_SENDBLUE_INBOUND_ENABLED', 'true', 'FOMO_SENDBLUE_INBOUND_ENABLED must be true for Friend B STOP.');
expectEquals('FOMO_GMAIL_POLLING_ENABLED', 'true', 'FOMO_GMAIL_POLLING_ENABLED must be true so all three users are polled.');
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: 'FOMO_FOUNDER_PHONE_NUMBER required for the founder regression check.'
  });
} else if (/^\+1555010\d{4}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='+1555010xxxx' is NANPA-reserved fictional. v0.5.4 requires a real founder phone for the regression check.`
  });
}

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message: 'FOMO_FRIEND_BETA_BASE_URL required. Friend B is on a different device — localhost is INSUFFICIENT. Use the HTTPS ngrok URL.'
  });
} else if (!/^https:\/\//.test(friendBaseUrl)) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message: `FOMO_FRIEND_BETA_BASE_URL='${friendBaseUrl.slice(0, 60)}...' must start with https:// for v0.5.4 (Friend B is on a different device).`
  });
}

checkCycleMin(
  'FOMO_GMAIL_POLLING_MAX_CYCLES',
  60,
  'Real-friend smoke needs the polling worker live ≥ 10 minutes so Friend B has time without cycle_cap_reached pauses.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  60,
  'Real-friend smoke needs the outbound worker live ≥ 10 minutes so the founder approval can fire without pauses.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.3 runtime registry invariants (still required for v0.5.4)         */
/* ---------------------------------------------------------------------- */

const requiredAuditActions = [
  'fomo.sendblue.contact_registered',
  'fomo.sendblue.contact_registration_failed',
  'fomo.send.contact_not_registered',
  'fomo.oauth.refreshed',
  'fomo.oauth.refresh_failed',
  'fomo.db.connection_error',
  'fomo.sendblue.delivery_gap_detected'
] as const;
const auditActionSet = new Set(FOMO_AUDIT_ACTIONS);
const missingAudits = requiredAuditActions.filter((a) => !auditActionSet.has(a));
if (missingAudits.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `v0.5.3 hardening audit actions missing from registry (still required for v0.5.4): ${missingAudits.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('sendblue_contact_status')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message: "v0.5.3 invariant (carried into v0.5.4): 'sendblue_contact_status' must be registered in MEMORY_SIGNAL_KINDS."
  });
}
if (!memorySignalSet.has('stop_active')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message: "v0.5.4 cross-tenant invariant: 'stop_active' must be registered (Morris's row is what criteria 13 is diffed against)."
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.4-specific invariants                                             */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_4_FRIEND_BRIEFED',
  'true',
  'v0.5.4 hard gate (briefing-before-OAuth carries forward from v0.5.2 correction #2): the founder must brief Friend B out-of-band BEFORE this smoke starts. ' +
    'Set FOMO_V0_5_4_FRIEND_BRIEFED=true only after the briefing conversation (privacy copy, Gmail-readonly, STOP semantics, beta status, expected volume). ' +
    'This is an assertion, not something the system can verify — set it honestly.'
);

const friendName = (process.env.FOMO_V0_5_4_FRIEND_NAME ?? '').trim();
if (!friendName) {
  issues.push({
    name: 'FOMO_V0_5_4_FRIEND_NAME',
    severity: 'error',
    message:
      'FOMO_V0_5_4_FRIEND_NAME required (first name only — used in the SMOKE_REPORT for traceability). PII boundary: this is for the report header only; it is NEVER written to audit_log.'
  });
}

expectEquals(
  'FOMO_V0_5_4_BASELINE_CONFIRMED',
  'true',
  'v0.5.4 cross-tenant gate: the founder must run the runbook §0 baseline snapshot BEFORE the smoke starts. ' +
    'That snapshot records Morris\'s + founder\'s current `memory_signals` rows so criteria 13/14 (state UNTOUCHED) can be diffed against a known-good baseline. ' +
    'Set FOMO_V0_5_4_BASELINE_CONFIRMED=true only AFTER you have captured `psql ... -c "SELECT user_id, kind, jsonb_pretty(detail), updated_at FROM memory_signals WHERE kind=\'stop_active\'"` into the runbook §0.'
);

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.4                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({ name: 'FOMO_AUTO_SEND_ENABLED', severity: 'error', message: 'v0.5.4 hard boundary: founder review required. Set FOMO_AUTO_SEND_ENABLED=false (or unset).' });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message: 'BREVIO_DEV_MODE=true is a hard error in v0.5.4 — ephemeral per-process keys would un-bind Friend B\'s invite token after restart.'
  });
}

console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

const errors = issues.filter((i) => i.severity === 'error');
const warns = issues.filter((i) => i.severity === 'warn');

if (errors.length === 0) {
  if (warns.length > 0) {
    console.log(`! ${warns.length} warning(s):\n`);
    for (const w of warns) console.log(`  [WARN]  ${w.name}: ${w.message}`);
    console.log('');
  }
  console.log('✓ Preflight passed.');
  console.log('');
  console.log('  Next steps (see docs/smoke-test-v0.5.4-second-friend.md):');
  console.log('    1. §0 baseline snapshot already captured into the runbook (FOMO_V0_5_4_BASELINE_CONFIRMED=true)');
  console.log('    2. Friend B was briefed out-of-band (FOMO_V0_5_4_FRIEND_BRIEFED=true)');
  console.log('    3. Friend B\'s Gmail is added to Google Cloud Console test-user list');
  console.log('    4. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.4.log');
  console.log("    5. Mint Friend B invite: pnpm issue-friend-token -- --phone +1<friend-b-real-e164> --confirm-briefed yes-friend-was-briefed");
  console.log('    6. Send invite URL to Friend B on their own device');
  console.log('    7. Friend B completes /onboard with their own Gmail');
  console.log('    8. Friend B texts the Brevio number once (SendBlue Free Sandbox verification — same as Morris in v0.5.2)');
  console.log('    9. Friend B\'s FOMO-worthy email arrives → friend-safe Slack card');
  console.log('   10. Founder approves → real iMessage to Friend B');
  console.log('   11. Friend B replies STOP from real iMessage thread');
  console.log('   12. Concurrent founder regression (same window)');
  console.log('   13. pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4');
  console.log('   14. Fill in docs/SMOKE_REPORT_v0.5.4.md, including the §6 cross-tenant baseline diff');
  process.exit(0);
}

console.log(`✖ ${errors.length} required check(s) failed:\n`);
for (const e of errors) console.log(`  [ERROR] ${e.name}: ${e.message}`);
console.log('');
if (warns.length > 0) {
  console.log(`! ${warns.length} warning(s):\n`);
  for (const w of warns) console.log(`  [WARN]  ${w.name}: ${w.message}`);
  console.log('');
}
process.exit(1);
