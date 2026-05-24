// Phase 3C.2 ranker bake-off — Haiku 4.5 vs Sonnet 4.6.
//
// Per founder directive: 3C.2 is BAKE-OFF ONLY. No Gmail-worker
// integration, no FOMO_RANKER_ENABLED kill switch, no production
// store writes (uses InMemoryCostStore so Neon stays untouched).
//
// What this does:
//   * For each registered candidate model:
//       - Construct an AnthropicBackend
//       - Loop the 20 synthetic ranker fixtures (built via
//         buildRankerEvalFixtures: applyEgressForRanker → buildRankerPrompt)
//       - For each fixture: call the backend, parse via validateRankerOutput,
//         record latency / input_tokens / output_tokens / estimated_cost
//       - Compute aggregate metrics: precision, recall, TP/FP/TN/FN,
//         json_valid, mean & p95 latency, total cost
//   * Apply the founder-confirmed pick rule (Conservative):
//       precision >= 0.85 AND recall >= 0.85 AND json_valid_rate >= 0.95
//       primary  = cheapest among models passing all three
//       failover = the other model, IF its json_valid_rate >= 0.95
//       If no model passes → recommend 'investigate' (no auto-pick)
//   * Print a per-model summary table + the recommendation
//   * Write a structured JSON artifact to docs/bakeoff-3c2-results.json
//     for the founder to commit alongside their interpretive report
//
// Run with: pnpm --filter @brevio/fomo run bakeoff:3c2
// Requires ANTHROPIC_API_KEY in env (validated by preflight-3c2.ts).

import { writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  AnthropicAuthError,
  AnthropicApiError,
  AnthropicBackend,
  type AnthropicModelId
} from '../src/core/model-backends/anthropic.js';
import { computeEstimatedCost } from '../src/core/cost-tracking.js';
import { buildRankerEvalFixtures } from '../src/eval/ranker-eval.js';
import { type RankLabel, validateRankerOutput } from '../src/ranker/validator.js';
import { PROMPT_VERSION } from '../src/ranker/prompt.js';

/* ---------------------------------------------------------------------- */
/* Pick rule (founder-confirmed: Conservative)                            */
/* ---------------------------------------------------------------------- */

const QUALITY_GATE = Object.freeze({
  min_precision: 0.85,
  min_recall: 0.85,
  min_json_valid_rate: 0.95
});

/* ---------------------------------------------------------------------- */
/* Per-call + per-model record shapes                                     */
/* ---------------------------------------------------------------------- */

interface PerFixtureResult {
  readonly fixture_id: string;
  readonly expected_label: RankLabel;
  readonly predicted_label: RankLabel | null;
  readonly json_valid: boolean;
  readonly latency_ms: number | null;
  readonly input_tokens: number | null;
  readonly output_tokens: number | null;
  readonly estimated_cost_usd: number | null;
  readonly error: string | null;
}

interface ModelBakeoffResult {
  readonly model_id: AnthropicModelId;
  readonly prompt_version: string;
  readonly total_fixtures: number;
  readonly json_valid: number;
  readonly json_valid_rate: number;
  readonly actually_positive: number;
  readonly predicted_positive: number;
  readonly tp: number;
  readonly fp: number;
  readonly tn: number;
  readonly fn: number;
  readonly precision: number | null;
  readonly recall: number | null;
  readonly f1: number | null;
  readonly mean_latency_ms: number;
  readonly p95_latency_ms: number;
  readonly total_input_tokens: number;
  readonly total_output_tokens: number;
  readonly total_cost_usd: number;
  readonly cost_per_1k_emails_usd: number;
  readonly per_fixture: readonly PerFixtureResult[];
  readonly errors: readonly string[];
}

interface Recommendation {
  readonly primary: AnthropicModelId | null;
  readonly failover: AnthropicModelId | null;
  readonly reason: string;
  readonly gate: typeof QUALITY_GATE;
  readonly verdict: 'auto_picked' | 'manual_review_required';
}

