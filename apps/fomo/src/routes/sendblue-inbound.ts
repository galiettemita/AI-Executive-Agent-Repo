// SendBlue inbound webhook — Phase 3F.1.
//
// One endpoint:
//
//   POST /sendblue/inbound
//     Auth: SendBlue webhook HMAC signature verification (NOT session auth).
//     The request comes from SendBlue's servers, signed with the
//     account's webhook signing secret. We MUST verify the signature
//     before parsing the payload.
//
// Defense-in-depth layers (mirrors slack-interactivity.ts pattern):
//
//   1. Audit `fomo.sendblue.inbound_received` for every inbound POST,
//      BEFORE signature verification. Even a flood of unsigned
//      requests is visible in the audit log.
//   2. Kill switch — re-checks `FOMO_SENDBLUE_INBOUND_ENABLED` at
//      request time. Returns 200 (no info leak) but audits
//      `fomo.sendblue.kill_switch_off` and processes nothing.
//   3. Signature verification via `verifySendBlueSignature`.
//   4. Reject malformed payloads (missing required fields).
//   5. Reject wrong from-number (not the founder's phone).
//   6. Idempotency: insert into inbound_replies ON CONFLICT
//      (provider_message_id) DO NOTHING. On conflict, audit
//      `fomo.sendblue.reply_duplicate` and return 200. SendBlue
//      retries on non-200 — the route MUST be safe to receive the
//      same payload N times.
//   7. Parser-pass (deterministic safety pre-pass → classifier).
//      Per founder directive 2026-05-26, STOP / UNSUBSCRIBE / CANCEL /
//      START are handled deterministically; soft intents go through
//      the OpenAI classifier with low-confidence fail-safe.
//   8. Apply outcome:
//      - 'stop' (deterministic):  feedback event + memory_signal
//        `stop_active=true` + audit `fomo.sendblue.stop_recorded`.
//        No alert-state transition (STOP is a global compliance
//        signal, not a per-alert intent).
//      - 'start' (deterministic): memory_signal `stop_active=false`
//        + audit `fomo.sendblue.start_recorded`. No feedback event.
//      - 'snooze' (classifier):   feedback event `user_snoozed` with
//        `snooze_until` in detail + alert state `sent → replied →
//        snoozed`. NO actual re-surface — that's deferred to a
//        future phase (founder directive 2026-05-26).
//      - 'ignore' (classifier):   feedback event `user_ignored` +
//        alert state `sent → replied → ignored`.
//      - 'ignore_sender':         feedback event `ignored_sender` +
//        memory_signal `sender_suppressed` for the sender +
//        alert state `sent → replied → ignored`.
//      - 'why':                   feedback event `asked_why` +
//        alert state `sent → replied` (no terminal transition).
//      - 'false_positive':        feedback event `false_positive` +
//        alert state `sent → replied → ignored`.
//      - 'unclear':               no state transition past `replied`,
//        audit `fomo.sendblue.reply_unclear`. Operator inspects.
//
// Privacy invariants in audit + feedback rows:
//   * NEVER persist the raw inbound webhook payload string.
//   * NEVER persist the full from-phone (only a 4-char from_slug).
//   * NEVER persist the founder's raw reply text in audit detail.
//     The reply text reaches the classifier prompt (egress-redacted
//     view) but does NOT land in audit/feedback/memory.
//   * NEVER include the SendBlue signing secret in any output.
//
// Response shape:
//   * 200 with empty JSON for happy path / idempotent duplicate /
//     kill-switch-off (avoid leaking whether the route is active).
//   * 401 when signature verification fails (SendBlue will not retry).
//   * 400 when payload is malformed (no retry).
//   * 403 when authorization fails (wrong from-number).
//   * 500 only on truly unexpected internal error (SendBlue WILL retry).

import { Buffer } from 'node:buffer';
import type http from 'node:http';

