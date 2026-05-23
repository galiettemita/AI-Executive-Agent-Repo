// Phase 3B.3 preflight — verifies every env var required for the
// founder's real-Gmail smoke test is set and well-formed BEFORE the
// server boots. Run via `pnpm smoke:preflight` (or via direct node
// invocation with --experimental-strip-types).
//
// Exit 0: all checks passed; founder can `pnpm dev` next.
// Exit 1: one or more required vars missing/invalid. Errors are
// surfaced as a checklist so the founder can fix all in one pass.
//
// No network calls. No DB calls. No tokens decrypted. Pure config
// inspection — safe to run as many times as needed.

import { GMAIL_READONLY_SCOPE } from '../src/adapters/gmail/client.js';
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
  // Accept base64 or hex: prefix. We measure decoded length.
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

console.log('Phase 3B.3 preflight — Gmail real smoke test\n');

/* ---------------------------------------------------------------------- */
/* Required env vars                                                      */
/* ---------------------------------------------------------------------- */

// Persistence (Neon Postgres path — production-like)
require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');

// At-rest encryption for OAuth tokens
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');

// OAuth state HMAC + session HMAC
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

// Google OAuth client
require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required from Google Cloud OAuth 2.0 Client');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required from Google Cloud OAuth 2.0 Client');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required — must exactly match the redirect URI configured in Google Cloud (e.g. http://localhost:8080/oauth/google/callback)'
);

// Polling explicitly enabled for the smoke test only
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  "Polling must be explicitly enabled for the smoke test. Set FOMO_GMAIL_POLLING_ENABLED=true"
);

// Cycle cap — Phase 3B.3 invariant. Worker must auto-stop after N cycles.
const cap = process.env.FOMO_GMAIL_POLLING_MAX_CYCLES;
if (!cap) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'error',
    message: 'Phase 3B.3 requires a bounded smoke window. Set FOMO_GMAIL_POLLING_MAX_CYCLES=1 (or 3).'
  });
} else if (!/^\d+$/.test(cap.trim()) || Number(cap) <= 0) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'error',
    message: `Cap must be a positive integer; got '${cap}'`
  });
}

/* ---------------------------------------------------------------------- */
/* Safety warnings (non-fatal but flagged)                                */
/* ---------------------------------------------------------------------- */

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message: "BREVIO_DEV_MODE=true bypasses production fail-closed checks. The smoke test should run with this UNSET so the same code path that runs in production is exercised."
  });
}

// Anything that looks like a non-readonly Gmail scope would be a Phase 3B.3
// violation. We don't read scopes from env (they're hardcoded in the
// adapter) but we double-print so the founder visually verifies.
console.log(`Hardcoded Gmail scope: ${GMAIL_READONLY_SCOPE}`);
console.log('(Phase 3B.3 invariant: NO other Gmail scopes may be added in this PR.)\n');

// Print the resolved kill-switch view so the founder can sanity-check.
const switches = loadKillSwitches(process.env);
console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

// Forbidden flips during the smoke test — fail loudly if any are on.
if (switches.send_enabled) {
  issues.push({
    name: 'FOMO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3B.3 non-goal: no SendBlue sends. Set FOMO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3B.3 non-goal: no auto-send. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3B.3 non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
  });
}

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.length === 0) {
  console.log('✓ Preflight passed. All required env vars are set and well-formed.');
  console.log('  Next steps (see docs/smoke-test-3b3-gmail.md):');
  console.log('    1. pnpm --filter @brevio/fomo run build');
  console.log('    2. pnpm --filter @brevio/fomo run dev');
  console.log('    3. open http://localhost:8080/oauth/google/start  (session auth required)');
  console.log('    4. after the bounded poll window, pnpm smoke:evidence');
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
