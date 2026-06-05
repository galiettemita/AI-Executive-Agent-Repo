// Phase v0.5.5 preflight — STOP Enforcement + Confirmation (founder-only smoke).
//
// SCAFFOLDING-COMMIT BOUNDARY (founder-locked 2026-06-04):
//   This script is part of the v0.5.5 SCAFFOLDING commit, which lands BEFORE
//   the runtime implementation. It is intentionally permissive about the four
//   v0.5.5-NEW audit kinds:
//     - fomo.sendblue.stop_confirmation_sent
//     - fomo.sendblue.stop_confirmation_failed
//     - fomo.alert.suppressed_stop_active
//     - fomo.poll.skipped_stop_active
//   These are EXPECTED OUTPUTS of the future runtime commit, not
//   already-existing behaviour. While the runtime commit is pending, these
//   kinds will be absent from FOMO_AUDIT_ACTIONS and preflight will WARN
//   (severity 'warn', exit code 0). Once the runtime commit lands and
//   registers them, the warns disappear automatically.
//
// Pure config inspection — no DB, no network. Validates that the substrate
// is the v0.5.2 + v0.5.3 + v0.5.4 shape (so the founder smoke runs against a
// known-good substrate) AND that the v0.5.5-specific founder gate is set:
//
//   * v0.5.2/v0.5.3/v0.5.4 carry-forward: FOMO_FRIEND_BETA_ENABLED=true, real
//     founder phone, HTTPS ngrok base URL (SendBlue webhook delivery needs a
//     public URL even though no friend is involved this phase), ≥ 60 cycle
//     caps on both workers, all v0.5.3 hardening kinds still registered.
//   * v0.5.5-specific:
//     - FOMO_V0_5_5_BASELINE_CONFIRMED=true (founder asserts the runbook §0
//       baseline snapshot was captured BEFORE the smoke — criteria C7 cross-
//       tenant diff is computed against this baseline).
//     - FOMO_V0_5_5_WINDOW_HOURS (smoke-window for evidence queries; default
//       24, override only if you need a wider window because the smoke ran
//       across multiple sessions).
//
// Forbidden in v0.5.5 (same as v0.5.2/v0.5.3/v0.5.4):
//   * FOMO_AUTO_SEND_ENABLED=true
//   * BREVIO_DEV_MODE=true (ephemeral keys break the founder's existing
//     OAuth tokens; v0.5.5 needs the real founder OAuth grant alive across
//     restarts so STOP/START round-trips are deterministic).
//
// The runbook (docs/smoke-test-v0.5.5-stop-enforcement.md) covers the
// out-of-band requirements preflight cannot check:
//   * Baseline snapshot of stop_active rows captured
//   * Founder iPhone is the device sending STOP/START
//   * ngrok is forwarding SendBlue webhooks to localhost:8080
//   * No friend is involved this phase (the three-friend cap holds —
//     Friend B was the last GUARANTEED smoke).

import { loadKillSwitches } from '../src/core/kill-switches.js';
import { FOMO_AUDIT_ACTIONS } from '../src/core/audit.js';
import { MEMORY_SIGNAL_KINDS } from '../src/memory/memory-signals.js';

type Severity = 'error' | 'warn';
interface Check {
  readonly name: string;
  readonly severity: Severity;
  readonly message: string;
}
const issues: Check[] = [];

function require_(name: string, message: string): void {
  if (!(process.env[name] ?? '').trim()) issues.push({ name, severity: 'error', message });
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
function checkCycleMin(name: string, min: number, ctx: string): void {
  const raw = (process.env[name] ?? '').trim();
  if (!raw) {
    issues.push({
      name,
      severity: 'error',
      message: `${name} required for v0.5.5 (${ctx}). Set ≥ ${min} explicitly.`
    });
    return;
  }
  const n = Number(raw);
  if (!Number.isFinite(n) || n < min) {
    issues.push({
      name,
      severity: 'error',
      message: `${name}=${raw} is below the v0.5.5 minimum of ${min}. ${ctx}`
    });
  }
}

console.log('Phase v0.5.5 preflight — STOP Enforcement + Confirmation (founder-only smoke)\n');

/* ---------------------------------------------------------------------- */
/* Substrate carry-forward (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4)            */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required.');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');
requireMin('BREVIO_PHONE_HASH_KEY', 32, 'BREVIO_PHONE_HASH_KEY must be a 32-byte key');

expectEquals(
  'FOMO_FRIEND_BETA_ENABLED',
  'true',
  'v0.5.5 substrate still requires the friend-beta kill switch ON (carry-forward; even though no friend is involved this phase, the existing v0.5.4 substrate stays live).'
);

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (founder Gmail polling continues).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required. Confirm /oauth/google/callback is registered at the public HTTPS URL.'
);