import { verifySendBlueSignature } from '../adapters/sendblue/client.js';
import { type AuditStore } from '../core/audit.js';
import { type AlertStateTransitionStore } from '../core/alert-state-transitions.js';
import { type KillSwitches } from '../core/kill-switches.js';
import {
  transition,
  type AlertState
} from '../core/state-machine.js';
import { type Alert, type AlertStore } from '../memory/alerts.js';
import { type FeedbackStore, type FeedbackEventKind } from '../memory/feedback-events.js';
import { type InboundReplyStore } from '../memory/inbound-replies.js';
import {
  type MemorySignalStore
} from '../memory/memory-signals.js';
import { type RankResultStore } from '../memory/rank-results.js';
import {
  type ReplyParseResult,
  type ReplyParserRequest,
  computeSnoozeDurationSeconds
} from '../reply-parser/index.js';

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface SendBlueInboundRouteDeps {
  // SendBlue webhook signing secret. Different from the API
  // key/secret used for outbound sends — webhook signing is per-
  // webhook, rotated independently in the SendBlue dashboard.
  readonly signingSecret: string;
  // The founder's destination phone (E.164). Inbound webhooks from
  // any other from-number are rejected as unauthorized. v0.1 is
  // founder-only.
  readonly founderPhoneNumber: string;
  // The founder's user_id (same string used in OAuth/cursor/alerts).
  // Every inbound reply is attributed to this user.
  readonly founderUserId: string;

  readonly killSwitches: KillSwitches;
  readonly inboundReplyStore: InboundReplyStore;
  readonly alertStore: AlertStore;
  readonly rankResultStore: RankResultStore;
  readonly transitions: AlertStateTransitionStore;
  readonly feedbackStore: FeedbackStore;
  readonly memoryStore: MemorySignalStore;
  readonly auditStore: AuditStore;

  // The reply-parser orchestrator. Injected so tests can stub.
  readonly replyParser: {
    parse: (req: ReplyParserRequest) => Promise<ReplyParseResult>;
  };

  // Optional clock for tests.
  readonly now?: () => Date;
}

/* ---------------------------------------------------------------------- */
/* HTTP response shape                                                    */
/* ---------------------------------------------------------------------- */

export interface HttpResponse {
  readonly status: number;
  readonly headers: Readonly<Record<string, string>>;
  readonly body: string;
}

function jsonResponse(status: number, payload: unknown): HttpResponse {
  return Object.freeze({
    status,
    headers: Object.freeze({ 'content-type': 'application/json' }),
    body: JSON.stringify(payload)
  });
}

/* ---------------------------------------------------------------------- */
/* Inbound payload extraction (tolerant)                                  */
/* ---------------------------------------------------------------------- */

// SendBlue's exact inbound webhook payload schema is not documented
// in our repo as of 3F.1. The extractor tolerates the most common
// field names; the 3F.2 founder smoke confirms the actual format and
// patches if needed (same pattern as the 3E.1 → 3E.2 fix cycle).
interface ExtractedInbound {
  readonly fromNumber: string;
  readonly content: string;
  readonly providerMessageId: string;
}

function asString(node: unknown, key: string): string | undefined {
  if (!node || typeof node !== 'object') return undefined;
  const v = (node as Record<string, unknown>)[key];
  return typeof v === 'string' ? v : undefined;
}

function extractInbound(payload: unknown): ExtractedInbound | null {
  if (!payload || typeof payload !== 'object') return null;
  const fromNumber =
    asString(payload, 'from_number') ??
    asString(payload, 'fromNumber') ??
    asString(payload, 'from');
  const content =
    asString(payload, 'content') ??
    asString(payload, 'text') ??
    asString(payload, 'body') ??
    asString(payload, 'message');
  const providerMessageId =
    asString(payload, 'message_handle') ??
    asString(payload, 'messageHandle') ??
    asString(payload, 'message_id') ??
    asString(payload, 'messageId') ??
    asString(payload, 'uuid') ??
    asString(payload, 'id');
  if (!fromNumber || !content || !providerMessageId) return null;
  return Object.freeze({ fromNumber, content, providerMessageId });
}

// Last-4 of a phone, for audit traceability. Never the full number.
function phoneSlug(phone: string | undefined): string {
  if (!phone) return '<unknown>';
  return phone.length <= 4 ? phone : phone.slice(-4);
}

