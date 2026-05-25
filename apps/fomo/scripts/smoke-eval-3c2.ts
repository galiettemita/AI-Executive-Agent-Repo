// Phase 3C.2 OpenAI ranker smoke eval.
//
// Founder directive 2026-05-24: 3C.2 is single-provider, single-model.
// NOT a bake-off. The goal is "does this OpenAI mini model meet the
// minimum quality bar to advance to 3C.3 (Gmail worker integration)"
// — not "which of N models is best."
//
// What this does:
//   * Loads OPENAI_API_KEY and resolves the model id (default
//     gpt-5-mini, overridable via FOMO_OPENAI_MODEL).
//   * Constructs OpenAIBackend with the ranker JSON Schema as
//     response_format so OpenAI enforces the output shape server-side.
//     The existing validateRankerOutput() runs as defense-in-depth.
//   * Builds the 20 synthetic fixtures via buildRankerEvalFixtures
//     (applyEgressForRanker + buildRankerPrompt — same path the
//     production ranker would use).
//   * Calls OpenAI once per fixture, recording latency / tokens / cost.
//   * Computes precision / recall / F1 / TP-FP-TN-FN / json_valid_rate
//     / mean & p95 latency / total cost / cost per 1k emails.
//   * Applies the Conservative pass gate:
//       precision >= 0.85 AND recall >= 0.85 AND json_valid_rate >= 0.95
//   * Prints a summary + verdict (PASS or INVESTIGATE).
//   * Writes structured JSON artifact to docs/openai-smoke-eval-3c2-results.json
//     for the founder to commit alongside the interpretive report.
//
// Run with: pnpm --filter @brevio/fomo run smoke-eval:3c2
// Requires OPENAI_API_KEY in env (validated by preflight-3c2.ts).

import { writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { computeEstimatedCost } from '../src/core/cost-tracking.js';
import {
  OpenAIAuthError,
  OpenAIApiError,
  OpenAIBackend,
  type OpenAIResponseFormat
} from '../src/core/model-backends/openai.js';
import { buildRankerEvalFixtures } from '../src/eval/ranker-eval.js';
import { PROMPT_VERSION } from '../src/ranker/prompt.js';
import { type RankLabel, validateRankerOutput } from '../src/ranker/validator.js';

/* ---------------------------------------------------------------------- */
/* Pass gate (founder-confirmed Conservative)                             */
/* ---------------------------------------------------------------------- */

const PASS_GATE = Object.freeze({
  min_precision: 0.85,
  min_recall: 0.85,
  min_json_valid_rate: 0.95
});

/* ---------------------------------------------------------------------- */
/* Structured-output schema sent to OpenAI                                */
/* ---------------------------------------------------------------------- */

const RANKER_RESPONSE_FORMAT: OpenAIResponseFormat = Object.freeze({
  type: 'json_schema',
  json_schema: {
    name: 'ranker_decision',
    strict: true,
    schema: Object.freeze({
      type: 'object',
      properties: {
        label: { type: 'string', enum: ['important', 'not_important'] },
        // OpenAI strict mode doesn't accept minimum/maximum. The
        // validator (validateRankerOutput) enforces 0..1 client-side.
        score: { type: 'number' },
        reason: { type: 'string' }
      },
      required: ['label', 'score', 'reason'],
      additionalProperties: false
    })
  }
});

/* ---------------------------------------------------------------------- */
/* Per-call + aggregate types                                             */
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

interface SmokeEvalResult {
  readonly model_id: string;
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

interface Verdict {
  readonly status: 'PASS' | 'INVESTIGATE';
  readonly reason: string;
  readonly gate: typeof PASS_GATE;
  readonly allows_3c3: boolean;
}

/* ---------------------------------------------------------------------- */
/* Smoke eval runner                                                      */
/* ---------------------------------------------------------------------- */

async function runSmokeEval(model: string, apiKey: string): Promise<SmokeEvalResult> {
  const backend = new OpenAIBackend({
    apiKey,
    model,
    responseFormat: RANKER_RESPONSE_FORMAT
  });
  const converted = buildRankerEvalFixtures();
  const per_fixture: PerFixtureResult[] = [];
  const errors: string[] = [];

  console.log(`\n=== Running smoke eval on ${model} (${converted.length} fixtures) ===`);

  for (const c of converted) {
    const fxId = c.fixture.id;
    process.stdout.write(`  [${fxId}] ${c.fixture.expected_label.padEnd(13)} ...`);
    try {
      const startedAt = Date.now();
      const r = await backend.call({ prompt: c.evalFixture.prompt, timeout_ms: 30_000 });
      const latency = Date.now() - startedAt;
      const validated = validateRankerOutput(r.text);
      const predicted = validated.ok ? validated.value.label : null;
      const estCost = computeEstimatedCost(model, r.input_tokens, r.output_tokens);
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
      if (err instanceof OpenAIAuthError) {
        console.log('  (auth error — aborting smoke eval; rest of fixtures will fail the same way)');
        break;
      }
      if (err instanceof OpenAIApiError && !err.retryable) {
        console.log('  (non-retryable API error — aborting)');
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
    latencies.length === 0
      ? 0
      : latencies[Math.min(latencies.length - 1, Math.floor(latencies.length * 0.95))] ?? 0;
  const total_input_tokens = per_fixture.reduce((s, p) => s + (p.input_tokens ?? 0), 0);
  const total_output_tokens = per_fixture.reduce((s, p) => s + (p.output_tokens ?? 0), 0);
  const total_cost_usd = per_fixture.reduce((s, p) => s + (p.estimated_cost_usd ?? 0), 0);
  const cost_per_1k_emails_usd = total_fixtures === 0 ? 0 : (total_cost_usd / total_fixtures) * 1000;

  return Object.freeze({
    model_id: model,
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

function decideVerdict(r: SmokeEvalResult): Verdict {
  const precisionOk =
    r.precision !== null && r.precision >= PASS_GATE.min_precision;
  const recallOk = r.recall !== null && r.recall >= PASS_GATE.min_recall;
  const jsonOk = r.json_valid_rate >= PASS_GATE.min_json_valid_rate;
  const allOk = precisionOk && recallOk && jsonOk;
  const reasonParts: string[] = [];
  if (!precisionOk) {
    reasonParts.push(
      `precision ${r.precision === null ? 'n/a' : r.precision.toFixed(3)} < ${PASS_GATE.min_precision}`
    );
  }
  if (!recallOk) {
    reasonParts.push(
      `recall ${r.recall === null ? 'n/a' : r.recall.toFixed(3)} < ${PASS_GATE.min_recall}`
    );
  }
  if (!jsonOk) {
    reasonParts.push(
      `json_valid_rate ${r.json_valid_rate.toFixed(3)} < ${PASS_GATE.min_json_valid_rate}`
    );
  }
  return Object.freeze({
    status: allOk ? ('PASS' as const) : ('INVESTIGATE' as const),
    reason: allOk
      ? `${r.model_id} cleared all three gates (precision≥${PASS_GATE.min_precision}, recall≥${PASS_GATE.min_recall}, json_valid≥${PASS_GATE.min_json_valid_rate}).`
      : `${r.model_id} failed: ${reasonParts.join('; ')}.`,
    gate: PASS_GATE,
    allows_3c3: allOk
  });
}

/* ---------------------------------------------------------------------- */
/* Printers                                                               */
/* ---------------------------------------------------------------------- */

function printSummary(r: SmokeEvalResult): void {
  console.log('\n========================================================================');
  console.log('Phase 3C.2 OpenAI smoke eval — summary');
  console.log('========================================================================');
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

function printVerdict(v: Verdict): void {
  console.log('\n========================================================================');
  console.log('Verdict');
  console.log('========================================================================');
  console.log(`  Gate: precision >= ${v.gate.min_precision}, recall >= ${v.gate.min_recall}, json_valid >= ${v.gate.min_json_valid_rate}`);
  console.log(`  ${v.reason}`);
  console.log(`  Status: ${v.status}`);
  console.log(`  Phase 3C.3 allowed by this run: ${v.allows_3c3 ? 'YES' : 'NO (founder must INVESTIGATE)'}`);
}

/* ---------------------------------------------------------------------- */
/* Main                                                                   */
/* ---------------------------------------------------------------------- */

async function main(): Promise<void> {
  const apiKey = (process.env.OPENAI_API_KEY ?? '').trim();
  if (!apiKey) {
    console.error('OPENAI_API_KEY required. Run `pnpm preflight:3c2` first.');
    process.exit(2);
  }

  const DEFAULT_MODEL = 'gpt-5-mini';
  const model = (process.env.FOMO_OPENAI_MODEL ?? '').trim() || DEFAULT_MODEL;

  console.log('Phase 3C.2 — OpenAI ranker smoke eval');
  console.log(`Model:      ${model}${model === DEFAULT_MODEL ? '' : ' (overridden via FOMO_OPENAI_MODEL)'}`);
  console.log(`Fixtures:   20 synthetic, prompt_version=${PROMPT_VERSION}`);
  console.log(`Pass gate:  Conservative (precision≥${PASS_GATE.min_precision}, recall≥${PASS_GATE.min_recall}, json_valid≥${PASS_GATE.min_json_valid_rate})`);

  const startedAt = new Date().toISOString();
  const result = await runSmokeEval(model, apiKey);
  const finishedAt = new Date().toISOString();
  const verdict = decideVerdict(result);

  printSummary(result);
  printVerdict(verdict);

  const here = path.dirname(fileURLToPath(import.meta.url));
  const artifactPath = path.resolve(here, '../../../docs/openai-smoke-eval-3c2-results.json');
  const artifact = Object.freeze({
    phase: '3C.2',
    started_at: startedAt,
    finished_at: finishedAt,
    prompt_version: PROMPT_VERSION,
    pass_gate: PASS_GATE,
    model_id: result.model_id,
    result,
    verdict
  });
  await writeFile(artifactPath, JSON.stringify(artifact, null, 2), 'utf8');
  console.log(`\nArtifact written: ${artifactPath}`);

  process.exit(verdict.status === 'PASS' ? 0 : 1);
}

main().catch((err: unknown) => {
  console.error('smoke-eval-3c2 crashed:', err instanceof Error ? err.message : String(err));
  process.exit(2);
});
