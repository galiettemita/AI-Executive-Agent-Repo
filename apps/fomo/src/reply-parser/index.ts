// Reply Parser — Phase 3F.1 orchestrator.
//
// Founder directive 2026-05-26:
//   "Deterministic safety/control pre-pass, then OpenAI classifier
//    for soft intents."
//
// Architecture:
//
//   1. parseReplyDeterministic(text) — frozen exact-match set for
//      compliance commands (STOP / UNSUBSCRIBE / CANCEL / START / etc).
//      If matched → return immediately, source='deterministic'.
//      LLM is NEVER consulted for these.
//
//   2. parseReplyClassifier(view, deps) — egress-redacted reply
//      context + alert context → OpenAI gpt-5-mini with strict
//      JSON schema → soft intent in
//      { snooze, ignore, ignore_sender, why, false_positive, unclear }.
//
//   3. Confidence threshold: if classifier returns confidence
//      < confidenceThreshold (default 0.7), the orchestrator FORCES
//      the intent to 'unclear' (fail-safe). The orchestrator does NOT
//      auto-retry the classifier — the founder directive: "If
//      classifier confidence is low, fail safe."
//
//   4. On classifier failure (router error, timeout, schema_invalid),
//      the orchestrator returns ReplyParseFailure. The route handler
//      audits and acknowledges the inbound webhook but does NOT
//      transition alert state past `replied`.
//
// Privacy: this module is the ONLY place the user's reply text
// reaches a model call. Egress is applied via applyEgressForReplyParser
// (subject + sender_name + message_id from alert; user_reply_text;
// nothing else). NO raw email body, NO headers, NO attachment names,
// NO PII beyond the alert-context fields the egress policy permits.

import {
  applyEgressForReplyParser,
  type ReplyParserEgressView
} from '../core/egress-policy.js';
import { type ModelRouter, type ModelRouteResult } from '../core/model-router.js';

import { countNormalizedWordTokens, parseReplyDeterministic, type DeterministicMatch } from './deterministic.js';
import { PROMPT_VERSION, buildReplyParserPrompt } from './prompt.js';
import {
  type ReplyClassification,
  type ReplyIntent,
  validateReplyClassifierOutput
} from './validator.js';

export { PROMPT_VERSION } from './prompt.js';
export type {
  DeterministicIntent,
  DeterministicMatch
} from './deterministic.js';
export type { ReplyClassification, ReplyIntent, SnoozeHint } from './validator.js';

export const DEFAULT_CONFIDENCE_THRESHOLD = 0.7;

// Phase v0.5.10 (Q3.C) — ≤3-word safe rule. Any inbound reply whose
// normalized word-token count is ≤ this threshold AND that did NOT match
// the deterministic pre-pass (compliance OR soft allowlist) is forced
// to 'unclear' regardless of the classifier's output. Protects against
// ambiguous short replies like "ok", "yes", "thanks", "got it".
//
// The allowlist absorbs CANONICAL short feedback phrases ("this mattered",
// "not important", etc.) into the deterministic path BEFORE this rule
// fires, so genuine 2-3-word feedback is still routed correctly.
export const SHORT_REPLY_SAFE_RULE_MAX_WORDS = 3;

export interface ReplyParserDeps {
  readonly router: ModelRouter;
  // Threshold below which the classifier's intent is forced to
  // 'unclear'. Default 0.7. Tunable for smoke runs.
  readonly confidenceThreshold?: number;
  // Per-call model timeout. Defaults to the router's default if omitted.
  readonly timeoutMs?: number;
}

export interface ReplyAlertContext {
  // The subject line of the original alerted email — egress-redacted
  // before being passed to the classifier.
  readonly alert_subject: string;
  readonly alert_sender_name?: string;
  // The Gmail message_id of the original email. Operational only.
  readonly alert_message_id: string;
}

export interface ReplyParserRequest {
  // Raw user reply text — straight from SendBlue's webhook payload.
  // The parser does NOT persist this text; it's used only to feed
  // the deterministic matcher + (if no deterministic match) the
  // classifier prompt.
  readonly user_reply_text: string;
  readonly alert_context: ReplyAlertContext;
  // user_id for cost-tracking attribution.
  readonly user_id: string;
}

// Parse outcomes. Three shapes:
//   * deterministic: source='deterministic', intent='stop'|'start'.
//   * classifier:    source='classifier', intent ∈ {snooze, ignore,
//                    ignore_sender, why, false_positive, unclear}
//                    + confidence, snooze_hint, model metadata.
//   * failure:       source='classifier_error', router error info.

export interface ReplyParseDeterministic {
  readonly ok: true;
  readonly source: 'deterministic';
  readonly intent: DeterministicMatch['intent'];
}

export interface ReplyParseClassifier {
  readonly ok: true;
  readonly source: 'classifier';
  readonly classification: ReplyClassification;
  readonly model_name: string;
  readonly prompt_version: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
  // True when the classifier's raw intent was forced to 'unclear'
  // by the confidence-threshold fail-safe. Visible in audit so ops
  // can see WHY we fell to unclear.
  readonly low_confidence_forced_unclear: boolean;
}