/* ---------------------------------------------------------------------- */
/* Outcome → state/feedback/memory mapping                                */
/* ---------------------------------------------------------------------- */

// What the route ends up writing on top of the parser outcome.
// Returned for tests + audit detail.
interface RouteOutcome {
  readonly classified_intent: string;
  readonly source: 'deterministic' | 'classifier' | 'classifier_error';
  readonly alert_matched: boolean;
  readonly applied_state_transition: { from: AlertState; to: AlertState } | null;
  readonly feedback_kind: FeedbackEventKind | null;
  readonly memory_signal_kind: 'stop_active' | 'sender_suppressed' | null;
  readonly snooze_until: string | null;
}

/* ---------------------------------------------------------------------- */
/* Handler                                                                */
/* ---------------------------------------------------------------------- */

export interface HandleInboundInput {
  // Raw request body string (NOT JSON-parsed). Required for signature
  // verification — the HMAC is computed over the original bytes.
  readonly body: string;
  // Verbatim signature header value (e.g. `sha256=<hex>` or bare hex).
  readonly signature: string;
  // Optional timestamp header (if SendBlue sends one and the
  // operator configured freshness checking via SENDBLUE_WEBHOOK_MAX_AGE_SECONDS).
  readonly timestamp?: string;
}

export async function handleSendBlueInbound(
  input: HandleInboundInput,
  deps: SendBlueInboundRouteDeps
): Promise<HttpResponse> {
  const now = deps.now ?? ((): Date => new Date());

  // 1. Audit inbound_received BEFORE signature verification.
  //    Sanitized — only the body length, not the body.
  await deps.auditStore.write({
    actor_user_id: null,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.inbound_received',
    target: 'route:sendblue.inbound',
    result: 'success',
    detail: {
      body_bytes: input.body.length,
      has_timestamp: input.timestamp !== undefined
    }
  });

  // 2. Kill switch — defense-in-depth at request time. Bootstrap is
  //    expected to skip mounting this route when sendblue_inbound_enabled
  //    is false, but the handler still refuses to do anything when the
  //    switch is off. Returns 200 (we don't want to leak whether the
  //    route is wired) but does NOT process the request.
  if (!deps.killSwitches.sendblue_inbound_enabled) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.kill_switch_off',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: { error_code: 'kill_switch_off' }
    });
    return jsonResponse(200, {});
  }

  // 3. Signature verification.
  const verify = verifySendBlueSignature({
    signingSecret: deps.signingSecret,
    signature: input.signature,
    body: input.body,
    timestamp: input.timestamp
  });
  if (!verify.ok) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.signature_invalid',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: { error_code: verify.reason }
    });
    return jsonResponse(401, { error: 'signature_invalid' });
  }

  // 4. Parse JSON payload.
  let parsedPayload: unknown;
  try {
    parsedPayload = JSON.parse(input.body);
  } catch {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.payload_invalid',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: { error_code: 'body_not_json' }
    });
    return jsonResponse(400, { error: 'payload_invalid' });
  }

  const extracted = extractInbound(parsedPayload);
  if (!extracted) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.payload_invalid',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: { error_code: 'missing_required_fields' }
    });
    return jsonResponse(400, { error: 'payload_invalid' });
  }

  const fromSlug = phoneSlug(extracted.fromNumber);

  // 5. From-number allowlist — founder-only.
  if (extracted.fromNumber !== deps.founderPhoneNumber) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_unauthorized',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        from_slug: fromSlug,
        error_code: 'not_founder_phone',
        provider_message_id: extracted.providerMessageId
      }
    });
    return jsonResponse(403, { error: 'unauthorized', reason: 'wrong_from_number' });
  }

  // 6. Idempotency — LOAD-BEARING. SendBlue retries on non-2xx; the
  //    inbound_replies UNIQUE constraint catches duplicates and we
  //    early-return 200 so SendBlue stops retrying.
  const dedupOutcome = await deps.inboundReplyStore.record({
    provider_message_id: extracted.providerMessageId,
    user_id: deps.founderUserId
  });
  if (!dedupOutcome.inserted) {
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_duplicate',
      target: 'route:sendblue.inbound',
      result: 'success',
      detail: {
        from_slug: fromSlug,
        provider_message_id: extracted.providerMessageId,
        original_received_at: dedupOutcome.record.received_at
      }
    });
    return jsonResponse(200, { ok: true, duplicate: true });
  }

  // 7. Find the alert this reply most likely refers to. For v0.1
  //    (founder-only, single phone conversation) we use "the most
  //    recent alert in 'sent' state for this user". SendBlue's
  //    payload may also carry a thread/message-handle reference; if
  //    we surface a payload field that contains the original
  //    provider_message_handle we can refine this lookup in 3F.2.
  //    For substrate scope, the recent-sent heuristic is sufficient.
  const matchedAlert = await findMostRecentSentAlert(deps);

  // 8. Run the parser (deterministic safety pre-pass → classifier).
  //    The parser never persists the reply text; it only feeds the
  //    classifier prompt (egress-redacted).
  const parseResult: ReplyParseResult = await deps.replyParser.parse({
    user_reply_text: extracted.content,
    alert_context: {
      alert_subject: await resolveAlertSubject(deps, matchedAlert),
      alert_sender_name: undefined, // not available without re-reading Gmail
      alert_message_id: matchedAlert?.message_id ?? 'no-alert-match'
    },
    user_id: deps.founderUserId
  });

  // 9. Apply outcome. NEVER throws back to caller; every path audits
  //    its result and returns a 200/202-shaped response. Defensive:
  //    SendBlue keeps retrying anything non-2xx.
  let routeOutcome: RouteOutcome;
  try {
    routeOutcome = await applyParseResult(deps, parseResult, matchedAlert, fromSlug, extracted.providerMessageId, now);
  } catch (err) {
    // True internal error — surface 500 so SendBlue retries (idempotency
    // gate above will dedupe).
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.payload_invalid',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        from_slug: fromSlug,
        provider_message_id: extracted.providerMessageId,
        error_code: 'apply_failed',
        reason: err instanceof Error ? err.message : String(err)
      }
    });
    return jsonResponse(500, { error: 'internal' });
  }

  return jsonResponse(200, {
    ok: true,
    intent: routeOutcome.classified_intent,
    source: routeOutcome.source
  });
}