require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (STOP confirmation send).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');
require_('SENDBLUE_FROM_NUMBER', 'SENDBLUE_FROM_NUMBER required.');
require_(
  'SENDBLUE_WEBHOOK_SECRET',
  'SENDBLUE_WEBHOOK_SECRET required (founder STOP/START inbound webhook).'
);

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker + reply parser).');
expectEquals('FOMO_RANKER_ENABLED', 'true', 'Ranker must be on (founder regression flow C11).');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Slack review must be on (founder regression flow C11).'
);
expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so the STOP confirmation outbound can fire AND so the founder regression alert can fire on approval.'
);
expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'FOMO_SENDBLUE_INBOUND_ENABLED must be true so the founder STOP/START iMessages are received.'
);
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'FOMO_GMAIL_POLLING_ENABLED must be true so the polling-after-STOP suppression (C6) can be observed.'
);
require_('SLACK_BOT_TOKEN', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');
require_('FOMO_FOUNDER_USER_ID', 'FOMO_FOUNDER_USER_ID required.');

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message:
      'FOMO_FOUNDER_PHONE_NUMBER required for the founder STOP/START smoke (this IS the device that sends and receives during v0.5.5).'
  });
} else if (/^\+1555010\d{4}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='+1555010xxxx' is NANPA-reserved fictional. v0.5.5 requires a real founder phone — STOP confirmations are sent here.`
  });
}

const friendBaseUrl = (process.env.FOMO_FRIEND_BETA_BASE_URL ?? '').trim();
if (!friendBaseUrl) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message:
      'FOMO_FRIEND_BETA_BASE_URL required. Even though no friend is involved this phase, SendBlue webhook delivery (for the founder STOP/START iMessages) requires a public HTTPS URL — keep the v0.5.4 ngrok URL pointed at localhost:8080.'
  });
} else if (!/^https:\/\//.test(friendBaseUrl)) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_BASE_URL',
    severity: 'error',
    message: `FOMO_FRIEND_BETA_BASE_URL='${friendBaseUrl.slice(0, 60)}...' must start with https:// for SendBlue webhook delivery.`
  });
}

checkCycleMin(
  'FOMO_GMAIL_POLLING_MAX_CYCLES',
  60,
  'STOP-enforcement smoke needs the polling worker live long enough to (a) observe the founder regression alert and (b) verify polling-after-STOP suppression across multiple cycles.'
);
checkCycleMin(
  'FOMO_OUTBOUND_MAX_CYCLES',
  60,
  'STOP-enforcement smoke needs the outbound worker live long enough to send the founder regression iMessage and the STOP confirmation reply.'
);

/* ---------------------------------------------------------------------- */
/* v0.5.3 hardening registry invariants (carried forward)                 */
/* ---------------------------------------------------------------------- */

const requiredHardeningActions = [
  'fomo.sendblue.contact_registered',
  'fomo.sendblue.contact_registration_failed',
  'fomo.send.contact_not_registered',
  'fomo.oauth.refreshed',
  'fomo.oauth.refresh_failed',
  'fomo.db.connection_error',
  'fomo.sendblue.delivery_gap_detected'
] as const;
// Strictly typed: FOMO_AUDIT_ACTIONS is a `readonly AuditAction[]`. The 4
// v0.5.5 kinds are now registered (runtime commit landed); the type system
// guarantees they are members of the union, so the .has() calls below resolve
// against the strict union without any widening. If anyone removes one of
// the 4 v0.5.5 kinds from FOMO_AUDIT_ACTIONS in a future PR, tsc fails here
// — exactly the protection the founder asked for in the v0.5.5 runtime
// directive: "After runtime lands, remove/avoid any loose string-cast
// workaround that hides missing audit action registration."
const auditActionSet = new Set(FOMO_AUDIT_ACTIONS);
const missingHardeningActions = requiredHardeningActions.filter((a) => !auditActionSet.has(a));
if (missingHardeningActions.length > 0) {
  issues.push({
    name: 'FOMO_AUDIT_ACTIONS',
    severity: 'error',
    message: `v0.5.3 hardening audit actions missing from registry (still required for v0.5.5): ${missingHardeningActions.join(', ')}`
  });
}

const memorySignalSet = new Set(MEMORY_SIGNAL_KINDS as readonly string[]);
if (!memorySignalSet.has('stop_active')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message:
      "v0.5.5 hard requirement: 'stop_active' memory_signal kind must be registered — this is what STOP enforcement keys off."
  });
}
if (!memorySignalSet.has('sendblue_contact_status')) {
  issues.push({
    name: 'MEMORY_SIGNAL_KINDS',
    severity: 'error',
    message:
      "v0.5.3 invariant carried into v0.5.5: 'sendblue_contact_status' must be registered (founder's contact-status row is read by the outbound layer before sending STOP confirmation)."
  });
}

