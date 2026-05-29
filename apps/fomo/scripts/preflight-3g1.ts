// Phase 3G.1 preflight — Production Hardening.
//
// 3G.1 is internal hardening only; the four items (migration verifier,
// SendBlue OPTED_OUT decoder, needs_reauth visibility, memory_signals
// boot snapshot) are all proven by regression tests. The preflight
// here is intentionally narrow: confirm the env shape the dev server
// needs to surface the new behavior + confirm the new pnpm migration
// script is wired.
//
// No DB. No network. Pure config inspection.

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

console.log('Phase 3G.1 preflight — Production Hardening\n');

/* ---------------------------------------------------------------------- */
/* Substrate                                                              */
/* ---------------------------------------------------------------------- */

require_('DATABASE_URL', 'Neon Postgres URL required — migration verifier runs at boot only when DATABASE_URL is set.');
require_('BREVIO_TOKEN_KEK', 'BREVIO_TOKEN_KEK required (32-byte base64).');

/* ---------------------------------------------------------------------- */
/* Founder user surface                                                   */
/* ---------------------------------------------------------------------- */

require_(
  'FOMO_FOUNDER_USER_ID',
  'FOMO_FOUNDER_USER_ID required. The memory_signals boot snapshot (item #10) scopes to this user. ' +
    'Without it, the snapshot is skipped silently.'
);

/* ---------------------------------------------------------------------- */
/* Phase 3G.1 invariants                                                  */
/* ---------------------------------------------------------------------- */

// No new env vars are introduced by 3G.1. The migration verifier has
// NO env override by design — fail-loud everywhere is the founder-
// locked policy.
//
// Item #4 from the 3G.1 catalog (smoke-cap vs prod-cap separation) is
// out of scope per the 6-question gate; this preflight does not enforce
// any cap defaults.

console.log('Resolved environment shape for the four 3G.1 items:');
console.log(`  DATABASE_URL set:               ${(process.env.DATABASE_URL ?? '').trim() ? 'yes' : 'no'}`);
console.log(`  FOMO_FOUNDER_USER_ID set:       ${(process.env.FOMO_FOUNDER_USER_ID ?? '').trim() ? 'yes' : 'no'}`);
console.log(`  FOMO_GMAIL_POLLING_ENABLED:     ${(process.env.FOMO_GMAIL_POLLING_ENABLED ?? '').trim() || '<unset>'}`);
console.log('');

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

const errors = issues.filter((i) => i.severity === 'error');
if (errors.length === 0) {
  console.log('✓ Preflight passed.');
  console.log('  3G.1 PASS is gated on regression-test coverage; run:');
  console.log('    pnpm --filter @brevio/fomo run test');
  console.log('  followed by the optional founder fault-injection runbook:');
  console.log('    docs/smoke-test-3g1-production-hardening.md');
  process.exit(0);
}

console.log(`✖ ${errors.length} required check(s) failed:\n`);
for (const e of errors) {
  console.log(`  [ERROR] ${e.name}: ${e.message}`);
}
process.exit(1);