/* ---------------------------------------------------------------------- */
/* Per-model loop                                                         */
/* ---------------------------------------------------------------------- */

async function runOneModel(
  modelId: AnthropicModelId,
  apiKey: string
): Promise<ModelBakeoffResult> {
  const backend = new AnthropicBackend({ apiKey, model: modelId });
  const converted = buildRankerEvalFixtures();
  const per_fixture: PerFixtureResult[] = [];
  const errors: string[] = [];

  console.log(`\n=== Running ${modelId} on ${converted.length} fixtures ===`);

  for (const c of converted) {
    const fxId = c.fixture.id;
    process.stdout.write(`  [${fxId}] ${c.fixture.expected_label.padEnd(13)} ...`);
    try {
      const startedAt = Date.now();
      const r = await backend.call({ prompt: c.evalFixture.prompt, timeout_ms: 30_000 });
      const latency = Date.now() - startedAt;
      const validated = validateRankerOutput(r.text);
      const predicted = validated.ok ? validated.value.label : null;
      const estCost = computeEstimatedCost(modelId, r.input_tokens, r.output_tokens);
      per_fixture.push({
        fixture_id: fxId,
        expected_label: c.fixture.expected_label,
        predicted_label: predicted,
        json_valid: validated.ok,
        latency_ms: latency,
        input_tokens: r.input_tokens,
        output_tokens: r.output_tokens,
        estimated_cost_usd: estCost,
        error: validated.ok ? null : validated.reason
      });
      const mark =
        validated.ok && predicted === c.fixture.expected_label
          ? '✓'
          : validated.ok
            ? '✗'
            : '!';
      console.log(
        ` ${mark} predicted=${predicted ?? '<invalid>'} latency=${latency}ms in=${r.input_tokens}t out=${r.output_tokens}t`
      );
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      errors.push(`${fxId}: ${msg}`);
      per_fixture.push({
        fixture_id: fxId,
        expected_label: c.fixture.expected_label,
        predicted_label: null,
        json_valid: false,
        latency_ms: null,
        input_tokens: null,
        output_tokens: null,
        estimated_cost_usd: null,
        error: msg
      });
      console.log(` ! error: ${msg}`);
      // Auth errors are global — abort the rest of this model's loop
      // because every subsequent call will fail the same way.
      if (err instanceof AnthropicAuthError) {
        console.log(`  (auth error — aborting ${modelId} loop)`);
        break;
      }
      // Non-retryable API errors abort too; retryable ones continue
      // and just count as one failed fixture (no retry logic in 3C.2).
      if (err instanceof AnthropicApiError && !err.retryable) {
        console.log(`  (non-retryable error — aborting ${modelId} loop)`);
        break;
      }
    }
  }

  // Aggregate
  const total_fixtures = converted.length;
  const json_valid = per_fixture.filter((p) => p.json_valid).length;
  const json_valid_rate = total_fixtures === 0 ? 0 : json_valid / total_fixtures;
  const actually_positive = per_fixture.filter((p) => p.expected_label === 'important').length;
  const predicted_positive = per_fixture.filter((p) => p.predicted_label === 'important').length;
  let tp = 0;
  let fp = 0;
  let tn = 0;
  let fn = 0;
  for (const p of per_fixture) {
    const isPos = p.expected_label === 'important';
    const predPos = p.predicted_label === 'important';
    if (isPos && predPos) tp++;
    else if (!isPos && predPos) fp++;
    else if (!isPos && !predPos) tn++;
    else fn++;
  }
  const precision = predicted_positive === 0 ? null : tp / predicted_positive;
  const recall = actually_positive === 0 ? null : tp / actually_positive;
  const f1 =
    precision === null || recall === null || precision + recall === 0
      ? null
      : (2 * precision * recall) / (precision + recall);
  const latencies = per_fixture
    .map((p) => p.latency_ms)
    .filter((x): x is number => x !== null)
    .sort((a, b) => a - b);
  const mean_latency_ms =
    latencies.length === 0
      ? 0
      : Math.round(latencies.reduce((a, b) => a + b, 0) / latencies.length);
  const p95_latency_ms =
    latencies.length === 0 ? 0 : latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * 0.95))] ?? 0;
  const total_input_tokens = per_fixture.reduce((s, p) => s + (p.input_tokens ?? 0), 0);
  const total_output_tokens = per_fixture.reduce((s, p) => s + (p.output_tokens ?? 0), 0);
  const total_cost_usd = per_fixture.reduce((s, p) => s + (p.estimated_cost_usd ?? 0), 0);
  const cost_per_1k_emails_usd = total_fixtures === 0 ? 0 : (total_cost_usd / total_fixtures) * 1000;

  return Object.freeze({
    model_id: modelId,
    prompt_version: PROMPT_VERSION,
    total_fixtures,
    json_valid,
    json_valid_rate,
    actually_positive,
    predicted_positive,
    tp,
    fp,
    tn,
    fn,
    precision,
    recall,
    f1,
    mean_latency_ms,
    p95_latency_ms,
    total_input_tokens,
    total_output_tokens,
    total_cost_usd,
    cost_per_1k_emails_usd,
    per_fixture: Object.freeze(per_fixture),
    errors: Object.freeze(errors)
  });
}