/* ---------------------------------------------------------------------- */
/* Apply parse result → state + feedback + memory                         */
/* ---------------------------------------------------------------------- */

async function applyParseResult(
  deps: SendBlueInboundRouteDeps,
  result: ReplyParseResult,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string,
  now: () => Date
): Promise<RouteOutcome> {
  // Deterministic → stop / start (compliance commands).
  if (result.ok && result.source === 'deterministic') {
    if (result.intent === 'stop') {
      return applyStop(deps, fromSlug, providerMessageId);
    }
    return applyStart(deps, fromSlug, providerMessageId);
  }

  // Classifier failure — audit only, no state writes.
  if (!result.ok) {
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_unclear',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        from_slug: fromSlug,
        provider_message_id: providerMessageId,
        error_code: 'classifier_error',
        classifier_code: result.code,
        reason: result.reason
      }
    });
    return Object.freeze({
      classified_intent: 'unclear',
      source: 'classifier_error' as const,
      alert_matched: matchedAlert !== null,
      applied_state_transition: null,
      feedback_kind: null,
      memory_signal_kind: null,
      snooze_until: null
    });
  }

  // Classifier success — soft intent dispatch.
  const c = result.classification;
  switch (c.intent) {
    case 'snooze':
      return applySnooze(deps, matchedAlert, c.snooze_hint, fromSlug, providerMessageId, now);
    case 'ignore':
      return applyIgnore(deps, matchedAlert, 'user_ignored', fromSlug, providerMessageId);
    case 'ignore_sender':
      return applyIgnoreSender(deps, matchedAlert, fromSlug, providerMessageId);
    case 'why':
      return applyWhy(deps, matchedAlert, fromSlug, providerMessageId);
    case 'false_positive':
      return applyIgnore(deps, matchedAlert, 'false_positive', fromSlug, providerMessageId);
    case 'unclear':
      return applyUnclear(deps, matchedAlert, fromSlug, providerMessageId, result.low_confidence_forced_unclear);
  }
}

