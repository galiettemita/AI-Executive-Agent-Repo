// Phase v0.5.10 (Q1.B founder lock) — Reply-parser feedback routing module.
//
// SINGLE TESTABLE PLACE for the policy "intent → feedback_event_input
// + applyFeedback decision". The reply-parser stays stateless (intent
// in → classified intent out); this module knows how each intent maps
// to the v0.5.9 Brevio-wide Feedback substrate.
//
// Architecture:
//
//   routeReplyFeedback(input, deps): Promise<RouteOutcome>
//
//   * Resolves the (verb, dimension, role, value?, ...detail) tuple
//     for the intent (see INTENT_MAPPING table below)
//   * Writes the feedback_event via FeedbackStore.write — with the
//     v0.5.9-locked source_surface='email_alert' default + extended
//     v0.5.10 detail fields
//   * Emits the v0.5.10-extended feedback.written audit per Q6.A-modified
//     (the 10-field detail schema; NEVER raw reply text)
//   * Invokes applyFeedback ONLY for intent='ignore_sender' per Q4.A
//     founder lock (v0.5.10 does NOT add new applyFeedback consumer
//     arms; positive intents are feedback_events only)
//
// Intents NOT handled by this module:
//   * 'stop' / 'start' — deterministic compliance/control path
//     (existing v0.5.5 applyStop / applyStart paths in sendblue-inbound.ts).
//     Per founder lock: STOP/START NEVER become preference feedback.
//   * 'unclear' — the route handler's applyUnclear() continues to walk
//     the state machine to 'replied' and audit fomo.sendblue.reply_unclear.
//     This module returns { kind: 'unclear_no_op' } so the route handler
//     can short-circuit the feedback write side.
//
// PRIVACY GUARDRAILS (founder runtime lock 2026-06-06):
//   * NO raw reply text in feedback_events, audit detail, memory_signal
//     detail, logs, or fixtures. The module accepts intent_source +
//     parser_intent + parser_confidence (enum/numeric metadata) only.
//   * NO new applyFeedback consumer arms beyond v0.5.9's
//     (email_alert, ignored, sender) → sender_feedback_ignored.
//   * Positive intents (this_mattered / more_like_this) write
//     feedback_events but NEVER trigger memory_signal upsert in v0.5.10.

import type { AuditStore } from '../core/audit.js';
import { applyFeedback, type AppliedFeedbackResult } from '../memory/feedback-apply.js';
import {
  type BrevioFeedbackEventKind,
  type FeedbackEvent,
  type FeedbackStore,
  mapLegacyFeedbackKind
} from '../memory/feedback-events.js';
import type { MemorySignalStore } from '../memory/memory-signals.js';
import type { SendOutcome } from '../adapters/sendblue/client.js';

import {
  isAckableFeedbackIntent,
  renderFeedbackAck
} from './feedback-ack-template.js';
import type { ReplyIntent } from './validator.js';

/* ---------------------------------------------------------------------- */
/* Input + output types                                                   */
/* ---------------------------------------------------------------------- */

// `IntentSource` distinguishes the path that produced the intent.
// Locked per Q6.A-modified founder direction. The set is fixed at v0.5.10;
// future surfaces add new values here only.
export type IntentSource =
  | 'reply_parser_classifier'        // LLM classifier path
  | 'reply_parser_deterministic'     // compliance OR explicit-feedback allowlist
  | 'slack_interactivity'            // founder Slack approve/reject (v0.5.9)
  | 'ops_inject';                    // ops:feedback-inject CLI (v0.5.9)

export interface RouteReplyFeedbackInput {
  readonly user_id: string;
  readonly intent: Exclude<ReplyIntent, 'unclear'>; // unclear caller short-circuits before calling us
  readonly intent_source: Extract<IntentSource, 'reply_parser_classifier' | 'reply_parser_deterministic'>;
  readonly parser_confidence: number;        // 0..1; 1.0 for deterministic
  readonly inbound_reply_id: number;
  readonly alert_id: string | null;
  // The sender_email of the original alerted email (NOT the reply
  // sender). This is the existing v0.5.x feedback_events.sender_email
  // column convention — preserved as-is for backward compat. For the
  // ignore_sender consumer arm, this is the value HMAC-hashed into the
  // memory_signals.scope_key by applyFeedback.
  readonly sender_email: string | null;
  // Optional snooze hint, forwarded for the snooze intent only.
  readonly snooze_hint?: 'later' | 'tomorrow' | 'remind_me_later' | 'unspecified' | null;
  // Phase v0.5.14 — optional E.164 phone number of the inbound reply
  // sender (the user). Used ONLY to send the acknowledgment SMS via
  // deps.feedbackAck. NEVER logged, NEVER persisted by this module,
  // NEVER included in feedback_event detail or audit detail. When
  // undefined (or when deps.feedbackAck is undefined, or when intent
  // is not ackable), no ack is sent.
  readonly from_number?: string;
}

