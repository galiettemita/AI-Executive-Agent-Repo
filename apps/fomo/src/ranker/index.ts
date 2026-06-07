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

import { PROMPT_VERSION, PROMPT_VERSION_WITH_PIL, buildRankerPrompt } from './prompt.js';
import { type PilContext } from './pil-context.js';
import { type RankDecision, validateRankerOutput } from './validator.js';

export { PROMPT_VERSION, PROMPT_VERSION_WITH_PIL } from './prompt.js';
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
  // Phase v0.5.12 — when non-null, prompt assembly includes the PIL prior
  // block AND prompt_version becomes 'ranker-v0.3.0'. When null (default),
  // behavior is bit-identical to v0.5.11 / v0.5.7 (single-call, ranker-v0.2.0).
  // The two-call hybrid for the score cap (Q1.C + Q2.A) lives at the production
  // call site (worker), NOT inside rankEmail — keeps this function pure for
  // multi-model bake-off testing.
  readonly pil_context?: PilContext | null;
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
  const pilContext = req.pil_context ?? null;
  const prompt = buildRankerPrompt(view, pilContext);
  // v0.5.12: per-call prompt_version. PIL-context calls use ranker-v0.3.0;
  // baseline (null) calls use ranker-v0.2.0. The two-call hybrid emits BOTH
  // versions (one cost_records row per call). rank_results.prompt_version
  // stores whichever produced the persisted decision (worker chooses).
  const promptVersion = pilContext !== null ? PROMPT_VERSION_WITH_PIL : PROMPT_VERSION;

  const routed: ModelRouteResult<RankDecision> = await deps.router.route<RankDecision>({
    capability: 'classification',
    prompt,
    prompt_version: promptVersion,
    user_id: req.user_id,
    validate: validateRankerOutput,
    timeout_ms: deps.timeoutMs
  });

  if (routed.ok) {
    return Object.freeze({
      ok: true as const,
      decision: routed.output,
      model_name: routed.model_name,
      prompt_version: promptVersion,
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
    prompt_version: promptVersion
  });
}

/* ====================================================================== */
/* Phase v0.5.12 — Two-call hybrid (Q1.C-modified)                        */
/* ====================================================================== */

import { type AuditStore } from '../core/audit.js';
import { CANONICAL_SCOPE_KEY_REGEX } from './pil-context.js';

/**
 * Regex on the model-authored rank.reason text: does the reason appear to
 * mention the PIL prior? Used to populate the `model_mentioned_pil_in_reason`
 * audit field as a boolean — the reason TEXT itself never enters audit detail.
 *
 * Conservative match — many phrases the v0.2.0 prompt examples show would
 * trigger if the model is genuinely referencing prior feedback. Captures both
 * acknowledgment ("you've ignored", "previously ignored") and override
 * ("despite past", "even though earlier"). Per the founder transparency floor
 * principle: ≥40% of bounded-influence cases should mention the prior; if the
 * model never mentions it, smoke surfaces the gap.
 */
const PIL_MENTION_REGEX =
  /\b(prior|previously|past|earlier|history|previously? (ignore|ignored|approve|approved|mark|marked|like|liked)|you (have |usually |often |once )?(ignore|ignored|approve|approved|mark|marked|like|liked)|despite|even though|usually unimportant|past feedback|past behavior|known sender)\b/i;

export function modelMentionedPilInReason(reason: string): boolean {
  if (typeof reason !== 'string' || reason.length === 0) return false;
  return PIL_MENTION_REGEX.test(reason);
}

/**
 * Locked 9-field detail shape per Q6.A. Exported as a type so the audit
 * writer and the eval harness share the contract.
 *
 * Privacy: NO raw sender_email, subject, body, snippet, headers, or raw
 * rank.reason. `scope_key_hash` IS the HMAC (safe because it IS the hash).
 * `model_mentioned_pil_in_reason` is a bool computed from rank.reason via
 * regex; the reason text itself stays on rank_results.reason.
 */
export interface BrevioRankPilAppliedDetail {
  readonly rank_result_id: number;
  readonly pil_signal_kinds_present: readonly ('sender_importance' | 'sender_suppressed')[];
  readonly score_before_pil_cap: number;
  readonly score_after_pil_cap: number;
  readonly pil_score_delta: number;
  readonly pil_score_delta_was_capped: boolean;
  readonly model_mentioned_pil_in_reason: boolean;
  readonly source_surface: 'email_alert';
  readonly scope_key_hash: string;
}