async function applyStop(
  deps: SendBlueInboundRouteDeps,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  // 1. Memory signal: stop_active = true. Idempotent — second STOP
  //    just re-upserts the same row.
  await deps.memoryStore.upsert({
    user_id: deps.founderUserId,
    kind: 'stop_active',
    scope_key: null,
    detail: {
      active: true,
      recorded_at: new Date().toISOString()
    },
    source: 'user_confirmed',
    confidence: 1.0
  });
  // 2. Feedback event.
  await deps.feedbackStore.write({
    user_id: deps.founderUserId,
    alert_id: null,
    sender_email: null,
    kind: 'stop',
    detail: { source: 'sendblue_inbound', provider_message_id: providerMessageId }
  });
  // 3. Audit.
  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.stop_recorded',
    target: 'memory_signal:stop_active',
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      stop_active: true
    }
  });
  return Object.freeze({
    classified_intent: 'stop',
    source: 'deterministic' as const,
    alert_matched: false, // STOP is global, not per-alert
    applied_state_transition: null,
    feedback_kind: 'stop' as FeedbackEventKind,
    memory_signal_kind: 'stop_active' as const,
    snooze_until: null
  });
}

async function applyStart(
  deps: SendBlueInboundRouteDeps,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  await deps.memoryStore.upsert({
    user_id: deps.founderUserId,
    kind: 'stop_active',
    scope_key: null,
    detail: {
      active: false,
      recorded_at: new Date().toISOString()
    },
    source: 'user_confirmed',
    confidence: 1.0
  });
  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.start_recorded',
    target: 'memory_signal:stop_active',
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      stop_active: false
    }
  });
  return Object.freeze({
    classified_intent: 'start',
    source: 'deterministic' as const,
    alert_matched: false,
    applied_state_transition: null,
    feedback_kind: null,
    memory_signal_kind: 'stop_active' as const,
    snooze_until: null
  });
}

async function applySnooze(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null,
  hint: 'later' | 'tomorrow' | 'remind_me_later' | 'unspecified' | null,
  fromSlug: string,
  providerMessageId: string,
  now: () => Date
): Promise<RouteOutcome> {
  const seconds = computeSnoozeDurationSeconds(hint);
  const snoozeUntil = new Date(now().getTime() + seconds * 1000).toISOString();

  if (!matchedAlert) {
    // No alert to attach the snooze to. Audit only.
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_parsed',
      target: 'route:sendblue.inbound',
      result: 'success',
      detail: {
        from_slug: fromSlug,
        provider_message_id: providerMessageId,
        intent: 'snooze',
        intent_source: 'classifier',
        snooze_hint: hint,
        snooze_until: snoozeUntil,
        alert_matched: false
      }
    });
    return Object.freeze({
      classified_intent: 'snooze',
      source: 'classifier' as const,
      alert_matched: false,
      applied_state_transition: null,
      feedback_kind: null,
      memory_signal_kind: null,
      snooze_until: snoozeUntil
    });
  }

  // Walk: sent → replied → snoozed
  await walkTransition(deps, matchedAlert, 'sent', 'replied', `sendblue:snooze hint=${hint ?? 'null'}`);
  await walkTransition(deps, matchedAlert, 'replied', 'snoozed', `sendblue:snooze hint=${hint ?? 'null'} until=${snoozeUntil}`);

  await deps.feedbackStore.write({
    user_id: deps.founderUserId,
    alert_id: matchedAlert.alert_id,
    sender_email: null,
    kind: 'user_snoozed',
    detail: {
      source: 'sendblue_inbound',
      provider_message_id: providerMessageId,
      snooze_hint: hint,
      snooze_until: snoozeUntil
    }
  });

  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_parsed',
    target: `alert:${matchedAlert.alert_id}`,
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert.alert_id,
      intent: 'snooze',
      intent_source: 'classifier',
      snooze_hint: hint,
      snooze_until: snoozeUntil
    }
  });

  return Object.freeze({
    classified_intent: 'snooze',
    source: 'classifier' as const,
    alert_matched: true,
    applied_state_transition: { from: 'replied' as AlertState, to: 'snoozed' as AlertState },
    feedback_kind: 'user_snoozed' as FeedbackEventKind,
    memory_signal_kind: null,
    snooze_until: snoozeUntil
  });
}

