// Phase 3G preflight — verifies every env var required for the
// founder Full Demo Smoke Test BEFORE the server boots.
// Run via `pnpm preflight:3g`.
//
// 3G is the v0.1 milestone: ONE email flows end-to-end through every
// piece of the substrate against real providers — Gmail → ranker →
// Slack approval → SendBlue outbound text → SendBlue inbound reply →
// state transitions + feedback + memory writes. It is a SUPERSET of
// the 3F.2 requirements (same env shape, same kill switches), because
// 3G exercises every prior phase's gate path in one continuous run.
//
// 3G-specific reminders (preflight cannot enforce these — they're
// runtime / DB state, not env config):
//   * Gmail must be re-authed for the founder user BEFORE this run.
//     3F.2 left `oauth_tokens.needs_reauth=true` for `founder`, so
//     polling was skipping the user. Verify with:
//       psql "$DATABASE_URL" -P pager=off -c \
//         "SELECT user_id, needs_reauth FROM oauth_tokens WHERE user_id='founder';"
//     Expect `needs_reauth = f`. If `t`, run the OAuth flow before
//     starting the smoke.
//   * Send yourself the FOMO-worthy test email AFTER the server starts
//     polling — the runbook walks the exact subject/body that lands a
//     classifier `label=important` reliably.
//
// Forbidden during 3G (same as every prior gate):
//   * FOMO_AUTO_SEND_ENABLED=true  — manual approval only
//   * FOMO_FRIEND_BETA_ENABLED=true — founder-only
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

console.log('Phase 3G preflight — Full Founder Demo Smoke Test (v0.1 milestone)\n');

/* ---------------------------------------------------------------------- */
/* Substrate (DATABASE_URL + crypto keys)                                 */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

/* ---------------------------------------------------------------------- */
/* Gmail + OAuth (3B.3 carry-over)                                        */
/* ---------------------------------------------------------------------- */

require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required (Gmail OAuth callback).');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required.');
require_('BREVIO_OAUTH_REDIRECT_URI_GOOGLE', 'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required (must match the OAuth client config).');
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  '3G demo invariant: Gmail polling MUST be on so a real email triggers the chain.'
);

/* ---------------------------------------------------------------------- */
/* Ranker (3C.4 carry-over)                                               */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_RANKER_ENABLED',
  'true',
  '3G demo invariant: ranker MUST score the test email and label it `important`.'
);
require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (ranker + reply parser use OpenAI).');

/* ---------------------------------------------------------------------- */
/* Slack review (3D.2 carry-over)                                         */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  '3G demo invariant: founder approval click drives queued_for_review → approved.'
);
expectPrefix('SLACK_BOT_TOKEN', 'xoxb-', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required (verifies the inbound Slack callback).');
require_(
  'SLACK_FOUNDER_USER_ID',
  'SLACK_FOUNDER_USER_ID required — defense-in-depth: only this Slack user may approve. ' +
    'Same value as 3D.2 PASS.'
);

/* ---------------------------------------------------------------------- */
/* SendBlue outbound (3E.2 carry-over)                                    */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  '3G demo invariant: approved alert MUST fire a real iMessage to founder phone.'
);
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required.');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');

const fromNumber = (process.env.SENDBLUE_FROM_NUMBER ?? '').trim();
if (!fromNumber || !/^\+\d{7,15}$/.test(fromNumber)) {
  issues.push({
    name: 'SENDBLUE_FROM_NUMBER',
    severity: 'error',
    message:
      `SENDBLUE_FROM_NUMBER missing or not in E.164 format (got '${fromNumber.slice(0, 4) || '<unset>'}...'). ` +
      'Required for outbound send.'
  });
}

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone || !/^\+\d{7,15}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message:
      `FOMO_FOUNDER_PHONE_NUMBER missing or not in E.164 format. The inbound route uses this as the ` +
      "from-number allowlist; the outbound-sender uses it as the destination. Same value as 3E.2 / 3F.2."
  });
}

const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
if (!founderUserId) {
  issues.push({
    name: 'FOMO_FOUNDER_USER_ID',
    severity: 'error',
    message:
      'FOMO_FOUNDER_USER_ID required. Same value as 3E.2 / 3F.2 (typically the literal string `founder`).'
  });
}

/* ---------------------------------------------------------------------- */
/* SendBlue inbound (3F.2 carry-over — the reply-back step of the demo)   */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  '3G demo invariant: founder texts back during the demo, so the inbound route MUST be mounted.'
);
require_(
  'SENDBLUE_WEBHOOK_SECRET',
  'SENDBLUE_WEBHOOK_SECRET required (plain shared secret in `sb-signing-secret` header — Scenario A ' +
    'confirmed by 3F.2 PASS report §5).'
);

