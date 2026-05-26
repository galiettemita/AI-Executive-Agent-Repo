// Phase 3E.2 preflight — verifies every env var required for the
// founder SendBlue real-iMessage smoke test BEFORE the server boots.
// Run via `pnpm preflight:3e2`.
//
// Extends the 3D.2 preflight: keeps the Gmail + ranker + Slack
// requirements (since the chain still needs to produce a real
// approved alert), then layers the 3E.1 send substrate requirements
// on top:
//   * FOMO_SEND_ENABLED=true (kill switch ON for this run — gates
//     the policy decision and starts the outbound-sender worker)
//   * SENDBLUE_API_KEY_ID — from SendBlue dashboard
//   * SENDBLUE_API_SECRET_KEY — from SendBlue dashboard
//   * FOMO_FOUNDER_PHONE_NUMBER — E.164 format, e.g. +14155551234.
//     This is the ONLY phone the outbound-sender worker is allowed
//     to text. Defense-in-depth founder-phone allowlist.
//   * FOMO_FOUNDER_USER_ID — the user_id whose alerts the worker
//     may text. `destinationFor(user_id)` returns the founder phone
//     ONLY for this id; null for everyone else.
//   * FOMO_OUTBOUND_MAX_CYCLES — RECOMMENDED for the smoke run.
//     Caps the outbound worker to N cycles so it cannot keep firing
//     real iMessages during the smoke window.
//
// Forbidden during 3E.2:
//   * FOMO_AUTO_SEND_ENABLED=true — 3E.2 is MANUAL only; auto-send
//     waits on a later phase that has its own threshold logic
//   * FOMO_FRIEND_BETA_ENABLED=true — founder-only smoke
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

function expectPrefix(name: string, prefix: string, message: string): void {
  const v = (process.env[name] ?? '').trim();
  if (!v) {
    issues.push({ name, severity: 'error', message: `${message} (missing)` });
    return;
  }
  if (!v.startsWith(prefix)) {
    issues.push({
      name,
      severity: 'error',
      message: `${message} — expected to start with '${prefix}', got '${v.slice(0, prefix.length + 2)}...'`
    });
  }
}

console.log('Phase 3E.2 preflight — SendBlue founder real-iMessage smoke test\n');

/* ---------------------------------------------------------------------- */
/* Required env vars — substrate (subset of 3D.2)                         */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

/* ---------------------------------------------------------------------- */
/* 3E.2-specific — SendBlue real-send requirements                        */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'Phase 3E.2 invariant: outbound-sender MUST be on for this smoke run. Set FOMO_SEND_ENABLED=true'
);

require_(
  'SENDBLUE_API_KEY_ID',
  'SENDBLUE_API_KEY_ID required (from your SendBlue account dashboard). The outbound-sender worker authenticates with this id + secret pair.'
);

require_(
  'SENDBLUE_API_SECRET_KEY',
  'SENDBLUE_API_SECRET_KEY required (from your SendBlue account dashboard).'
);

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message:
      'FOMO_FOUNDER_PHONE_NUMBER required (E.164 format, e.g. +14155551234). ' +
      'This is the ONLY phone the outbound-sender is allowed to text — defense-in-depth ' +
      'founder allowlist. The worker refuses to dispatch without it.'
  });
} else if (!/^\+\d{7,15}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message: `FOMO_FOUNDER_PHONE_NUMBER='${founderPhone.slice(0, 4)}...' is not in E.164 format. Expected '+' followed by 7-15 digits, e.g. '+14155551234'.`
  });
}

const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
if (!founderUserId) {
  issues.push({
    name: 'FOMO_FOUNDER_USER_ID',
    severity: 'error',
    message:
      'FOMO_FOUNDER_USER_ID required (the user_id whose alerts the outbound-sender is ' +
      'allowed to text). Defense-in-depth allowlist: destinationFor(user_id) returns the ' +
      'founder phone ONLY for this id and null for everyone else. Use the same user_id ' +
      'you completed Google OAuth as.'
  });
}

const switches = loadKillSwitches(process.env);

