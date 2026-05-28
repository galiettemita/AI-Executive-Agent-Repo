// Phase 3F.2 preflight — verifies every env var required for the
// founder SendBlue inbound-reply smoke test BEFORE the server boots.
// Run via `pnpm preflight:3f2`.
//
// Extends the 3E.2 preflight: keeps the Gmail + ranker + Slack +
// outbound send requirements (the smoke needs the upstream chain
// to fire a real iMessage that the founder can text back to), then
// layers the 3F.1 inbound substrate requirements on top:
//   * FOMO_SENDBLUE_INBOUND_ENABLED=true (3F.2 invariant; route
//     mounted, parser dormant→active)
//   * SENDBLUE_WEBHOOK_SECRET — the secret you configured in the
//     SendBlue dashboard's webhook settings. SendBlue echoes this
//     verbatim in a request header on every inbound POST per docs
//     at docs.sendblue.com/getting-started/webhooks. Plain shared
//     secret, NOT HMAC.
//   * OPENAI_API_KEY — the reply parser's classifier uses OpenAI
//     to map soft intents (snooze / ignore / why / etc.).
//
// Optional but strongly recommended:
//   * SENDBLUE_WEBHOOK_SECRET_HEADER — overrides the default header
//     name `sb-signing-secret`. SendBlue's public docs don't name
//     the header explicitly; the runbook walks the founder through
//     observing it from a real webhook during smoke and patching
//     this env var without a code change if it differs.
//   * FOMO_OUTBOUND_MAX_CYCLES — bounded smoke window for the
//     outbound worker (proves STOP blocks future sends).
//   * FOMO_GMAIL_POLLING_MAX_CYCLES — bounded smoke window for
//     the polling worker (the smoke needs a fresh alert).
//
// Forbidden during 3F.2:
//   * FOMO_AUTO_SEND_ENABLED=true — manual approval only
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

console.log('Phase 3F.2 preflight — SendBlue inbound reply smoke test\n');

/* ---------------------------------------------------------------------- */
/* Required env vars — substrate (subset of 3E.2)                         */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

/* ---------------------------------------------------------------------- */
/* 3F.2-specific — SendBlue inbound requirements                          */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SENDBLUE_INBOUND_ENABLED',
  'true',
  'Phase 3F.2 invariant: inbound route MUST be mounted for this smoke run. Set FOMO_SENDBLUE_INBOUND_ENABLED=true'
);

require_(
  'SENDBLUE_WEBHOOK_SECRET',
  'SENDBLUE_WEBHOOK_SECRET required. This is the secret you configured in your SendBlue dashboard\'s ' +
    'webhook settings. SendBlue echoes it back verbatim in a request header on every inbound POST ' +
    '(per docs.sendblue.com/getting-started/webhooks). NOT an HMAC signing key — plain shared secret ' +
    'compared with timing-safe equality. Refusing to boot an unauthenticated inbound endpoint.'
);

const headerOverride = (process.env.SENDBLUE_WEBHOOK_SECRET_HEADER ?? '').trim();
if (!headerOverride) {
  issues.push({
    name: 'SENDBLUE_WEBHOOK_SECRET_HEADER',
    severity: 'warn',
    message:
      'SENDBLUE_WEBHOOK_SECRET_HEADER not set. The route will read `sb-signing-secret` (default — ' +
      'matches SendBlue\'s `sb-*` API-header naming pattern). LOAD-BEARING smoke step: observe the ' +
      'first real SendBlue POST during smoke and confirm this header carries the configured secret. ' +
      "If SendBlue uses a different header name, set this env var to the observed name and restart " +
      '— NO code change required. See the runbook (§4 Auth Mechanism Confirmation) for the exact step.'
  });
} else {
  issues.push({
    name: 'SENDBLUE_WEBHOOK_SECRET_HEADER',
    severity: 'warn',
    message:
      `SENDBLUE_WEBHOOK_SECRET_HEADER='${headerOverride}'. This overrides the default ` +
      "`sb-signing-secret`. Confirm during smoke (§4 of the runbook) that this is the header SendBlue " +
      'is ACTUALLY sending — observe a real webhook POST before trusting the override.'
  });
}

const publicUrl = (process.env.SENDBLUE_INBOUND_PUBLIC_URL ?? '').trim();
if (!publicUrl) {
  issues.push({
    name: 'SENDBLUE_INBOUND_PUBLIC_URL',
    severity: 'warn',
    message:
      "SENDBLUE_INBOUND_PUBLIC_URL not set. This is INFORMATIONAL only — the server doesn't read it, " +
      "but the runbook expects you to paste your ngrok / cloudflared public URL + '/sendblue/inbound' " +
      "into the SendBlue dashboard's webhook configuration. Setting this env var helps you track " +
      'which URL is currently configured.'
  });
} else if (!/^https:\/\//.test(publicUrl)) {
  issues.push({
    name: 'SENDBLUE_INBOUND_PUBLIC_URL',
    severity: 'warn',
    message: `SENDBLUE_INBOUND_PUBLIC_URL='${publicUrl.slice(0, 40)}...' should start with https:// (SendBlue requires HTTPS).`
  });
}

require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (reply parser classifier path uses OpenAI).');

