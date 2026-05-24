// FOMO Ranker — Phase 3C.1.
//
// Pure function that takes a RawEmailContext (as produced by the
// gmail.read executor in Phase 3B.2), reduces it through the egress
// policy, builds a prompt, routes through the model router with the
// validator, and returns a RankerResult. No store writes, no audit
// writes — the caller (Phase 3C.2 polling-worker integration) decides
// what to do with the result.
//
// Why pure: 3C.1's load-bearing test is "does the ranker reliably
// classify our synthetic fixtures." That's a function-of-(prompt,
// model) question. Putting the function behind an integration layer
// would make it harder to score across multiple models for the 3C.2
// bake-off.
//
// Egress invariant: rankEmail() ALWAYS calls applyEgressForRanker
// first. Callers cannot pass a custom view that includes body_html /
// headers / attachment filenames. This module is the only path from
// RawEmailContext to a model call for ranking.

import {
  type RawEmailContext,
  applyEgressForRanker,
  DEFAULT_EGRESS_OPTIONS,
  type EgressOptions
} from '../core/egress-policy.js';
import { type ModelRouter, type ModelRouteResult } from '../core/model-router.js';

import { PROMPT_VERSION, buildRankerPrompt } from './prompt.js';
import { type RankDecision, validateRankerOutput } from './validator.js';

export { PROMPT_VERSION } from './prompt.js';
export type { RankDecision, RankLabel } from './validator.js';

export interface RankerDeps {
  readonly router: ModelRouter;
  // Egress options override (defaults to DEFAULT_EGRESS_OPTIONS).
  readonly egressOptions?: EgressOptions;
  // Per-call model timeout. Defaults to the router's default if omitted.
  readonly timeoutMs?: number;
}

export interface RankerRequest {
  readonly raw: RawEmailContext;
  readonly user_id: string;
}

export interface RankerSuccess {
  readonly ok: true;
  readonly decision: RankDecision;
  readonly model_name: string;
  readonly prompt_version: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
}

export interface RankerFailure {
  readonly ok: false;
  // Mirrors ModelRouteErrorCode plus the ranker's own 'no_backend' for
  // when the model router has no registered classification backend.
  readonly code:
    | 'unknown_capability'
    | 'no_backend_for_capability'
    | 'backend_error'
    | 'timeout'
    | 'schema_invalid';
  readonly reason: string;
  readonly model_name: string | null;
  readonly prompt_version: string;
}

export type RankerResult = RankerSuccess | RankerFailure;

export async function rankEmail(req: RankerRequest, deps: RankerDeps): Promise<RankerResult> {
  const egressOptions = deps.egressOptions ?? DEFAULT_EGRESS_OPTIONS;
  const view = applyEgressForRanker(req.raw, egressOptions);
  const prompt = buildRankerPrompt(view);

  const routed: ModelRouteResult<RankDecision> = await deps.router.route<RankDecision>({
    capability: 'classification',
    prompt,
    prompt_version: PROMPT_VERSION,
    user_id: req.user_id,
    validate: validateRankerOutput,
    timeout_ms: deps.timeoutMs
  });

  if (routed.ok) {
    return Object.freeze({
      ok: true as const,
      decision: routed.output,
      model_name: routed.model_name,
      prompt_version: PROMPT_VERSION,
      latency_ms: routed.latency_ms,
      input_tokens: routed.input_tokens,
      output_tokens: routed.output_tokens,
      estimated_cost_usd: routed.estimated_cost_usd
    });
  }

  return Object.freeze({
    ok: false as const,
    code: routed.code,
    reason: routed.reason,
    model_name: routed.model_name,
    prompt_version: PROMPT_VERSION
  });
}