// Phase v0.5.14 — outcome details surfaced when the optional feedback-
// acknowledgment send fires. `attempted` covers "we tried" regardless
// of provider outcome; `ack` is null when we did NOT attempt (intent
// not ackable, dep not wired, from_number missing). When attempted=true,
// `ack.send_outcome_kind` differentiates 'sent' (provider acked) from
// 'failed'/'send_status_unknown' (provider rejected or network). The
// audit_log is the load-bearing record; this field exists so the route
// handler / tests can inspect the outcome without re-querying.
export interface FeedbackAckRouteOutcome {
  readonly attempted: boolean;
  readonly template_version: string | null;
  readonly send_outcome_kind: SendOutcome['kind'] | null;
  readonly threw: boolean;
}

export type RouteOutcome =
  | {
      readonly kind: 'wrote';
      readonly feedback_event: FeedbackEvent;
      readonly applied: AppliedFeedbackResult | null; // null when consumer was not invoked
      // Phase v0.5.14 — non-null only when the routing module entered
      // the ack send path (intent ackable + feedbackAck dep wired +
      // from_number present). Always non-null on the 'wrote' outcome
      // so callers can inspect "we attempted: yes/no" + outcome shape.
      readonly ack: FeedbackAckRouteOutcome;
    }
  | { readonly kind: 'unclear_no_op' };

// Phase v0.5.14 — optional SendBlue-backed acknowledgment send. Same
// shape as v0.5.5 stopConfirmation dep (single send(to, content)
// method returning SendOutcome). The route handler injects this with
// the SendBlue client closed over; tests inject a mock that records
// invocations.
export interface FeedbackAckDep {
  readonly send: (args: { to: string; content: string }) => Promise<SendOutcome>;
}

export interface RouteReplyFeedbackDeps {
  readonly feedbackStore: FeedbackStore;
  readonly auditStore: AuditStore;
  readonly memoryStore: MemorySignalStore;
  // HMAC key for the v0.5.9 consumer's scope_key derivation. Reused
  // here for the ignore_sender → sender_feedback_ignored upsert.
  readonly senderHashKey: Buffer;
  readonly now?: () => number;
  // Phase v0.5.14 — optional acknowledgment send. When absent, the
  // routing module behaves exactly as v0.5.10 (no ack send, no
  // fomo.sendblue.feedback_ack_* audit). When present, ackable intents
  // (per isAckableFeedbackIntent) with a non-null input.from_number
  // produce a best-effort deterministic ack SMS.
  readonly feedbackAck?: FeedbackAckDep;
}

/* ---------------------------------------------------------------------- */
/* Intent → feedback_event_input mapping (founder-locked Q2.A-modified)   */
/* ---------------------------------------------------------------------- */

interface IntentMapping {
  // The generic Brevio verb (kind column literal). Stored directly —
  // no legacy-kind indirection for the reply-parser path. The v0.5.9
  // mapLegacyFeedbackKind helper continues to support kernel-test +
  // Slack-interactivity legacy callers.
  readonly verb: BrevioFeedbackEventKind;
  // Whether this intent should fire the v0.5.9 consumer (applyFeedback).
  // Per Q4.A founder lock: ONLY ignore_sender triggers in v0.5.10.
  readonly fires_apply_feedback: boolean;
  // Static detail overlay applied to every event of this intent.
  // The route handler may merge additional context (snooze_hint, etc).
  readonly detail_overlay: Record<string, unknown>;
}

// The locked policy table. ONE place to evolve when new intents land.
const INTENT_MAPPING: Readonly<Record<Exclude<ReplyIntent, 'unclear'>, IntentMapping>> = Object.freeze({
  snooze: {
    verb: 'snoozed',
    fires_apply_feedback: false,
    detail_overlay: { dimension: 'alert', role: 'user' }
  },
  ignore: {
    verb: 'ignored',
    fires_apply_feedback: false,
    detail_overlay: { dimension: 'alert', role: 'user' }
  },
  ignore_sender: {
    verb: 'ignored',
    // THE v0.5.10 Q5.B → v0.5.9 (email_alert, ignored, sender) consumer arm.
    // The ONLY intent that triggers applyFeedback in v0.5.10.
    fires_apply_feedback: true,
    detail_overlay: { dimension: 'sender', role: 'user' }
  },
  why: {
    verb: 'asked_why',
    fires_apply_feedback: false,
    detail_overlay: { role: 'user' }
  },
  false_positive: {
    // Per Q2 founder correction: vocabulary is 'important'/'not_important'
    // (truth labels the user is asserting), NOT 'positive'/'negative'
    // (model output labels). See project_v05-10-scope.md Q2.A-modified.
    verb: 'corrected',
    fires_apply_feedback: false,
    detail_overlay: {
      role: 'user',
      dimension: 'ranker_label',
      previous_label: 'important',
      corrected_label: 'not_important'
    }
  },
  // Phase v0.5.10 Q2.A-modified — positive-signal intents. Both write
  // feedback_events with verb='approved' + a positive-signal dimension/
  // value pair. NEITHER triggers applyFeedback in v0.5.10 (PIL / a
  // positive-signal phase decides consumption later).
  this_mattered: {
    verb: 'approved',
    fires_apply_feedback: false,
    detail_overlay: {
      role: 'user',
      dimension: 'importance',
      value: 'confirmed_important'
    }
  },
  more_like_this: {
    verb: 'approved',
    fires_apply_feedback: false,
    detail_overlay: {
      role: 'user',
      dimension: 'pattern',
      value: 'more_like_this'
    }
  }
});

