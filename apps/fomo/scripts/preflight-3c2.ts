// Phase 3C.2 preflight — verifies env required for the Haiku-vs-Sonnet
// bake-off BEFORE the script bills any Anthropic credits.
//
// No network calls, no DB calls. Pure config inspection.
//
// Exit 0: ready to run `pnpm bakeoff:3c2`.
// Exit 1: missing/invalid config; founder fixes the listed items and re-runs.
//
// 3C.2 scope (per founder directive):
//   - Run real Anthropic bake-off on the 20 synthetic fixtures.
//   - Compare Haiku vs Sonnet.
//   - Pick primary + failover.
//   - NO Gmail-worker integration in this PR.

interface Check {
  readonly name: string;
  readonly severity: 'error' | 'warn';
  readonly message: string;
}

const issues: Check[] = [];

function require_(name: string, message: string): void {
  if (!(process.env[name] ?? '').trim()) {
    issues.push({ name, severity: 'error', message });
  }
}

console.log('Phase 3C.2 preflight — Anthropic ranker bake-off\n');

require_(
  'ANTHROPIC_API_KEY',
  'Anthropic API key required. Get one at https://console.anthropic.com/. Set ANTHROPIC_API_KEY=sk-ant-...'
);

const key = (process.env.ANTHROPIC_API_KEY ?? '').trim();
if (key && !key.startsWith('sk-ant-')) {
  issues.push({
    name: 'ANTHROPIC_API_KEY',
    severity: 'warn',
    message: "Doesn't start with 'sk-ant-'. Anthropic keys normally have that prefix; double-check you copied the right value."
  });
}

// Cost estimate so founder knows what they're about to spend.
// 20 fixtures × 2 models × ~600 input tokens + ~30 output tokens each.
// Rough estimate using MODEL_PRICING from cost-tracking.ts:
//   Haiku  20 calls × (600 × $1/1M + 30 × $5/1M)  = ~$0.015
//   Sonnet 20 calls × (600 × $3/1M + 30 × $15/1M) = ~$0.045
//   Total ~ $0.06 for a full bake-off run.
console.log('Estimated cost of one full bake-off run: ~ $0.05–$0.10');
console.log('  Haiku  20 × ~630 tokens ≈ $0.015');
console.log('  Sonnet 20 × ~630 tokens ≈ $0.045');
console.log('  (Real Anthropic billing — credits or card on file required.)');
console.log('');

// Forbidden flags during the bake-off — none change pricing, but if the
// founder has FOMO_RANKER_ENABLED or anything similar leaked from a
// later phase, flag it so this script doesn't accidentally pull in
// production wiring.
if ((process.env.FOMO_RANKER_ENABLED ?? '').toLowerCase() === 'true') {
  issues.push({
    name: 'FOMO_RANKER_ENABLED',
    severity: 'warn',
    message:
      "Set to 'true'. The bake-off doesn't read this flag, but it indicates 3C.3 worker-integration plumbing is configured — unset for a clean 3C.2 run."
  });
}

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.length === 0) {
  console.log('✓ Preflight passed.');
  console.log('  Next: pnpm --filter @brevio/fomo run bakeoff:3c2');
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
