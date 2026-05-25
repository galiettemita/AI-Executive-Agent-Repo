// Phase 3C.4 preflight — verifies every env var required for the founder
// real-Gmail + real-OpenAI ranker smoke test is set and well-formed
// BEFORE the server boots. Run via `pnpm preflight:3c4`.
//
// 3C.4 is the first smoke run where the FULL chain fires for real:
//   real Gmail → polling worker → gmail.read dispatch → real OpenAI
//   ranker call → rank_results row → audit
//
// Extends Phase 3B.3 preflight with the ranker-specific requirements:
//   * FOMO_RANKER_ENABLED=true (the kill switch must be ON for this run)
//   * OPENAI_API_KEY present (else buildRankerDep throws at boot)
//   * FOMO_OPENAI_MODEL optional (default gpt-5-mini, the model 3C.2
//     validated against the 20 synthetic fixtures)
//
// Exit 0: all checks passed; founder can `pnpm dev` next.
// Exit 1: one or more required vars missing/invalid. Errors are
// surfaced as a checklist so the founder can fix all in one pass.
//
// No network. No DB. No tokens decrypted. Pure config inspection.

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

console.log('Phase 3C.4 preflight — Gmail real + OpenAI ranker smoke test\n');

/* ---------------------------------------------------------------------- */
/* Required env vars (3B.3 substrate)                                     */
/* ---------------------------------------------------------------------- */

// Persistence (Neon Postgres — production-like)
require_('DATABASE_URL', 'Neon Postgres connection string required. Set DATABASE_URL=postgres://...');

// At-rest encryption + HMAC keys
requireMin('BREVIO_TOKEN_KEK', 32, 'BREVIO_TOKEN_KEK must be a 32-byte key (base64 or hex:)');
requireMin('BREVIO_OAUTH_STATE_KEY', 32, 'BREVIO_OAUTH_STATE_KEY must be a 32-byte key');
requireMin('BREVIO_SESSION_SIGNING_KEY', 32, 'BREVIO_SESSION_SIGNING_KEY must be a 32-byte key');

// Google OAuth client
require_('GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_ID required from Google Cloud OAuth 2.0 Client');
require_('GOOGLE_CLIENT_SECRET', 'GOOGLE_CLIENT_SECRET required from Google Cloud OAuth 2.0 Client');
require_(
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE',
  'BREVIO_OAUTH_REDIRECT_URI_GOOGLE required — must exactly match the redirect URI configured in Google Cloud'
);

// Polling
expectEquals(
  'FOMO_GMAIL_POLLING_ENABLED',
  'true',
  'Polling must be explicitly enabled. Set FOMO_GMAIL_POLLING_ENABLED=true'
);

// Cycle cap — required by Phase 3B.3 invariant, still required here.
const cap = process.env.FOMO_GMAIL_POLLING_MAX_CYCLES;
if (!cap) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'error',
    message: 'Phase 3C.4 requires a bounded smoke window. Set FOMO_GMAIL_POLLING_MAX_CYCLES=3 (or higher for the idempotency exercise; the runbook walks through this).'
  });
} else if (!/^\d+$/.test(cap.trim()) || Number(cap) <= 0) {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_MAX_CYCLES',
    severity: 'error',
    message: `Cap must be a positive integer; got '${cap}'`
  });
}

/* ---------------------------------------------------------------------- */
/* Phase 3C.4-specific: ranker requirements                               */
/* ---------------------------------------------------------------------- */

expectEquals(
  'FOMO_RANKER_ENABLED',
  'true',
  'Phase 3C.4 invariant: the ranker MUST be on for this smoke run. Set FOMO_RANKER_ENABLED=true'
);

require_(
  'OPENAI_API_KEY',
  'OPENAI_API_KEY required — buildRankerDep() throws at boot otherwise. Use the same key the 3C.2 smoke eval used.'
);

// FOMO_OPENAI_MODEL is optional; if set, surface for visibility but no enforcement.
const modelOverride = (process.env.FOMO_OPENAI_MODEL ?? '').trim();
if (modelOverride && modelOverride !== 'gpt-5-mini') {
  issues.push({
    name: 'FOMO_OPENAI_MODEL',
    severity: 'warn',
    message: `FOMO_OPENAI_MODEL='${modelOverride}'. The 3C.2 smoke eval PASS verdict was against gpt-5-mini. A different model is fine but is not gate-validated; consider re-running 3C.2 first if changing.`
  });
}

/* ---------------------------------------------------------------------- */
/* Safety warnings (non-fatal but flagged)                                */
/* ---------------------------------------------------------------------- */

if ((process.env.BREVIO_DEV_MODE ?? '').trim() === 'true') {
  issues.push({
    name: 'BREVIO_DEV_MODE',
    severity: 'warn',
    message: 'BREVIO_DEV_MODE=true bypasses production fail-closed checks. The smoke test should run with this UNSET so the production code path is exercised.'
  });
}

// Visual scope reminder.
console.log(`Hardcoded Gmail scope: ${GMAIL_READONLY_SCOPE}`);
console.log('(Phase 3C.4 regression check: NO new Gmail scopes since 3B.3.)\n');

console.log(`Resolved OpenAI model: ${modelOverride || 'gpt-5-mini'} (default gpt-5-mini; override FOMO_OPENAI_MODEL)`);
console.log('(Phase 3C.4 invariant: only the 3C.2-validated model should be used.)\n');

// Resolved kill-switch view.
const switches = loadKillSwitches(process.env);
console.log('Resolved kill switches:');
console.log(JSON.stringify(switches, null, 2));
console.log('');

// Forbidden flips during the smoke test.
if (switches.send_enabled) {
  issues.push({
    name: 'FOMO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3C.4 non-goal: no SendBlue sends. Set FOMO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.auto_send_enabled) {
  issues.push({
    name: 'FOMO_AUTO_SEND_ENABLED',
    severity: 'error',
    message: 'Phase 3C.4 non-goal: no auto-send. Set FOMO_AUTO_SEND_ENABLED=false (or unset).'
  });
}
if (switches.friend_beta_enabled) {
  issues.push({
    name: 'FOMO_FRIEND_BETA_ENABLED',
    severity: 'error',
    message: 'Phase 3C.4 non-goal: founder only. Set FOMO_FRIEND_BETA_ENABLED=false (or unset).'
  });
}

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.length === 0) {
  console.log('✓ Preflight passed. All required env vars are set and well-formed.');
  console.log('  Next steps (see docs/smoke-test-3c4-rank-on-poll.md):');
  console.log('    1. pnpm --filter @brevio/fomo run build');
  console.log('    2. pnpm --filter @brevio/fomo run dev');
  console.log('    3. open http://localhost:8080/oauth/google/start  (session auth required)');
  console.log('    4. send yourself 1-2 test emails (one obviously important, one promotional)');
  console.log('    5. after the bounded poll window, pnpm smoke-evidence:3c4');
  console.log('    6. re-run the cap-extended cycle to exercise idempotency (runbook §"second cycle")');
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
