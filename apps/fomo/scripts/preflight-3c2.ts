// Phase 3C.2 preflight (OpenAI ranker smoke eval).
//
// Founder directive 2026-05-24 superseded the prior Haiku-vs-Sonnet
// bakeoff path. 3C.2 is now: OpenAI as initial Brevio ranker brain,
// with a smoke eval against the 20 synthetic fixtures to catch
// catastrophic failures before 3C.3 wires the ranker into the Gmail
// worker.
//
// This preflight validates the env that scripts/smoke-eval-3c2.ts will
// use. No network calls, no DB calls.
//
// Exit 0: ready to run `pnpm smoke-eval:3c2`.
// Exit 1: missing/invalid config; founder fixes and re-runs.

import { MODEL_PRICING } from '../src/core/cost-tracking.js';

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

console.log('Phase 3C.2 preflight — OpenAI ranker smoke eval\n');

require_(
  'OPENAI_API_KEY',
  'OpenAI API key required. Get one at https://platform.openai.com/api-keys. Set OPENAI_API_KEY=sk-...'
);

const key = (process.env.OPENAI_API_KEY ?? '').trim();
if (key && !key.startsWith('sk-')) {
  issues.push({
    name: 'OPENAI_API_KEY',
    severity: 'warn',
    message: "Doesn't start with 'sk-'. OpenAI keys normally have that prefix; double-check you copied the right value."
  });
}

// Optional org id — only flagged if present so the founder knows the
// smoke eval will run under that org.
const org = (process.env.OPENAI_ORG_ID ?? '').trim();
if (org) {
  console.log(`OPENAI_ORG_ID is set: ${org}`);
  console.log('(The smoke-eval call will be billed to this organization.)\n');
}

// Model — founder picked gpt-5-mini as initial. Allow override via
// FOMO_OPENAI_MODEL in case the account doesn't yet have gpt-5-mini
// available.
const DEFAULT_MODEL = 'gpt-5-mini';
const resolvedModel = (process.env.FOMO_OPENAI_MODEL ?? '').trim() || DEFAULT_MODEL;
console.log(`Resolved model: ${resolvedModel} (default: ${DEFAULT_MODEL}; override: FOMO_OPENAI_MODEL)`);

if (!Object.prototype.hasOwnProperty.call(MODEL_PRICING, resolvedModel)) {
  issues.push({
    name: 'FOMO_OPENAI_MODEL',
    severity: 'warn',
    message:
      `'${resolvedModel}' has no entry in MODEL_PRICING. The smoke eval will still run, but estimated_cost_usd will report $0 for every call. Add an entry to apps/fomo/src/core/cost-tracking.ts before relying on the cost number.`
  });
} else {
  const p = MODEL_PRICING[resolvedModel]!;
  console.log(`  pricing: $${p.input_per_1m_usd}/1M input, $${p.output_per_1m_usd}/1M output`);
  // Cost estimate: 20 fixtures × ~600 input + ~30 output tokens.
  const estInUsd = (20 * 600 / 1_000_000) * p.input_per_1m_usd;
  const estOutUsd = (20 * 30 / 1_000_000) * p.output_per_1m_usd;
  const total = estInUsd + estOutUsd;
  console.log(`  estimated total cost for one full smoke eval (20 fixtures): ~$${total.toFixed(4)}`);
}
console.log('');

// Forbidden flags during the smoke eval — keep this run isolated from
// any prod/worker plumbing.
if ((process.env.FOMO_RANKER_ENABLED ?? '').toLowerCase() === 'true') {
  issues.push({
    name: 'FOMO_RANKER_ENABLED',
    severity: 'warn',
    message:
      "Set to 'true'. The smoke eval doesn't read this flag, but it indicates 3C.3 worker-integration plumbing is configured. Unset for a clean 3C.2 run."
  });
}
if ((process.env.FOMO_GMAIL_POLLING_ENABLED ?? '').toLowerCase() === 'true') {
  issues.push({
    name: 'FOMO_GMAIL_POLLING_ENABLED',
    severity: 'warn',
    message:
      "Set to 'true'. The Gmail polling worker is unrelated to the 3C.2 smoke eval — but if it's running in another process it will keep polling the founder inbox while you eval. Unset unless you want both at once."
  });
}

/* ---------------------------------------------------------------------- */
/* Report                                                                 */
/* ---------------------------------------------------------------------- */

if (issues.length === 0) {
  console.log('✓ Preflight passed.');
  console.log('  Next: pnpm --filter @brevio/fomo run smoke-eval:3c2');
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