export interface RankerWithPilDeps extends RankerDeps {
  readonly auditStore: AuditStore;
  // Q5.A — when false, the wrapper short-circuits to a single rankEmail call
  // with pil_context: null AND skips the brevio.rank.pil_applied audit. The
  // worker is the source of truth for the boolean (loaded from KillSwitches).
  readonly pil_live_enabled: boolean;
  // Q2.A — hard cap on absolute PIL score delta. Enforced AFTER both model
  // calls return. Founder lock: without the baseline call the cap is not real.
  readonly pil_score_cap: number;
}

export interface RankEmailWithLivePilArgs extends RankerRequest {
  /** Pre-computed via buildLivePilContext (read-side filter already applied). */
  readonly pil_context: PilContext | null;
  /** scope_key_hash — must match the canonical 32-hex HMAC shape. */
  readonly sender_email_hash: string | null;
}

export interface RankEmailWithLivePilResult {
  /** The chosen result that should be stored in rank_results (after clamp). */
  readonly result: RankerResult;
  /** Set when the audit was emitted (post rank_results write); null otherwise. */
  readonly audit_emitted_for_rank_result_id: number | null;
  /** Captured for the worker to emit AFTER it writes rank_results (rank_result_id is not known until write). */
  readonly audit_payload: Omit<BrevioRankPilAppliedDetail, 'rank_result_id'> | null;
  /** Set to the baseline RankerSuccess when the two-call hybrid ran AND BOTH calls succeeded. */
  readonly baseline_result: RankerResult | null;
  /** Set to the PIL RankerSuccess when the two-call hybrid ran AND BOTH calls succeeded. */
  readonly pil_result: RankerResult | null;
}

/**
 * Phase v0.5.12 — production rank with optional PIL two-call hybrid.
 *
 * Behavior:
 *   - When deps.pil_live_enabled is false OR args.pil_context is null OR
 *     args.sender_email_hash is null/non-canonical → SINGLE rankEmail call
 *     with pil_context: null. Behavior bit-identical to v0.5.11 (PROMPT_VERSION
 *     stays 'ranker-v0.2.0'). NO brevio.rank.pil_applied audit emitted.
 *   - Otherwise: TWO rankEmail calls — baseline (pil_context: null) AND PIL.
 *     If EITHER fails, the wrapper falls back to baseline-only result and
 *     emits NO audit. If BOTH succeed:
 *       raw_delta     = pil.score - baseline.score
 *       clamped_delta = clamp(raw_delta, -cap, +cap)
 *       was_capped    = (clamped_delta !== raw_delta)
 *       final_score   = baseline.score + clamped_delta
 *     The PIL call's reason becomes the persisted reason (it had the
 *     context); the score is the clamped final_score.
 *
 * The audit row is NOT written here — `rank_result_id` is only known after
 * the worker writes rank_results. The wrapper returns the audit payload so
 * the worker can write the audit once rank_result_id is in hand. This keeps
 * the two-write ordering visible at the call site (read +  rank_results
 * write + audit write).
 */