async function applyIgnore(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null,
  feedbackKind: 'user_ignored' | 'false_positive',
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  if (!matchedAlert) {
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_parsed',
      target: 'route:sendblue.inbound',
      result: 'success',
      detail: {
        from_slug: fromSlug,
        provider_message_id: providerMessageId,
        intent: feedbackKind === 'false_positive' ? 'false_positive' : 'ignore',
        intent_source: 'classifier',
        alert_matched: false
      }
    });
    return Object.freeze({
      classified_intent: feedbackKind === 'false_positive' ? 'false_positive' : 'ignore',
      source: 'classifier' as const,
      alert_matched: false,
      applied_state_transition: null,
      feedback_kind: feedbackKind,
      memory_signal_kind: null,
      snooze_until: null
    });
  }

  await walkTransition(deps, matchedAlert, 'sent', 'replied', `sendblue:${feedbackKind}`);
  await walkTransition(deps, matchedAlert, 'replied', 'ignored', `sendblue:${feedbackKind}`);

  await deps.feedbackStore.write({
    user_id: deps.founderUserId,
    alert_id: matchedAlert.alert_id,
    sender_email: null,
    kind: feedbackKind,
    detail: { source: 'sendblue_inbound', provider_message_id: providerMessageId }
  });

  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_parsed',
    target: `alert:${matchedAlert.alert_id}`,
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert.alert_id,
      intent: feedbackKind === 'false_positive' ? 'false_positive' : 'ignore',
      intent_source: 'classifier'
    }
  });

  return Object.freeze({
    classified_intent: feedbackKind === 'false_positive' ? 'false_positive' : 'ignore',
    source: 'classifier' as const,
    alert_matched: true,
    applied_state_transition: { from: 'replied' as AlertState, to: 'ignored' as AlertState },
    feedback_kind: feedbackKind,
    memory_signal_kind: null,
    snooze_until: null
  });
}

async function applyIgnoreSender(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  // We need a sender to suppress. Without an alert match, we can't
  // know which sender; degrade to audit-only.
  if (!matchedAlert) {
    await deps.auditStore.write({
      actor_user_id: deps.founderUserId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_parsed',
      target: 'route:sendblue.inbound',
      result: 'success',
      detail: {
        from_slug: fromSlug,
        provider_message_id: providerMessageId,
        intent: 'ignore_sender',
        intent_source: 'classifier',
        alert_matched: false,
        sender_suppressed: false
      }
    });
    return Object.freeze({
      classified_intent: 'ignore_sender',
      source: 'classifier' as const,
      alert_matched: false,
      applied_state_transition: null,
      feedback_kind: 'ignored_sender' as FeedbackEventKind,
      memory_signal_kind: null,
      snooze_until: null
    });
  }

  // We don't have the sender_email on the alerts table for privacy
  // reasons (3D.1 design). Use message_id as the scope_key proxy
  // for v0.1 substrate — the future re-derivation worker can map
  // message_id → sender via Gmail re-read and tighten the signal.
  await deps.memoryStore.upsert({
    user_id: deps.founderUserId,
    kind: 'sender_suppressed',
    scope_key: `message:${matchedAlert.message_id}`,
    detail: {
      source: 'sendblue_inbound',
      provider_message_id: providerMessageId,
      alert_id: matchedAlert.alert_id,
      message_id: matchedAlert.message_id
    },
    source: 'user_confirmed',
    confidence: 1.0
  });

  await walkTransition(deps, matchedAlert, 'sent', 'replied', `sendblue:ignore_sender`);
  await walkTransition(deps, matchedAlert, 'replied', 'ignored', `sendblue:ignore_sender`);

  await deps.feedbackStore.write({
    user_id: deps.founderUserId,
    alert_id: matchedAlert.alert_id,
    sender_email: null,
    kind: 'ignored_sender',
    detail: { source: 'sendblue_inbound', provider_message_id: providerMessageId }
  });

  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_parsed',
    target: `alert:${matchedAlert.alert_id}`,
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert.alert_id,
      intent: 'ignore_sender',
      intent_source: 'classifier',
      sender_suppressed: true
    }
  });

  return Object.freeze({
    classified_intent: 'ignore_sender',
    source: 'classifier' as const,
    alert_matched: true,
    applied_state_transition: { from: 'replied' as AlertState, to: 'ignored' as AlertState },
    feedback_kind: 'ignored_sender' as FeedbackEventKind,
    memory_signal_kind: 'sender_suppressed' as const,
    snooze_until: null
  });
}