/* ---------------------------------------------------------------------- */
/* Recommendation rule                                                    */
/* ---------------------------------------------------------------------- */

function passesGate(r: ModelBakeoffResult): boolean {
  return (
    r.precision !== null &&
    r.precision >= QUALITY_GATE.min_precision &&
    r.recall !== null &&
    r.recall >= QUALITY_GATE.min_recall &&
    r.json_valid_rate >= QUALITY_GATE.min_json_valid_rate
  );
}

function recommend(results: readonly ModelBakeoffResult[]): Recommendation {
  const passing = results.filter(passesGate);
  if (passing.length === 0) {
    return Object.freeze({
      primary: null,
      failover: null,
      reason:
        'No candidate model met the conservative gate (precision≥0.85, recall≥0.85, json_valid≥0.95). Investigate fixtures or prompt before picking.',
      gate: QUALITY_GATE,
      verdict: 'manual_review_required' as const
    });
  }
  // Primary = cheapest passing model.
  const primary = [...passing].sort(
    (a, b) => a.cost_per_1k_emails_usd - b.cost_per_1k_emails_usd
  )[0]!;
  // Failover = the other model, IFF its json_valid_rate also passes.
  const candidates = results.filter((r) => r.model_id !== primary.model_id);
  const failover =
    candidates.find((r) => r.json_valid_rate >= QUALITY_GATE.min_json_valid_rate) ?? null;
  return Object.freeze({
    primary: primary.model_id,
    failover: failover?.model_id ?? null,
    reason: failover
      ? `${primary.model_id} is the cheapest model passing the gate; ${failover.model_id} cleared json_valid threshold and stands by as failover.`
      : `${primary.model_id} is the cheapest model passing the gate; no other candidate met json_valid≥${QUALITY_GATE.min_json_valid_rate} so no failover is recommended.`,
    gate: QUALITY_GATE,
    verdict: 'auto_picked' as const
  });
}

/* ---------------------------------------------------------------------- */
/* Printers                                                               */
/* ---------------------------------------------------------------------- */