const headerOverride = (process.env.SENDBLUE_WEBHOOK_SECRET_HEADER ?? '').trim();
if (headerOverride && headerOverride !== 'sb-signing-secret') {
  issues.push({
    name: 'SENDBLUE_WEBHOOK_SECRET_HEADER',
    severity: 'warn',
    message:
      `SENDBLUE_WEBHOOK_SECRET_HEADER='${headerOverride}' overrides the default ` +
      "`sb-signing-secret` confirmed by 3F.2. Confirm this is still the header SendBlue sends; " +
      'if SendBlue changed its scheme, the runbook §0 calls for a fresh ngrok-inspector check.'
  });
}

const publicUrl = (process.env.SENDBLUE_INBOUND_PUBLIC_URL ?? '').trim();
if (!publicUrl) {
  issues.push({
    name: 'SENDBLUE_INBOUND_PUBLIC_URL',
    severity: 'warn',
    message:
      "SENDBLUE_INBOUND_PUBLIC_URL not set. Informational only — paste your ngrok URL + '/sendblue/inbound' " +
      "into the SendBlue dashboard's webhook configuration before starting the demo."
  });
} else if (!/^https:\/\//.test(publicUrl)) {
  issues.push({
    name: 'SENDBLUE_INBOUND_PUBLIC_URL',
    severity: 'warn',
    message: `SENDBLUE_INBOUND_PUBLIC_URL='${publicUrl.slice(0, 40)}...' should start with https://`
  });
}

/* ---------------------------------------------------------------------- */
/* Bounded smoke windows (strongly recommended)                           */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);

if (switches.polling_max_cycles === null) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'warn',
    message:
      'FOMO_GMAIL_POLLING_MAX_CYCLES not set. Recommend a small N (e.g. 60) so the polling worker ' +
      'auto-stops after a controlled window — gives ~10 minutes at 10s interval to send the test email, ' +
      'approve in Slack, get the SendBlue text, reply, and observe state writes.'
  });
}
if (switches.outbound_max_cycles === null) {
  issues.push({
    name: 'FOMO_OUTBOUND_MAX_CYCLES',
    severity: 'warn',
    message:
      'FOMO_OUTBOUND_MAX_CYCLES not set. Recommend the SAME N as polling (e.g. 60) so the outbound ' +
      'worker auto-stops on the same timeline.'
  });
}

/* ---------------------------------------------------------------------- */
/* Forbidden during the demo                                              */
/* ---------------------------------------------------------------------- */

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message: 'BREVIO_DEV_MODE=true bypasses production fail-closed checks. Should be UNSET for the demo.'
  });
}
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3G non-goal: manual approval only. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3G non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
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
  console.log('✓ Preflight passed. All required env vars are set and well-formed.');
  console.log('');
  console.log('  REMINDERS (preflight cannot check these — they are runtime / DB state):');
  console.log('    a. Gmail re-auth: verify oauth_tokens.needs_reauth = false for `founder` BEFORE');
  console.log('       starting. 3F.2 left it true. The runbook §1 walks through the OAuth flow.');
  console.log('    b. Database migrations: confirm every migration in apps/fomo/src/db/migrations/');
  console.log('       is applied to Neon. 3F.2 hit a 500-error wall when 0004 was PGlite-only.');
  console.log('       The runbook §0 includes the verification + apply commands.');
  console.log('    c. ngrok URL + SendBlue dashboard webhook URL match (transient ngrok subdomain).');
  console.log('');
  console.log('  Next steps (see docs/smoke-test-3g-full-demo.md):');
  console.log('    1. Re-auth Gmail (founder OAuth flow)');
  console.log('    2. ngrok http 8080 (T2)');
  console.log('    3. Update SendBlue webhook URL in dashboard');
  console.log('    4. pnpm --filter @brevio/fomo run build');
  console.log('    5. pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3g.log');
  console.log('    6. Send yourself the FOMO-worthy test email (subject + body in runbook §4)');
  console.log('    7. Watch the chain fire: poll → rank → Slack → approve → SendBlue → text back');
  console.log('    8. Reply `tomorrow` to the SendBlue iMessage');
  console.log('    9. pnpm smoke-evidence:3g');
  console.log('   10. Fill in docs/SMOKE_REPORT_3G.md + commit + merge → v0.1 done');
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
