// Eval Harness — runs a fixture set through a ModelBackend and reports
// binary precision / recall / TP / FP / TN / FN + JSON-validity counts.
//
// FOMO_PLAN §14 plans a model bake-off comparing GPT-mini vs GPT-strong vs
// Claude-Haiku vs Claude-Sonnet on real ranker fixtures. Phase 2D ships
// only the HARNESS — no fixtures, no ranker prompt, no real backends.
// Real fixtures and the bake-off itself land in Phase 3 with the ranker.
//
// Binary case only in v0.1 — one `positiveLabel` defines the positive class.
// Multi-label can extend the harness in Phase 3 when an actual call appears.
//
// The harness is intentionally a pure function over (fixtures, backend,
// label parser, positiveLabel). It does NOT touch the Model Router or
// CostStore: evals are a developer-time tool, not a production code path.
// If you want token accounting, use the router; this harness exists to
// answer "is this prompt + model good enough?".

import type { ModelBackend } from '../core/model-router.js';

export interface EvalFixture<TLabel = string> {
  readonly prompt: string;
  readonly expected_label: TLabel;
}

export interface EvalResult {
  readonly total: number;
  // Number of backend outputs the parseLabel function handled (returned non-null).
  readonly json_valid: number;
  // Number of fixtures whose expected_label === positiveLabel.
  readonly actually_positive: number;
  // Number of fixtures where the model predicted the positive label.
  readonly predicted_positive: number;
  readonly tp: number;
  readonly fp: number;
  readonly tn: number;
  readonly fn: number;
  // null when the denominator is zero — refusing to invent a metric.
  readonly precision: number | null;
  readonly recall: number | null;
}

export interface EvalHarnessRequest<TLabel> {
  readonly fixtures: readonly EvalFixture<TLabel>[];
  readonly backend: ModelBackend;
  // Parse the backend's text into a label. Return null if unparseable.
  readonly parseLabel: (text: string) => TLabel | null;
  readonly positiveLabel: TLabel;
  readonly timeout_ms?: number;
}

export async function runEval<TLabel>(req: EvalHarnessRequest<TLabel>): Promise<EvalResult> {
  const timeout_ms = req.timeout_ms ?? 30_000;

  let json_valid = 0;
  let actually_positive = 0;
  let predicted_positive = 0;
  let tp = 0;
  let fp = 0;
  let tn = 0;
  let fn = 0;

  for (const fixture of req.fixtures) {
    const expectedPositive = fixture.expected_label === req.positiveLabel;
    if (expectedPositive) actually_positive++;

    let predictedLabel: TLabel | null;
    try {
      const result = await req.backend.call({ prompt: fixture.prompt, timeout_ms });
      predictedLabel = req.parseLabel(result.text);
    } catch {
      // Backend error or unparseable — counts as unparseable / not-predicted.
      predictedLabel = null;
    }

    if (predictedLabel !== null) {
      json_valid++;
    }

    const predictedPositive = predictedLabel !== null && predictedLabel === req.positiveLabel;
    if (predictedPositive) predicted_positive++;

    if (expectedPositive && predictedPositive) tp++;
    else if (!expectedPositive && predictedPositive) fp++;
    else if (!expectedPositive && !predictedPositive) tn++;
    else fn++;
  }

  const precision = predicted_positive === 0 ? null : tp / predicted_positive;
  const recall = actually_positive === 0 ? null : tp / actually_positive;

  return Object.freeze({
    total: req.fixtures.length,
    json_valid,
    actually_positive,
    predicted_positive,
    tp,
    fp,
    tn,
    fn,
    precision,
    recall
  });
}
