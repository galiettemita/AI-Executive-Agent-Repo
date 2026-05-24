// Ranker eval wiring — Phase 3C.1.
//
// Bridges the synthetic ranker fixtures (eval/ranker-fixtures) to the
// generic eval harness (eval/harness) for the ranker's
// 'classification' capability.
//
// Per-fixture flow:
//   RankerFixture →
//     synth RawEmailContext →
//     applyEgressForRanker (Phase 2C safe view) →
//     buildRankerPrompt    (Phase 3C.1 versioned prompt) →
//     EvalFixture<RankLabel> for runEval()
//
// Backend is injected, so the same eval can run against
// MockModelBackend (CI tests, deterministic), AnthropicBackend
// (founder bake-off in Phase 3C.2), or any future ModelBackend.
//
// runRankerEval() returns the raw EvalResult plus the converted
// fixtures so callers can attribute hits/misses back to specific
// fixture IDs if they want — useful for the bake-off in 3C.2.

import {
  type RawEmailContext,
  applyEgressForRanker,
  DEFAULT_EGRESS_OPTIONS,
  type EgressOptions
} from '../core/egress-policy.js';
import { type ModelBackend } from '../core/model-router.js';
import { type EvalFixture, type EvalResult, runEval } from './harness.js';

import { RANKER_FIXTURES, type RankerFixture } from './ranker-fixtures/fixtures.js';
import { type RankLabel, validateRankerOutput } from '../ranker/validator.js';
import { buildRankerPrompt } from '../ranker/prompt.js';

export { RANKER_FIXTURES } from './ranker-fixtures/fixtures.js';
export type { RankerFixture } from './ranker-fixtures/fixtures.js';

// Convert a RankerFixture into the synthetic RawEmailContext the
// ranker module would see in production. body_html / headers /
// attachments are deliberately empty — the egress view doesn't
// expose them, and we don't want a fixture to fail by accident
// because of a header probe.
function fixtureToRawEmail(fx: RankerFixture, index: number): RawEmailContext {
  return Object.freeze({
    message_id: `fixture-${fx.id}`,
    thread_id: `fixture-thread-${fx.id}`,
    sender_email: fx.sender_email,
    sender_name: fx.sender_name,
    subject: fx.subject,
    body_plain: fx.body_plain,
    body_html: undefined,
    headers: {},
    attachments: fx.has_attachments
      ? Object.freeze([Object.freeze({ filename: 'fixture-attachment.pdf', size_bytes: 1024 })])
      : Object.freeze([]),
    // Stagger received_at by index so the prompt's "Received: …" line
    // isn't identical for every fixture.
    received_at: new Date(Date.UTC(2026, 4, 1) + index * 3600_000)
  } as RawEmailContext);
}

export interface ConvertedFixture {
  readonly fixture: RankerFixture;
  readonly evalFixture: EvalFixture<RankLabel>;
}

export function buildRankerEvalFixtures(
  fixtures: readonly RankerFixture[] = RANKER_FIXTURES,
  egressOptions: EgressOptions = DEFAULT_EGRESS_OPTIONS
): readonly ConvertedFixture[] {
  return Object.freeze(
    fixtures.map((fx, i) => {
      const raw = fixtureToRawEmail(fx, i);
      const view = applyEgressForRanker(raw, egressOptions);
      const prompt = buildRankerPrompt(view);
      return Object.freeze({
        fixture: fx,
        evalFixture: Object.freeze({
          prompt,
          expected_label: fx.expected_label
        })
      });
    })
  );
}

// parseLabel adapter for the eval harness — uses the same validator
// the production ranker uses, then narrows to the label only.
function parseLabel(text: string): RankLabel | null {
  const r = validateRankerOutput(text);
  return r.ok ? r.value.label : null;
}

export interface RankerEvalRequest {
  readonly backend: ModelBackend;
  readonly fixtures?: readonly RankerFixture[];
  readonly egressOptions?: EgressOptions;
  readonly timeout_ms?: number;
  // Defaults to 'important' (the FOMO-positive class).
  readonly positiveLabel?: RankLabel;
}

export interface RankerEvalResult {
  readonly summary: EvalResult;
  // Each fixture + its converted EvalFixture, in order. The summary
  // counts cannot tell you WHICH fixture missed; callers that want
  // per-fixture attribution can re-run them through the backend or
  // use this list for reference.
  readonly converted: readonly ConvertedFixture[];
}

export async function runRankerEval(req: RankerEvalRequest): Promise<RankerEvalResult> {
  const converted = buildRankerEvalFixtures(req.fixtures, req.egressOptions);
  const summary = await runEval<RankLabel>({
    fixtures: converted.map((c) => c.evalFixture),
    backend: req.backend,
    parseLabel,
    positiveLabel: req.positiveLabel ?? 'important',
    timeout_ms: req.timeout_ms
  });
  return Object.freeze({ summary, converted });
}