function printSummaryTable(results: readonly ModelBakeoffResult[]): void {
  console.log('\n========================================================================');
  console.log('Phase 3C.2 bake-off — per-model summary');
  console.log('========================================================================');
  for (const r of results) {
    console.log(`\nModel: ${r.model_id}`);
    console.log(`  prompt_version       ${r.prompt_version}`);
    console.log(`  total fixtures       ${r.total_fixtures}`);
    console.log(`  json_valid           ${r.json_valid}/${r.total_fixtures}  (${(r.json_valid_rate * 100).toFixed(1)}%)`);
    console.log(`  TP/FP/TN/FN          ${r.tp}/${r.fp}/${r.tn}/${r.fn}`);
    console.log(`  precision            ${r.precision === null ? 'n/a' : r.precision.toFixed(3)}`);
    console.log(`  recall               ${r.recall === null ? 'n/a' : r.recall.toFixed(3)}`);
    console.log(`  F1                   ${r.f1 === null ? 'n/a' : r.f1.toFixed(3)}`);
    console.log(`  mean / p95 latency   ${r.mean_latency_ms}ms / ${r.p95_latency_ms}ms`);
    console.log(`  total tokens (in/out) ${r.total_input_tokens} / ${r.total_output_tokens}`);
    console.log(`  total cost           $${r.total_cost_usd.toFixed(4)}`);
    console.log(`  cost per 1k emails   $${r.cost_per_1k_emails_usd.toFixed(2)}`);
    if (r.errors.length > 0) {
      console.log(`  errors:`);
      for (const e of r.errors) console.log(`    - ${e}`);
    }
  }
}

function printRecommendation(rec: Recommendation): void {
  console.log('\n========================================================================');
  console.log('Recommendation');
  console.log('========================================================================');
  console.log(`  Gate: precision >= ${rec.gate.min_precision}, recall >= ${rec.gate.min_recall}, json_valid >= ${rec.gate.min_json_valid_rate}`);
  console.log(`  Primary:  ${rec.primary ?? '(none — manual review required)'}`);
  console.log(`  Failover: ${rec.failover ?? '(none)'}`);
  console.log(`  ${rec.reason}`);
  console.log(`  Verdict: ${rec.verdict.toUpperCase()}`);
}

/* ---------------------------------------------------------------------- */
/* Main                                                                   */
/* ---------------------------------------------------------------------- */

async function main(): Promise<void> {
  const apiKey = (process.env.ANTHROPIC_API_KEY ?? '').trim();
  if (!apiKey) {
    console.error('ANTHROPIC_API_KEY required. Run `pnpm preflight:3c2` first.');
    process.exit(2);
  }

  const candidates: readonly AnthropicModelId[] = [
    'claude-haiku-4-5-20251001',
    'claude-sonnet-4-6'
  ];

  console.log('Phase 3C.2 — Anthropic ranker bake-off');
  console.log(`Candidates: ${candidates.join(', ')}`);
  console.log(`Fixtures:   20 synthetic, prompt_version=${PROMPT_VERSION}`);
  console.log(`Pick rule:  Conservative (precision≥${QUALITY_GATE.min_precision}, recall≥${QUALITY_GATE.min_recall}, json_valid≥${QUALITY_GATE.min_json_valid_rate})`);

  const startedAt = new Date().toISOString();
  const results: ModelBakeoffResult[] = [];
  for (const modelId of candidates) {
    results.push(await runOneModel(modelId, apiKey));
  }
  const finishedAt = new Date().toISOString();

  printSummaryTable(results);
  const rec = recommend(results);
  printRecommendation(rec);

  // Write JSON artifact. The founder reviews + commits this alongside
  // the BAKEOFF_REPORT_3C2.md they write from it.
  const here = path.dirname(fileURLToPath(import.meta.url));
  const artifactPath = path.resolve(here, '../../../docs/bakeoff-3c2-results.json');
  const artifact = Object.freeze({
    phase: '3C.2',
    started_at: startedAt,
    finished_at: finishedAt,
    prompt_version: PROMPT_VERSION,
    pick_rule: QUALITY_GATE,
    candidates,
    results,
    recommendation: rec
  });
  await writeFile(artifactPath, JSON.stringify(artifact, null, 2), 'utf8');
  console.log(`\nArtifact written: ${artifactPath}`);

  if (rec.verdict === 'manual_review_required') {
    process.exit(1);
  }
  process.exit(0);
}

main().catch((err: unknown) => {
  console.error('bakeoff-3c2 crashed:', err instanceof Error ? err.message : String(err));
  process.exit(2);
});