export async function rankEmailWithLivePil(
  args: RankEmailWithLivePilArgs,
  deps: RankerWithPilDeps
): Promise<RankEmailWithLivePilResult> {
  // Defensive: enforce the read-side filter here too — the worker should
  // already have filtered via buildLivePilContext, but a defensive second
  // check makes BB6 (legacy placeholder) impossible to leak through this
  // wrapper even if a caller bypasses the canonical buildLivePilContext.
  const senderHashCanonical =
    typeof args.sender_email_hash === 'string' && CANONICAL_SCOPE_KEY_REGEX.test(args.sender_email_hash);
  const shouldRunHybrid =
    deps.pil_live_enabled && args.pil_context !== null && senderHashCanonical;

  if (!shouldRunHybrid) {
    // Baseline-only path. Bit-identical to v0.5.11.
    const baselineOnly = await rankEmail(
      Object.freeze({ raw: args.raw, user_id: args.user_id, pil_context: null }),
      deps
    );
    return Object.freeze({
      result: baselineOnly,
      audit_emitted_for_rank_result_id: null,
      audit_payload: null,
      baseline_result: null,
      pil_result: null
    });
  }

  // Two-call hybrid. We run them sequentially (not in parallel) so that if
  // the baseline call fails fast the PIL call is skipped — saves an OpenAI
  // call on backend errors. Order is baseline first by founder rule (the
  // baseline is the anchor for the clamp).
  const baseline = await rankEmail(
    Object.freeze({ raw: args.raw, user_id: args.user_id, pil_context: null }),
    deps
  );
  if (!baseline.ok) {
    // Baseline failed; surface the failure. Per "real or absent" — do not
    // silently fall through to a PIL-only call (that would defeat the cap).
    return Object.freeze({
      result: baseline,
      audit_emitted_for_rank_result_id: null,
      audit_payload: null,
      baseline_result: null,
      pil_result: null
    });
  }

  const pilCall = await rankEmail(
    Object.freeze({
      raw: args.raw,
      user_id: args.user_id,
      pil_context: args.pil_context
    }),
    deps
  );
  if (!pilCall.ok) {
    // PIL call failed; fall back to baseline result. NO audit (PIL was not
    // applied). The rank_results row carries prompt_version='ranker-v0.2.0'.
    return Object.freeze({
      result: baseline,
      audit_emitted_for_rank_result_id: null,
      audit_payload: null,
      baseline_result: null,
      pil_result: null
    });
  }

  // Both succeeded. Compute clamped final score.
  const rawDelta = pilCall.decision.score - baseline.decision.score;
  const cap = deps.pil_score_cap;
  const clampedDelta = Math.max(-cap, Math.min(cap, rawDelta));
  const wasCapped = clampedDelta !== rawDelta;
  const finalScore = clamp01(baseline.decision.score + clampedDelta);

  // The PIL call's decision shape is the basis for the stored row (its
  // `reason` saw the prior + could mention it). We swap in the clamped score.
  const finalResult: RankerSuccess = Object.freeze({
    ok: true as const,
    decision: Object.freeze({
      label: pilCall.decision.label,
      score: finalScore,
      reason: pilCall.decision.reason
    }),
    model_name: pilCall.model_name,
    prompt_version: pilCall.prompt_version,
    latency_ms: pilCall.latency_ms,
    input_tokens: pilCall.input_tokens,
    output_tokens: pilCall.output_tokens,
    estimated_cost_usd: pilCall.estimated_cost_usd
  });

  const kindsPresent: ('sender_importance' | 'sender_suppressed')[] = [];
  if (args.pil_context!.sender_importance_n_events > 0 || args.pil_context!.sender_importance_score !== 0) {
    kindsPresent.push('sender_importance');
  }
  if (args.pil_context!.sender_suppressed) {
    kindsPresent.push('sender_suppressed');
  }

  const auditPayload: Omit<BrevioRankPilAppliedDetail, 'rank_result_id'> = Object.freeze({
    pil_signal_kinds_present: Object.freeze(kindsPresent) as readonly (
      | 'sender_importance'
      | 'sender_suppressed'
    )[],
    score_before_pil_cap: pilCall.decision.score,
    score_after_pil_cap: finalScore,
    pil_score_delta: clampedDelta,
    pil_score_delta_was_capped: wasCapped,
    model_mentioned_pil_in_reason: modelMentionedPilInReason(pilCall.decision.reason),
    source_surface: 'email_alert' as const,
    scope_key_hash: args.sender_email_hash!
  });

  return Object.freeze({
    result: finalResult,
    audit_emitted_for_rank_result_id: null,
    audit_payload: auditPayload,
    baseline_result: baseline,
    pil_result: pilCall
  });
}

/**
 * Phase v0.5.12 — write the brevio.rank.pil_applied audit AFTER rank_results
 * has been written and rank_result_id is known. Separate function to keep
 * the wrapper above pure with respect to side effects (easier to test the
 * clamp math without mocking an audit store).
 */
export async function writeBrevioRankPilAppliedAudit(
  auditStore: AuditStore,
  userId: string,
  rankResultId: number,
  payload: Omit<BrevioRankPilAppliedDetail, 'rank_result_id'>
): Promise<void> {
  await auditStore.write({
    actor_user_id: userId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'brevio.rank.pil_applied',
    target: `rank_result:${rankResultId}`,
    result: 'success',
    detail: {
      rank_result_id: rankResultId,
      pil_signal_kinds_present: payload.pil_signal_kinds_present,
      score_before_pil_cap: payload.score_before_pil_cap,
      score_after_pil_cap: payload.score_after_pil_cap,
      pil_score_delta: payload.pil_score_delta,
      pil_score_delta_was_capped: payload.pil_score_delta_was_capped,
      model_mentioned_pil_in_reason: payload.model_mentioned_pil_in_reason,
      source_surface: payload.source_surface,
      scope_key_hash: payload.scope_key_hash
    }
  });
}

function clamp01(n: number): number {
  if (!Number.isFinite(n)) return 0;
  if (n < 0) return 0;
  if (n > 1) return 1;
  return n;
}