async function applyWhy(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  // 'why' transitions sent → replied (acknowledgement) but does NOT
  // proceed to a terminal state — the user wants info, not closure.
  // A future phase can respond with an explanation iMessage.
  if (matchedAlert) {
    await walkTransition(deps, matchedAlert, 'sent', 'replied', 'sendblue:why');
    await deps.feedbackStore.write({
      user_id: deps.founderUserId,
      alert_id: matchedAlert.alert_id,
      sender_email: null,
      kind: 'asked_why',
      detail: { source: 'sendblue_inbound', provider_message_id: providerMessageId }
    });
  }
  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_parsed',
    target: matchedAlert ? `alert:${matchedAlert.alert_id}` : 'route:sendblue.inbound',
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert?.alert_id,
      intent: 'why',
      intent_source: 'classifier',
      alert_matched: matchedAlert !== null
    }
  });
  return Object.freeze({
    classified_intent: 'why',
    source: 'classifier' as const,
    alert_matched: matchedAlert !== null,
    applied_state_transition: matchedAlert
      ? { from: 'sent' as AlertState, to: 'replied' as AlertState }
      : null,
    feedback_kind: matchedAlert ? ('asked_why' as FeedbackEventKind) : null,
    memory_signal_kind: null,
    snooze_until: null
  });
}

async function applyUnclear(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string,
  lowConfidenceForced: boolean
): Promise<RouteOutcome> {
  await deps.auditStore.write({
    actor_user_id: deps.founderUserId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_unclear',
    target: matchedAlert ? `alert:${matchedAlert.alert_id}` : 'route:sendblue.inbound',
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert?.alert_id,
      alert_matched: matchedAlert !== null,
      low_confidence_forced: lowConfidenceForced
    }
  });
  return Object.freeze({
    classified_intent: 'unclear',
    source: 'classifier' as const,
    alert_matched: matchedAlert !== null,
    applied_state_transition: null,
    feedback_kind: null,
    memory_signal_kind: null,
    snooze_until: null
  });
}

/* ---------------------------------------------------------------------- */
/* Alert resolution + helpers                                             */
/* ---------------------------------------------------------------------- */

// For v0.1 founder-only: the inbound reply is most likely the
// founder's response to the MOST RECENT alert in 'sent' state. We
// iterate recent alerts and pick the first whose current state is
// 'sent'. The future scheduler can refine by passing a thread-id
// from SendBlue's payload (if available).
async function findMostRecentSentAlert(
  deps: SendBlueInboundRouteDeps
): Promise<Alert | null> {
  const recent = await deps.alertStore.recent(deps.founderUserId, 20);
  for (const a of recent) {
    const state = (await deps.transitions.currentState(a.alert_id)) ?? 'detected';
    if (state === 'sent') return a;
  }
  return null;
}