/* ---------------------------------------------------------------------- */
/* routeReplyFeedback — the single policy entry point                     */
/* ---------------------------------------------------------------------- */

export async function routeReplyFeedback(
  input: RouteReplyFeedbackInput,
  deps: RouteReplyFeedbackDeps
): Promise<RouteOutcome> {
  const mapping = INTENT_MAPPING[input.intent];
  // mapping is always defined — TS narrows from the Exclude<ReplyIntent,
  // 'unclear'> input type. Defensive null check kept out so the runtime
  // misuse surfaces as a TS error, not a silent no-op.

  // Merge mapping overlay with caller-supplied context (snooze_hint).
  // Privacy: detail is STRUCTURED METADATA ONLY. The caller never passes
  // raw reply text into this module; the module never accepts it.
  const detail: Record<string, unknown> = { ...mapping.detail_overlay };
  if (input.intent === 'snooze' && input.snooze_hint !== undefined) {
    detail.snooze_hint = input.snooze_hint ?? null;
  }

  // Write the feedback_event through the v0.5.9 active-surface gate.
  // source_surface='email_alert' is locked at v0.5.10 (no new active
  // surfaces this phase). The gate would reject any other value.
  const writtenEvent = await deps.feedbackStore.write({
    user_id: input.user_id,
    alert_id: input.alert_id,
    sender_email: input.sender_email,
    kind: mapping.verb,
    source_surface: 'email_alert',
    detail
  });

  // Emit the v0.5.10-extended feedback.written audit per Q6.A-modified.
  // 10-field detail schema. NEVER raw reply text — the parser_intent
  // is the canonical enum-shaped identifier; inbound_reply_id is the
  // numeric forward-link to inbound_replies for correlation.
  const mappedLegacy = mapLegacyFeedbackKind(mapping.verb);
  await deps.auditStore.write({
    actor_user_id: input.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'feedback.written',
    target: input.alert_id ? `alert:${input.alert_id}` : 'route:sendblue.inbound',
    result: 'success',
    detail: {
      feedback_event_id: writtenEvent.id ?? null,
      source_surface: writtenEvent.source_surface,
      verb: mapping.verb,
      // dimension may be undefined for some intents (e.g. 'why'); use null
      // for JSONB cleanliness — JSON.stringify drops undefined, postgres
      // jsonb stores null explicitly.
      dimension: (detail.dimension as string | undefined) ?? null,
      role: (detail.role as string | undefined) ?? null,
      // legacy_kind: present only when the mapping VERB happens to match
      // a legacy kind shape (it never will for the reply-parser path
      // because we write generic verbs directly; included structurally
      // for the audit schema contract).
      legacy_kind: mappedLegacy ? mapping.verb : null,
      intent_source: input.intent_source,
      inbound_reply_id: input.inbound_reply_id,
      parser_intent: input.intent,
      parser_confidence: input.parser_confidence,
      sender_present: writtenEvent.sender_email !== null
    }
  });

  // Q4.A consumer-arm gate: ONLY ignore_sender fires applyFeedback in
  // v0.5.10. Other intents (including the new positive-signal intents)
  // are feedback_events only.
  let applied: AppliedFeedbackResult | null = null;
  if (mapping.fires_apply_feedback) {
    applied = await applyFeedback(writtenEvent, {
      memoryStore: deps.memoryStore,
      auditStore: deps.auditStore,
      senderHashKey: deps.senderHashKey,
      now: deps.now
    });
  }

  // Phase v0.5.14 — HMR Feedback Acknowledgment Surface (best-effort).
  //
  // Fires ONLY when ALL of these hold:
  //   (1) deps.feedbackAck is wired (route handler injected the SendBlue
  //       client; tests may omit for parity with v0.5.10 baseline)
  //   (2) input.from_number is present (route handler passes the inbound
  //       E.164; absent in older v0.5.10 callers)
  //   (3) input.intent is in the locked ackable set (ignore_sender /
  //       this_mattered / more_like_this / false_positive)
  //
  // Mirrors v0.5.5 STOP-confirmation semantics:
  //   - separate success audit (fomo.sendblue.feedback_ack_sent)
  //   - separate failure audit (fomo.sendblue.feedback_ack_failed)
  //   - best-effort: throw/failure NEVER aborts; the feedback_event +
  //     applyFeedback writes are already durable above
  //   - no retry (v0.5.14 ships single-shot ack; future tier
  //     classification considers retry)
  //
  // Privacy: audit detail carries parser_intent + template_version +
  // feedback_event_id + send_outcome_kind only. The rendered body is
  // STATIC per template_version (no PII, no metadata) so it's safe to
  // include the version constant in the audit; the body text itself is
  // not stored. from_number flows through to deps.feedbackAck.send but
  // is never logged or persisted by this module.
  const ackOutcome: FeedbackAckRouteOutcome = await sendFeedbackAck({
    deps,
    input,
    feedback_event_id: writtenEvent.id ?? null
  });

  return Object.freeze({
    kind: 'wrote' as const,
    feedback_event: writtenEvent,
    applied,
    ack: ackOutcome
  });
}

