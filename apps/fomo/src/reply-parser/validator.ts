// Reply-parser classifier validator — Phase 3F.1.
//
// Plugs into ModelRouter.route()'s `validate` slot. Returns
// { ok: true, value: ReplyClassification } on a valid single-line
// JSON object matching the reply-parser schema; { ok: false, reason }
// otherwise.
//
// The router turns { ok: false } into ModelRouteError(schema_invalid),
// records the cost (the call did consume tokens), and surfaces the
// reason to the caller. The reply-parser orchestrator re-checks ok
// before acting.
//
// Defense-in-depth: OpenAI strict mode (response_format) enforces
// the schema server-side, but the validator runs anyway because:
//   1. Mock-backend tests need the validator without OpenAI in the loop.
//   2. OpenAI strict mode doesn't accept min/max on numbers, so the
//      0..1 confidence bound is enforced here.
//   3. Older/cheaper models we may bake-off later don't all honor
//      json_schema strict mode.

import { type ModelOutputValidator } from '../core/model-router.js';

export type ReplyIntent =
  | 'snooze'
  | 'ignore'
  | 'ignore_sender'
  | 'why'
  | 'false_positive'
  // Phase v0.5.10 (Q2.A-modified) — positive-signal intents.
  // `this_mattered`: user is CONFIRMING the alert was right (positive
  // confirmation; NOT a negative correction). Mapping: verb='approved',
  // detail.dimension='importance', detail.value='confirmed_important'.
  // `more_like_this`: user wants more alerts of this shape. Mapping:
  // verb='approved', detail.dimension='pattern',
  // detail.value='more_like_this'.
  // BOTH are WRITE-ONLY this phase: feedback_event written, but NO
  // memory_signal upsert (Q4.A lock). PIL / positive-signal phases
  // decide consumption later.
  | 'this_mattered'
  | 'more_like_this'
  | 'unclear';

export const REPLY_INTENTS: readonly ReplyIntent[] = Object.freeze([
  'snooze',
  'ignore',
  'ignore_sender',
  'why',
  'false_positive',
  // Phase v0.5.10 — positive-signal additions.
  'this_mattered',
  'more_like_this',
  'unclear'
]);

export type SnoozeHint = 'later' | 'tomorrow' | 'remind_me_later' | 'unspecified' | null;

export const SNOOZE_HINTS_NON_NULL = Object.freeze<readonly Exclude<SnoozeHint, null>[]>([
  'later',
  'tomorrow',
  'remind_me_later',
  'unspecified'
]);

export function isReplyIntent(value: unknown): value is ReplyIntent {
  return typeof value === 'string' && (REPLY_INTENTS as readonly string[]).includes(value);
}

export interface ReplyClassification {
  readonly intent: ReplyIntent;
  readonly confidence: number;
  readonly reason: string;
  readonly snooze_hint: SnoozeHint;
}

// Some models wrap JSON in code fences despite the instruction. Strip
// fence wrappers (``` or ```json) before parsing.
function stripCodeFence(text: string): string {
  const trimmed = text.trim();
  const fenceMatch = /^```(?:json)?\s*\n?([\s\S]*?)\n?```\s*$/.exec(trimmed);
  return fenceMatch ? (fenceMatch[1] ?? '').trim() : trimmed;
}

const MAX_REASON_LEN = 240;

export const validateReplyClassifierOutput: ModelOutputValidator<ReplyClassification> = (text) => {
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

  const intent = obj.intent;
  if (!isReplyIntent(intent)) {
    return {
      ok: false,
      reason: `intent must be one of [${REPLY_INTENTS.join(', ')}], got ${JSON.stringify(intent)}`
    };
  }

  const confidence = obj.confidence;
  if (typeof confidence !== 'number' || !Number.isFinite(confidence) || confidence < 0 || confidence > 1) {
    return {
      ok: false,
      reason: `confidence must be a number in [0, 1], got ${JSON.stringify(confidence)}`
    };
  }

  const reason = obj.reason;
  if (typeof reason !== 'string') {
    return { ok: false, reason: `reason must be a string, got ${typeof reason}` };
  }
  const truncatedReason =
    reason.length > MAX_REASON_LEN ? `${reason.slice(0, MAX_REASON_LEN - 1)}…` : reason;

  // snooze_hint: must be one of the allowed strings OR null. When
  // intent !== 'snooze' we normalize to null even if the model echoed
  // a hint — the downstream code only consumes it for snooze.
  const rawHint = obj.snooze_hint;
  let snoozeHint: SnoozeHint;
  if (rawHint === null || rawHint === undefined) {
    snoozeHint = null;
  } else if (typeof rawHint === 'string') {
    if (!(SNOOZE_HINTS_NON_NULL as readonly string[]).includes(rawHint)) {
      return {
        ok: false,
        reason: `snooze_hint must be one of [${SNOOZE_HINTS_NON_NULL.join(', ')}] or null, got ${JSON.stringify(rawHint)}`
      };
    }
    snoozeHint = rawHint as Exclude<SnoozeHint, null>;
  } else {
    return {
      ok: false,
      reason: `snooze_hint must be a string or null, got ${typeof rawHint}`
    };
  }
  // Normalize: non-snooze intent → null hint regardless of what the
  // model said. The orchestrator never reads hint for non-snooze.
  if (intent !== 'snooze') snoozeHint = null;

  return {
    ok: true,
    value: Object.freeze({
      intent,
      confidence,
      reason: truncatedReason,
      snooze_hint: snoozeHint
    })
  };
};
