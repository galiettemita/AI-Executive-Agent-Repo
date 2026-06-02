// Phase v0.5.3 preflight — production-hardening smoke.
//
// Validates that the four v0.5.3 fixes are wired into the substrate
// BEFORE the founder runs the smoke. Pure config inspection — no DB,
// no network. The substrate runtime (loadKillSwitches, audit-action
// registry, memory-signal kind registry) is what makes this check
// load-bearing: if a future commit silently removes any of these
// without updating the registry, preflight catches it.
//
// PASS criteria:
//   - Same env shape as v0.5.2 (DATABASE_URL, crypto keys, friend
//     beta on, SendBlue creds with the canonical _ID / _KEY suffixes)
//   - 7 new audit actions registered in FOMO_AUDIT_ACTIONS
//   - 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
//
// Run before booting the dev server for the v0.5.3 smoke.

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

console.log('Phase v0.5.3 preflight — production-hardening smoke\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward                                                */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals('FOMO_FRIEND_BETA_ENABLED', 'true', 'Phase v0.5.3 reuses v0.5.x substrate; /onboard route must mount.');

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (OAuth refresh helper uses this).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required.');

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (contact registration + reconciliation).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_('SENDBLUE_WEBHOOK_SECRET', 'SENDBLUE_WEBHOOK_SECRET required.');

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required.');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on.');
expectEquals('FOMO_SLACK_REVIEW_ENABLED', 'true', 'Slack review must be on.');
expectEquals('FOMO_SEND_ENABLED', 'true', 'FOMO_SEND_ENABLED must be true (outbound gate tests need it).');
expectEquals('FOMO_SENDBLUE_INBOUND_ENABLED', 'true', 'FOMO_SENDBLUE_INBOUND_ENABLED must be true.');
expectEquals('FOMO_GMAIL_POLLING_ENABLED', 'true', 'FOMO_GMAIL_POLLING_ENABLED must be true (OAuth refresh test needs polling on).');
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');
require_('FOMO_FOUNDER_PHONE_NUMBER', 'FOMO_FOUNDER_PHONE_NUMBER required.');

/* ---------------------------------------------------------------------- */
/* v0.5.3 runtime registry invariants                                     */
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
    message: `v0.5.3 audit actions missing from registry: ${missingAudits.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('sendblue_contact_status')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message: "v0.5.3 invariant: 'sendblue_contact_status' must be registered in MEMORY_SIGNAL_KINDS."
  });
}

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.3                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({ name: 'FOMO_AUTO_SEND_ENABLED', severity: 'error', message: 'v0.5.3 hard boundary: founder review required.' });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({ name: 'BREVIO_DEV_MODE', severity: 'error', message: 'BREVIO_DEV_MODE=true is a hard error in v0.5.3 hardening smoke.' });
}

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
  console.log('  Next steps (see docs/smoke-test-v0.5.3-production-hardening.md):');
  console.log('    1. Boot dev server; confirm pg pool error handler attached');
  console.log('    2. Run alt-Gmail-as-friend smoke (v0.5.1 substrate path)');
  console.log('    3. Verify SendBlue contact auto-registered (memory_signals + audit)');
  console.log('    4. Force OAuth token expiry; verify auto-refresh kicks in');
  console.log('    5. Simulate Neon ECONNRESET via pool.emit; verify no crash');
  console.log('    6. Run pnpm ops:reconcile-sendblue; verify it finds + audits gaps');
  console.log('    7. Run pnpm smoke-evidence:v0.5.3');
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