/* ---------------------------------------------------------------------- */
/* v0.5.5-specific operator gate                                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_V0_5_5_BASELINE_CONFIRMED',
  'true',
  'v0.5.5 cross-tenant gate: founder must run the runbook §0 baseline snapshot BEFORE the smoke starts. ' +
    'That snapshot records the current `memory_signals.stop_active` rows so criterion C7 (cross-tenant isolation) can be diffed against a known-good baseline. ' +
    'Set FOMO_V0_5_5_BASELINE_CONFIRMED=true only AFTER you have captured `psql ... -c "SELECT user_id, kind, jsonb_pretty(detail), updated_at FROM memory_signals WHERE kind=\'stop_active\' ORDER BY user_id"` into /tmp/v0.5.5-baseline-stop-active.txt.'
);

const windowHours = (process.env.FOMO_V0_5_5_WINDOW_HOURS ?? '').trim();
if (!windowHours) {
  issues.push({
    name: 'FOMO_V0_5_5_WINDOW_HOURS',
    severity: 'warn',
    message:
      'FOMO_V0_5_5_WINDOW_HOURS not set; smoke-evidence script will default to 24 hours. Override only if the smoke runs across multiple sessions and you need a wider window.'
  });
} else {
  const n = Number(windowHours);
  if (!Number.isFinite(n) || n < 1 || n > 168) {
    issues.push({
      name: 'FOMO_V0_5_5_WINDOW_HOURS',
      severity: 'error',
      message: `FOMO_V0_5_5_WINDOW_HOURS=${windowHours} is outside the sensible range (1–168). Set 24 by default or unset.`
    });
  }
}

/* ---------------------------------------------------------------------- */
/* v0.5.5-NEW audit kinds — PENDING runtime commit                        */
/* ---------------------------------------------------------------------- */
/*
 * These four kinds are EXPECTED runtime outputs of the future v0.5.5
 * implementation commit. While the runtime commit is pending, they will be
 * absent from FOMO_AUDIT_ACTIONS. Report each as a WARN with a clear
 * "PENDING runtime commit" message so the founder is not surprised when
 * smoke-evidence:v0.5.5 reports them as PENDING too.
 *
 * Once the runtime commit lands and adds them to FOMO_AUDIT_ACTIONS, these
 * warns disappear automatically.
 */
const expectedV055NewActions = [
  'fomo.sendblue.stop_confirmation_sent',
  'fomo.sendblue.stop_confirmation_failed',
  'fomo.alert.suppressed_stop_active',
  'fomo.poll.skipped_stop_active'
] as const;
const pendingV055Actions = expectedV055NewActions.filter((a) => !auditActionSet.has(a));
if (pendingV055Actions.length > 0) {
  for (const a of pendingV055Actions) {
    issues.push({
      name: 'FOMO_AUDIT_ACTIONS',
      severity: 'warn',
      message: `v0.5.5 expected audit kind PENDING runtime commit: '${a}'. This is normal at scaffolding time — the kind is registered by the future runtime implementation commit, not this scaffolding commit.`
    });
  }
}

/* ---------------------------------------------------------------------- */
/* Forbidden in v0.5.5                                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'v0.5.5 hard boundary: founder Slack review still required for FOMO alerts. STOP confirmation is the ONLY outbound that does not pass through Slack review (it is a system-initiated courtesy reply). Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'error',
    message:
      'BREVIO_DEV_MODE=true is a hard error in v0.5.5 — ephemeral per-process keys would invalidate the founder OAuth tokens between restarts, breaking the polling-after-STOP suppression test (C6).'
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
    console.log(`! ${warns.length} warning(s) (non-blocking):\n`);
    for (const w of warns) console.log(`  [WARN]  ${w.name}: ${w.message}`);
    console.log('');
  }
  console.log('✓ Preflight passed.');
  console.log('');
  console.log('  Next steps (see docs/smoke-test-v0.5.5-stop-enforcement.md):');
  console.log('    1. §0 baseline snapshot already captured (FOMO_V0_5_5_BASELINE_CONFIRMED=true)');
  console.log('    2. Boot dev server: pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.5.log');
  console.log('    3. ngrok forwarding to localhost:8080 (same URL as v0.5.4)');
  console.log('    4. Run §6 Test 1 (founder STOP → confirmation received)');
  console.log('    5. Run §6 Test 2 (duplicate STOP within 24h → no second confirmation)');
  console.log('    6. Run §6 Test 3 (founder START → alerts re-enabled)');
  console.log('    7. Run §6 Test 4 (induced SendBlue failure → confirmation_failed audit row, no retry)');
  console.log('    8. Run §6 Test 5 (cross-tenant — other users untouched)');
  console.log('    9. Run all 5 evidence scripts: pnpm smoke-evidence:v0.5.1 && pnpm smoke-evidence:v0.5.2 && pnpm smoke-evidence:v0.5.3 && pnpm smoke-evidence:v0.5.4 && pnpm smoke-evidence:v0.5.5');
  console.log('   10. Cross-tenant diff (baseline vs post stop_active)');
  console.log('   11. Fill in docs/SMOKE_REPORT_v0.5.5.md');
  console.log('');
  if (pendingV055Actions.length > 0) {
    console.log(
      `  NOTE: ${pendingV055Actions.length} v0.5.5 audit kind(s) are PENDING runtime commit. Smoke-evidence will report C1–C10 as PENDING until the runtime implementation lands. This is expected at scaffolding time.`
    );
    console.log('');
  }
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
