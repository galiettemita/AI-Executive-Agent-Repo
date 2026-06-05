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

// v0.5.6: aligned with RANKER_REASON_MAX_LEN in openai-response-format.ts.
// OpenAI strict mode enforces this maxLength server-side; the validator
// enforces it client-side as defense-in-depth.
//
// Founder-locked length policy 2026-06-05: reason budget ≤ 180 chars so
// the deterministic shell (sender + subject) can fit the rendered body
// inside the 220–280 char target / 320 hard cap.
const MAX_REASON_LEN = 180;

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
  // v0.5.6: fail-closed on length violation. Previously truncated with
  // ellipsis ("…"), which is now explicitly forbidden by founder Q4 lock
  // 2026-06-05 ("no arbitrary ellipsis from truncation"). Strict-mode
  // OpenAI should not produce reasons > MAX_REASON_LEN; this is defense-
  // in-depth for non-strict backends or schema-bypass edge cases. When
  // this fires, the call is rerouted through the existing
  // `fomo.rank.failed` audit path (gmail-poll.ts), the alert is NOT
  // created, and the next poll cycle re-ranks via the model router's
  // schema_invalid → ModelRouteError pathway. The v0.5.6
  // `fomo.alert.drafter_schema_failed` audit kind (registered in
  // audit.ts) is the BODY-RENDER-layer defense-in-depth, separate
  // from this validator-layer check.
  if (reason.length > MAX_REASON_LEN) {
    return {
      ok: false,
      reason: `reason length ${reason.length} exceeds MAX_REASON_LEN=${MAX_REASON_LEN}`
    };
  }

  return {
    ok: true,
    value: Object.freeze({
      label,
      score,
      reason
    })
  };
};