export interface ReplyParseFailure {
  readonly ok: false;
  readonly source: 'classifier_error';
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

export type ReplyParseResult =
  | ReplyParseDeterministic
  | ReplyParseClassifier
  | ReplyParseFailure;

export async function parseReply(
  req: ReplyParserRequest,
  deps: ReplyParserDeps
): Promise<ReplyParseResult> {
  // ─── Pass 1: deterministic safety/control pre-pass ─────────────
  // Per the founder directive, STOP / UNSUBSCRIBE / CANCEL / START
  // must be handled deterministically. The matcher returns null
  // when the input is not a bare compliance command — natural
  // phrasings like "please stop" fall through to the classifier.
  const det = parseReplyDeterministic(req.user_reply_text);
  if (det) {
    return Object.freeze({
      ok: true as const,
      source: 'deterministic' as const,
      intent: det.intent
    });
  }

  // ─── Pass 2: OpenAI classifier (egress-redacted context) ───────
  const view: ReplyParserEgressView = applyEgressForReplyParser(req.user_reply_text, {
    subject: req.alert_context.alert_subject,
    sender_name: req.alert_context.alert_sender_name,
    message_id: req.alert_context.alert_message_id
  });
  const prompt = buildReplyParserPrompt(view);

  const routed: ModelRouteResult<ReplyClassification> =
    await deps.router.route<ReplyClassification>({
      capability: 'classification',
      prompt,
      prompt_version: PROMPT_VERSION,
      user_id: req.user_id,
      validate: validateReplyClassifierOutput,
      timeout_ms: deps.timeoutMs
    });

  if (!routed.ok) {
    return Object.freeze({
      ok: false as const,
      source: 'classifier_error' as const,
      code: routed.code,
      reason: routed.reason,
      model_name: routed.model_name,
      prompt_version: PROMPT_VERSION
    });
  }

  // ─── Pass 3: confidence-threshold fail-safe ────────────────────
  const threshold = deps.confidenceThreshold ?? DEFAULT_CONFIDENCE_THRESHOLD;
  const rawClassification = routed.output;
  let classification: ReplyClassification;
  let forcedUnclear = false;
  if (rawClassification.confidence < threshold && rawClassification.intent !== 'unclear') {
    // Per founder directive: "If classifier confidence is low, fail safe."
    // Force the intent to 'unclear' regardless of the model's pick.
    classification = Object.freeze({
      intent: 'unclear' as ReplyIntent,
      confidence: rawClassification.confidence,
      reason: `forced_unclear: classifier picked '${rawClassification.intent}' with confidence ${rawClassification.confidence.toFixed(2)} < ${threshold}`,
      snooze_hint: null
    });
    forcedUnclear = true;
  } else {
    classification = rawClassification;
  }

  // ─── Pass 4 (Phase v0.5.10 Q3.C): ≤3-word safe rule ──────────────
  // Belt-and-suspenders on top of the confidence threshold. If the
  // reply is ≤ SHORT_REPLY_SAFE_RULE_MAX_WORDS words AND we got past
  // the deterministic pre-pass (so we know it didn't match the
  // explicit-feedback-phrase allowlist OR a compliance command) AND
  // the classifier produced a non-unclear intent, force it to unclear.
  // Protects against the LLM confidently misclassifying short ambiguous
  // replies like "ok" / "thanks" / "got it".
  if (
    classification.intent !== 'unclear' &&
    countNormalizedWordTokens(req.user_reply_text) <= SHORT_REPLY_SAFE_RULE_MAX_WORDS
  ) {
    classification = Object.freeze({
      intent: 'unclear' as ReplyIntent,
      confidence: classification.confidence,
      reason: `forced_unclear: ≤${SHORT_REPLY_SAFE_RULE_MAX_WORDS}_word_safe_rule (classifier picked '${classification.intent}' but reply too short + not in allowlist)`,
      snooze_hint: null
    });
    forcedUnclear = true;
  }

  return Object.freeze({
    ok: true as const,
    source: 'classifier' as const,
    classification,
    model_name: routed.model_name,
    prompt_version: PROMPT_VERSION,
    latency_ms: routed.latency_ms,
    input_tokens: routed.input_tokens,
    output_tokens: routed.output_tokens,
    estimated_cost_usd: routed.estimated_cost_usd,
    low_confidence_forced_unclear: forcedUnclear
  });
}

// Map a snooze_hint to a concrete duration in seconds. The route
// handler computes snooze_until = now + computeSnoozeDurationSeconds(hint).
// All values are conservative defaults; v0.1 doesn't tune them per
// user. Future phases can read timing_preference memory_signals to
// personalize.
export function computeSnoozeDurationSeconds(hint: ReplyClassification['snooze_hint']): number {
  switch (hint) {
    case 'later':
      return 60 * 60; // 1h
    case 'tomorrow':
      return 24 * 60 * 60; // 24h
    case 'remind_me_later':
      return 4 * 60 * 60; // 4h
    case 'unspecified':
    case null:
      return 60 * 60; // default 1h
  }
}
