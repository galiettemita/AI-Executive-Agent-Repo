// Phase 3D.2 preflight — verifies every env var required for the
// founder Slack approval-capture smoke test BEFORE the server boots.
// Run via `pnpm preflight:3d2`.
//
// Extends 3C.4 / 3D.1 preflights with the inbound interactivity
// requirements:
//   * FOMO_SLACK_REVIEW_ENABLED=true (kill switch ON for this run)
//   * SLACK_BOT_TOKEN (xoxb-...) — outbound chat.postMessage + chat.update
//   * SLACK_FOUNDER_CHANNEL_ID (C0123...) — only channel approve/reject
//     is accepted from
//   * SLACK_SIGNING_SECRET — verifies inbound /slack/interactivity
//   * SLACK_FOUNDER_USER_ID (U0123...) — RECOMMENDED but optional;
//     warned-on when missing
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

console.log('Phase 3D.2 preflight — Slack approval-capture smoke test\n');

/* ---------------------------------------------------------------------- */
/* Required env vars — substrate (subset of 3C.4)                         */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

/* ---------------------------------------------------------------------- */
/* Phase 3D.2-specific — Slack inbound requirements                       */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_SLACK_REVIEW_ENABLED',
  'true',
  'Phase 3D.2 invariant: Slack review MUST be on for this smoke run. Set FOMO_SLACK_REVIEW_ENABLED=true'
);

expectPrefix(
  'SLACK_BOT_TOKEN',
  'xoxb-',
  'SLACK_BOT_TOKEN required (Slack bot token from your app)'
);

const channelId = (process.env.SLACK_FOUNDER_CHANNEL_ID ?? '').trim();
if (!channelId) {
  issues.push({
    name: 'SLACK_FOUNDER_CHANNEL_ID',
    severity: 'error',
    message: 'SLACK_FOUNDER_CHANNEL_ID required (Slack channel id, like C0123ABCDEF, NOT a channel name)'
  });
} else if (!/^C[A-Z0-9]{8,}$/.test(channelId)) {
  issues.push({
    name: 'SLACK_FOUNDER_CHANNEL_ID',
    severity: 'warn',
    message: `SLACK_FOUNDER_CHANNEL_ID='${channelId}' does not look like a Slack channel id (expected C followed by 8+ uppercase alphanumerics). Channel names like '#fomo' won't work.`
  });
}

require_(
  'SLACK_SIGNING_SECRET',
  "SLACK_SIGNING_SECRET required (Slack app's signing secret from Basic Information). Used to verify inbound /slack/interactivity requests."
);

const founderUserId = (process.env.SLACK_FOUNDER_USER_ID ?? '').trim();
if (!founderUserId) {
  issues.push({
    name: 'SLACK_FOUNDER_USER_ID',
    severity: 'warn',
    message: 'SLACK_FOUNDER_USER_ID not set. The route will accept clicks from ANY user in the founder channel. Strongly recommended for production-shaped smoke testing — set to your Slack user id (U0123...).'
  });
} else if (!/^U[A-Z0-9]{8,}$/.test(founderUserId)) {
  issues.push({
    name: 'SLACK_FOUNDER_USER_ID',
    severity: 'warn',
    message: `SLACK_FOUNDER_USER_ID='${founderUserId}' does not look like a Slack user id (expected U followed by 8+ uppercase alphanumerics).`
  });
}

// The 3D.2 smoke test requires a publicly-reachable URL for Slack to
// POST interactivity payloads to. The preflight only verifies the env
// var is set as a reminder; it doesn't make a network call.
const publicUrl = (process.env.SLACK_INTERACTIVITY_PUBLIC_URL ?? '').trim();
if (!publicUrl) {
  issues.push({
    name: 'SLACK_INTERACTIVITY_PUBLIC_URL',
    severity: 'warn',
    message: "SLACK_INTERACTIVITY_PUBLIC_URL not set. This is INFORMATIONAL only — the server doesn't read it, but the runbook expects you to paste your ngrok / cloudflared public URL into the Slack app's Interactivity & Shortcuts panel. Setting this env var makes it easier to track which URL you configured."
  });
} else if (!/^https:\/\//.test(publicUrl)) {
  issues.push({
    name: 'SLACK_INTERACTIVITY_PUBLIC_URL',
    severity: 'warn',
    message: `SLACK_INTERACTIVITY_PUBLIC_URL='${publicUrl.slice(0, 40)}...' should start with https:// (Slack requires HTTPS).`
  });
}

/* ---------------------------------------------------------------------- */
/* 3D.1 carry-over env vars (just verify present)                         */
/* ---------------------------------------------------------------------- */

require_('FOMO_GMAIL_POLLING_ENABLED', 'FOMO_GMAIL_POLLING_ENABLED required (the smoke test relies on a real poll producing a real alert).');
require_('FOMO_RANKER_ENABLED', 'FOMO_RANKER_ENABLED required (the smoke test relies on a real rank → alert chain).');
require_('OPENAI_API_KEY', 'OPENAI_API_KEY required (the smoke test triggers a real ranker call).');

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

const switches = loadKillSwitches(process.env);
if (switches.send_enabled) {
  issues.push({
    name: 'FOMO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3D.2 non-goal: no SendBlue sends. Set FOMO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3D.2 non-goal: no auto-send. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3D.2 non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
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
  console.log('  Next steps (see docs/smoke-test-3d2-slack-approval.md):');
  console.log('    1. start ngrok or cloudflared tunneling localhost:8080');
  console.log('    2. paste the public URL + "/slack/interactivity" into Slack app → Interactivity & Shortcuts');
  console.log('    3. pnpm --filter @brevio/fomo run build');
  console.log('    4. pnpm --filter @brevio/fomo run dev');
  console.log('    5. trigger an alert (send yourself an important email; the polling worker should rank + post to Slack)');
  console.log('    6. click Approve on the Slack card');
  console.log('    7. pnpm smoke-evidence:3d2');
  console.log('    8. optionally do a Reject + an idempotent duplicate click');
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