async function resolveAlertSubject(
  deps: SendBlueInboundRouteDeps,
  matchedAlert: Alert | null
): Promise<string> {
  if (!matchedAlert) return '(no alert context available)';
  const rank = await deps.rankResultStore.get(deps.founderUserId, matchedAlert.message_id);
  if (!rank) return '(rank context missing)';
  // The rank_result.reason is the closest thing to a subject we
  // have at hand without re-reading Gmail. It's already model-
  // authored, ≤240 chars, and egress-safe (no body content).
  return rank.reason;
}

async function walkTransition(
  deps: SendBlueInboundRouteDeps,
  alert: Alert,
  fromState: AlertState,
  toState: AlertState,
  reason: string
): Promise<void> {
  const currentState = (await deps.transitions.currentState(alert.alert_id)) ?? 'detected';
  if (currentState !== fromState) {
    // State drift — log + skip rather than throw. The route's outer
    // error handler treats throws as 500; we'd rather acknowledge
    // the inbound and audit the drift.
    await deps.auditStore.write({
      actor_user_id: alert.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'state.transitioned',
      target: `alert:${alert.alert_id}`,
      result: 'failure',
      detail: {
        alert_id: alert.alert_id,
        from_state: fromState,
        to_state: toState,
        error_code: 'state_drift',
        current_state: currentState,
        reason
      }
    });
    return;
  }
  const validated = transition(fromState, toState, reason);
  if ('error' in validated) {
    await deps.auditStore.write({
      actor_user_id: alert.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'state.transitioned',
      target: `alert:${alert.alert_id}`,
      result: 'failure',
      detail: {
        alert_id: alert.alert_id,
        from_state: fromState,
        to_state: toState,
        error_code: 'invalid_transition',
        reason: validated.reason
      }
    });
    return;
  }
  await deps.transitions.write({
    alert_id: alert.alert_id,
    user_id: alert.user_id,
    from_state: fromState,
    to_state: toState,
    reason
  });
  await deps.auditStore.write({
    actor_user_id: alert.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'state.transitioned',
    target: `alert:${alert.alert_id}`,
    result: 'success',
    detail: {
      alert_id: alert.alert_id,
      from_state: fromState,
      to_state: toState
    }
  });
}

/* ---------------------------------------------------------------------- */
/* HTTP adapter                                                           */
/* ---------------------------------------------------------------------- */

async function readBody(req: http.IncomingMessage, maxBytes = 64_000): Promise<Buffer> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    let total = 0;
    req.on('data', (chunk: Buffer) => {
      total += chunk.length;
      if (total > maxBytes) {
        reject(new Error(`body too large (${total} > ${maxBytes})`));
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', reject);
  });
}

function getHeader(req: http.IncomingMessage, name: string): string {
  const v = req.headers[name.toLowerCase()];
  if (typeof v === 'string') return v;
  if (Array.isArray(v) && v.length > 0) return v[0] ?? '';
  return '';
}

// Returns the response to send, or null when the request did not
// match the sendblue-inbound route. The server in index.ts checks
// for null and falls through.
export async function tryHandleSendBlueInboundRequest(
  req: http.IncomingMessage,
  deps: SendBlueInboundRouteDeps
): Promise<HttpResponse | null> {
  const method = req.method ?? 'GET';
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (!(method === 'POST' && url.pathname === '/sendblue/inbound')) {
    return null;
  }
  let bodyBuf: Buffer;
  try {
    bodyBuf = await readBody(req);
  } catch (err) {
    return jsonResponse(413, {
      error: 'payload_too_large',
      message: err instanceof Error ? err.message : String(err)
    });
  }
  // SendBlue's exact header names are TBD; we accept several common
  // shapes. Operators configure SendBlue to send one of these.
  const signature =
    getHeader(req, 'x-sendblue-signature') ||
    getHeader(req, 'sb-signature') ||
    getHeader(req, 'x-signature');
  const timestamp =
    getHeader(req, 'x-sendblue-request-timestamp') ||
    getHeader(req, 'sb-request-timestamp') ||
    getHeader(req, 'x-request-timestamp') ||
    undefined;
  return handleSendBlueInbound(
    {
      body: bodyBuf.toString('utf8'),
      signature,
      timestamp: timestamp || undefined
    },
    deps
  );
}
