// Ranker output validator — Phase 3C.1.
//
// Plugs into ModelRouter.route()'s `validate` slot. Returns
// { ok: true, value: RankDecision } on a valid single-line JSON object
// matching the ranker schema; { ok: false, reason } otherwise.
//
// The router turns { ok: false } into ModelRouteError(schema_invalid),
// records the cost (the call did consume tokens), and surfaces the
// reason to the caller. The ranker module re-checks ok before acting.

import { type ModelOutputValidator } from '../core/model-router.js';

export type RankLabel = 'important' | 'not_important';

export interface RankDecision {
  readonly label: RankLabel;
  readonly score: number;
  readonly reason: string;
}

// Some models wrap JSON in code fences despite the instruction. Strip
// fence wrappers (``` or ```json) before parsing. Conservative: only
// strips the wrapper, never modifies the inner content.
function stripCodeFence(text: string): string {
  const trimmed = text.trim();
  const fenceMatch = /^```(?:json)?\s*\n?([\s\S]*?)\n?```\s*$/.exec(trimmed);
  return fenceMatch ? (fenceMatch[1] ?? '').trim() : trimmed;
}

const MAX_REASON_LEN = 240;

export const validateRankerOutput: ModelOutputValidator<RankDecision> = (text) => {
  const cleaned = stripCodeFence(text);
  if (cleaned.length === 0) {
    return { ok: false, reason: 'empty model output' };
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(cleaned);
  } catch (err) {
    return {
      ok: false,
      reason: `output is not JSON: ${err instanceof Error ? err.message : String(err)}`
    };
  }

  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return { ok: false, reason: 'output is not a JSON object' };
  }

  const obj = parsed as Record<string, unknown>;

  const label = obj.label;
  if (label !== 'important' && label !== 'not_important') {
    return {
      ok: false,
      reason: `label must be "important" or "not_important", got ${JSON.stringify(label)}`
    };
  }

  const score = obj.score;
  if (typeof score !== 'number' || !Number.isFinite(score) || score < 0 || score > 1) {
    return {
      ok: false,
      reason: `score must be a number in [0, 1], got ${JSON.stringify(score)}`
    };
  }

  const reason = obj.reason;
  if (typeof reason !== 'string') {
    return { ok: false, reason: `reason must be a string, got ${typeof reason}` };
  }
  // Truncate overlong reasons rather than fail-close — the substance is
  // still usable as ranker explanation, and ops would rather see a
  // truncated string than discard the call.
  const truncatedReason =
    reason.length > MAX_REASON_LEN ? `${reason.slice(0, MAX_REASON_LEN - 1)}…` : reason;

  return {
    ok: true,
    value: Object.freeze({
      label,
      score,
      reason: truncatedReason
    })
  };
};