if (switches.outbound_max_cycles === null) {
  issues.push({
    name: 'FOMO_OUTBOUND_MAX_CYCLES',
    severity: 'warn',
    message:
      'FOMO_OUTBOUND_MAX_CYCLES not set. The 3E.2 runbook strongly recommends setting ' +
      "this to a small N (e.g. '1' or '2') so the outbound-sender worker auto-stops after " +
      'a controlled window. Without the cap, an approved alert that fails ambiguously could ' +
      'leave the worker spinning. (Idempotency still protects you — second cycle finds zero ' +
      'approved alerts — but the cap is a cheap belt-and-suspenders for a real-send smoke.)'
  });
} else if (switches.outbound_max_cycles > 5) {
  issues.push({
    name: 'FOMO_OUTBOUND_MAX_CYCLES',
    severity: 'warn',
    message: `FOMO_OUTBOUND_MAX_CYCLES=${switches.outbound_max_cycles}. That seems high for a smoke test. The runbook suggests 1-3 so you can confirm a single iMessage delivery.`
  });
}

/* ---------------------------------------------------------------------- */
/* 3B.3 + 3C.4 + 3D.2 carry-over env vars                                 */
/* ---------------------------------------------------------------------- */

require_(
  'FOMO_GMAIL_POLLING_ENABLED',
  'FOMO_GMAIL_POLLING_ENABLED required (the smoke test relies on a real poll producing a real alert).'
);
require_(
  'FOMO_RANKER_ENABLED',
  'FOMO_RANKER_ENABLED required (the smoke test relies on a real rank → alert chain).'
);
require_(
  'OPENAI_API_KEY',
  'OPENAI_API_KEY required (the smoke test triggers a real ranker call).'
);
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'FOMO_SLACK_REVIEW_ENABLED must be true so the Slack approval click can fire the queued_for_review → approved transition the outbound-sender consumes.'
);
expectPrefix(
  'SLACK_BOT_TOKEN',
  'xoxb-',
  'SLACK_BOT_TOKEN required (bot token from your Slack app — needed for the approval flow that feeds the outbound-sender).'
);
require_(
  'SLACK_FOUNDER_CHANNEL_ID',
  'SLACK_FOUNDER_CHANNEL_ID required (the channel where the approval click happens upstream of the send).'
);
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');

/* ---------------------------------------------------------------------- */
/* Forbidden during the smoke test                                        */
/* ---------------------------------------------------------------------- */

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message: 'BREVIO_DEV_MODE=true bypasses production fail-closed checks. The smoke test should run with this UNSET.'
  });
}

if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message:
      'Phase 3E.2 non-goal: no auto-send. 3E.2 only proves the MANUAL approval → send path. ' +
      'Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3E.2 non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
  });
}

console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.length === 0) {
  console.log('✓ Preflight passed. All required env vars are set and well-formed.');
  console.log('  Next steps (see docs/smoke-test-3e2-sendblue-outbound.md):');
  console.log('    1. pnpm --filter @brevio/fomo run build');
  console.log('    2. pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3e2.log');
  console.log('    3. (in another terminal) start ngrok / cloudflared for /slack/interactivity');
  console.log('    4. trigger an alert (send yourself an important email)');
  console.log('    5. click Approve on the Slack card');
  console.log('    6. confirm: the outbound-sender worker sends ONE real iMessage to your phone');
  console.log('    7. confirm: a second cycle does NOT re-send (idempotency proof)');
  console.log('    8. pnpm smoke-evidence:3e2');
  process.exit(0);
}

const errors = issues.filter((i) => i.severity === 'error');
const warns = issues.filter((i) => i.severity === 'warn');

if (errors.length > 0) {
  console.log(`✖ ${errors.length} required check(s) failed:\n`);
  for (const e of errors) {
    console.log(`  [ERROR] ${e.name}: ${e.message}`);
  }
  console.log('');
}
if (warns.length > 0) {
  console.log(`! ${warns.length} warning(s):\n`);
  for (const w of warns) {
    console.log(`  [WARN]  ${w.name}: ${w.message}`);
  }
  console.log('');
}

process.exit(errors.length > 0 ? 1 : 0);