// Helper kept inline-private to feedback-routing — the ack-send policy
// belongs with the rest of the routing decisions. Returns a structured
// outcome the route handler / tests can inspect; the audit_log row is
// the load-bearing record.
async function sendFeedbackAck(args: {
  deps: RouteReplyFeedbackDeps;
  input: RouteReplyFeedbackInput;
  feedback_event_id: number | null;
}): Promise<FeedbackAckRouteOutcome> {
  const { deps, input, feedback_event_id } = args;

  // Gate 1-3: dep wired + from_number present + intent ackable.
  if (
    deps.feedbackAck === undefined ||
    input.from_number === undefined ||
    !isAckableFeedbackIntent(input.intent)
  ) {
    return Object.freeze({
      attempted: false,
      template_version: null,
      send_outcome_kind: null,
      threw: false
    });
  }

  const rendered = renderFeedbackAck(input.intent);

  let outcome: SendOutcome;
  try {
    outcome = await deps.feedbackAck.send({
      to: input.from_number,
      content: rendered.body
    });
  } catch (err) {
    // Provider client threw (network/abort before HTTP boundary). The
    // SendBlueClient.send normally returns a SendOutcome with
    // kind='send_status_unknown' for these, but defense-in-depth catches
    // the throw too — mirrors v0.5.5 applyStop's try/catch around
    // stopConfirmation.send.
    await deps.auditStore.write({
      actor_user_id: input.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.feedback_ack_failed',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        parser_intent: input.intent,
        template_version: rendered.template_version,
        feedback_event_id,
        error_code: 'send_throw',
        // Bounded message; never the full stack or any raw reply text.
        // SendBlueClient already redacts API keys before throwing.
        error_message: (err instanceof Error ? err.message : String(err)).slice(0, 200)
      }
    });
    return Object.freeze({
      attempted: true,
      template_version: rendered.template_version,
      send_outcome_kind: null,
      threw: true
    });
  }

  // Provider returned a SendOutcome. 'sent' → success audit; everything
  // else ('failed', 'send_status_unknown') → failure audit. The route
  // handler does NOT retry either way.
  if (outcome.kind === 'sent') {
    await deps.auditStore.write({
      actor_user_id: input.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.feedback_ack_sent',
      target: 'route:sendblue.inbound',
      result: 'success',
      detail: {
        parser_intent: input.intent,
        template_version: rendered.template_version,
        feedback_event_id,
        send_outcome_kind: outcome.kind
      }
    });
  } else {
    await deps.auditStore.write({
      actor_user_id: input.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.feedback_ack_failed',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        parser_intent: input.intent,
        template_version: rendered.template_version,
        feedback_event_id,
        send_outcome_kind: outcome.kind,
        // Sanitized provider error fields (already redacted by
        // SendBlueClient — see SendBlueProviderError contract).
        provider_status: outcome.providerStatus ?? null,
        http_status: outcome.httpStatus
      }
    });
  }

  return Object.freeze({
    attempted: true,
    template_version: rendered.template_version,
    send_outcome_kind: outcome.kind,
    threw: false
  });
}

// Exported for unit-test introspection. Tests assert each intent maps
// to the locked verb + dimension + role + fires_apply_feedback flag.
export const _INTENT_MAPPING_FOR_TESTS = INTENT_MAPPING;