/* ---------------------------------------------------------------------- */
/* 3E.2 carry-over (outbound send chain — needed for STOP-blocks-outbound) */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SEND_ENABLED',
  'true',
  'FOMO_SEND_ENABLED must be true so we can verify STOP enforcement blocks a future outbound send during the smoke.'
);
require_('SENDBLUE_API_KEY_ID', 'SENDBLUE_API_KEY_ID required (outbound chain still wired for the STOP-blocks test).');
require_('SENDBLUE_API_SECRET_KEY', 'SENDBLUE_API_SECRET_KEY required.');

const fromNumber = (process.env.SENDBLUE_FROM_NUMBER ?? '').trim();
if (!fromNumber || !/^\+\d{7,15}$/.test(fromNumber)) {
  issues.push({
    name: 'SENDBLUE_FROM_NUMBER',
    severity: 'error',
    message:
      `SENDBLUE_FROM_NUMBER missing or not in E.164 format (got '${fromNumber.slice(0, 4) || '<unset>'}...'). ` +
      'Required for any outbound send during the smoke.'
  });
}

const founderPhone = (process.env.FOMO_FOUNDER_PHONE_NUMBER ?? '').trim();
if (!founderPhone || !/^\+\d{7,15}$/.test(founderPhone)) {
  issues.push({
    name: 'FOMO_FOUNDER_PHONE_NUMBER',
    severity: 'error',
    message:
      `FOMO_FOUNDER_PHONE_NUMBER missing or not in E.164 format. The inbound route ALSO uses this as the ` +
      "from-number allowlist — webhooks from any other number get 403'd. The outbound-sender uses this as " +
      'the destination. Same value as 3E.2.'
  });
}

const founderUserId = (process.env.FOMO_FOUNDER_USER_ID ?? '').trim();
if (!founderUserId) {
  issues.push({
    name: 'FOMO_FOUNDER_USER_ID',
    severity: 'error',
    message:
      'FOMO_FOUNDER_USER_ID required. Inbound replies are attributed to this user_id; the outbound-sender ' +
      'uses it to enforce the founder-phone allowlist. Same value as 3E.2 (typically the literal string `founder`).'
  });
}

/* ---------------------------------------------------------------------- */
/* 3C.4 + 3D.2 carry-over (Gmail + ranker + Slack — needed upstream)      */
/* ---------------------------------------------------------------------- */

require_('FOMO_GMAIL_POLLING_ENABLED', 'FOMO_GMAIL_POLLING_ENABLED required (smoke needs a fresh alert from real Gmail).');
require_('FOMO_RANKER_ENABLED', 'FOMO_RANKER_ENABLED required.');
expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'FOMO_SLACK_REVIEW_ENABLED must be true so the founder approval click fires the queued_for_review → approved transition.'
);
expectPrefix('SLACK_BOT_TOKEN', 'xoxb-', 'SLACK_BOT_TOKEN required.');
require_('SLACK_FOUNDER_CHANNEL_ID', 'SLACK_FOUNDER_CHANNEL_ID required.');
require_('SLACK_SIGNING_SECRET', 'SLACK_SIGNING_SECRET required.');

/* ---------------------------------------------------------------------- */
/* Bounded smoke windows (recommended)                                    */
/* ---------------------------------------------------------------------- */

const switches = loadKillSwitches(process.env);

if (switches.polling_max_cycles === null) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'warn',
    message:
      'FOMO_GMAIL_POLLING_MAX_CYCLES not set. Recommend a small N (e.g. 30) so the polling worker ' +
      'auto-stops after a controlled window — gives ~5 minutes at 10s interval to send the test email + ' +
      'approve in Slack.'
  });
}
if (switches.outbound_max_cycles === null) {
  issues.push({
    name: 'FOMO_OUTBOUND_MAX_CYCLES',
    severity: 'warn',
    message:
      'FOMO_OUTBOUND_MAX_CYCLES not set. Recommend the SAME N as polling (e.g. 30) so the outbound ' +
      'worker auto-stops on the same timeline.'
  });
}

/* ---------------------------------------------------------------------- */
/* Forbidden during the smoke test                                        */
/* ---------------------------------------------------------------------- */

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message: 'BREVIO_DEV_MODE=true bypasses production fail-closed checks. Should be UNSET for the smoke.'
  });
}
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3F.2 non-goal: no auto-send. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3F.2 non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
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
  console.log('  Next steps (see docs/smoke-test-3f2-sendblue-inbound.md):');
  console.log('    1. ngrok http 8080  (T2)');
  console.log('    2. Configure SendBlue webhook URL + secret in dashboard');
  console.log('    3. pnpm --filter @brevio/fomo run build');
  console.log('    4. pnpm --filter @brevio/fomo run dev 2>&1 | tee /tmp/fomo-3f2.log');
  console.log('    5. **LOAD-BEARING:** observe the first SendBlue POST (ngrok inspector @ http://localhost:4040');
  console.log('       OR server-log inspection) → confirm auth header name + that header carries the literal secret');
  console.log('    6. Run the 5 smoke scenarios (trigger alert + approve, soft intent, STOP, idempotent retry,');
  console.log('       invalid-auth rejection via curl, optional START)');
  console.log('    7. pnpm smoke-evidence:3f2');
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
