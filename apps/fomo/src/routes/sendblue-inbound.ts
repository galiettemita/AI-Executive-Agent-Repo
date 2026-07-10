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

import { verifySendBlueWebhookSecret } from '../adapters/sendblue/client.js';
import { type AuditStore } from '../core/audit.js';
import { type AlertStateTransitionStore } from '../core/alert-state-transitions.js';
import { sanitizeProviderError } from '../core/sanitize-provider-error.js';
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
  createVisibleMemoryCommandAppAdapterAuditStoreRecorder,
  handleVisibleMemoryCommandAppAdapterRequest,
  type VisibleExplicitPreferenceRecallQuery,
  type VisiblePreferenceCorrectOptions,
  type VisiblePreferenceRememberOptions
} from '../memory/typed-memory-visible-recall.js';
import {
  type ReplyParseResult,
  type ReplyParserRequest,
  computeSnoozeDurationSeconds
} from '../reply-parser/index.js';
// Phase v0.5.10 — single policy entry point for intent → feedback_event +
// applyFeedback routing.
import { routeReplyFeedback } from '../reply-parser/feedback-routing.js';
import { type ReplyIntent } from '../reply-parser/validator.js';
// Phase v0.5.11 — PIL substrate aggregation pipe (canonical HMAC-keyed
// sender_importance / sender_suppressed upserts on the 4 PIL-relevant
// intents).
import { applyPilAggregation } from '../memory/pil-aggregation.js';
// Phase v0.7.0A — "Why?" explainability surface deterministic renderer.
import {
  EMPTY_STATE_COPY,
  EXPLAIN_LOOKUP_WINDOW_MS,
  EXPLAIN_SOURCE_SURFACE,
  EXPLAIN_TEMPLATE_VERSION,
  composeExplanationFromReason,
  hashAlertIdForAudit
} from '../core/explain-renderer.js';

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface SendBlueInboundRouteDeps {
  // The webhook secret you configured in the SendBlue dashboard.
  // SendBlue's auth scheme (per docs.sendblue.com/getting-started/
  // webhooks): "When you configure a secret, Sendblue will include
  // it in the webhook request headers, allowing you to verify that
  // the request is genuinely from Sendblue." NOT HMAC, NOT a body
  // signature — plain header-equality with a timing-safe compare.
  // Different from the API key/secret used for outbound sends —
  // this is per-webhook, rotated independently in the dashboard.
  readonly webhookSecret: string;
  // The HTTP header name SendBlue uses to send the configured
  // secret. SendBlue's public docs don't name it explicitly; the
  // default `sb-signing-secret` matches their `sb-*` API-header
  // naming pattern. The bootstrap honors a
  // SENDBLUE_WEBHOOK_SECRET_HEADER env var so the founder can
  // patch this during 3F.2 smoke (when we observe a real inbound
  // request) without a code change.
  readonly webhookSecretHeader: string;
  // The founder's destination phone (E.164). v0.1 single-user
  // shortcut; matched first before the phoneAllowlist below. Inbound
  // webhooks from any other from-number that ALSO don't match a
  // user via the phoneAllowlist are rejected as unauthorized.
  readonly founderPhoneNumber: string;
  // The founder's user_id (same string used in OAuth/cursor/alerts).
  // Used when the from-number matches founderPhoneNumber.
  readonly founderUserId: string;

  // Phase v0.5.1 Step 6 — multi-tenant routing. When the from-number
  // doesn't match founderPhoneNumber, the route hashes it via
  // phoneHash and looks up the user_id via phoneAllowlist. This
  // enables friends (provisioned through /onboard) to text STOP and
  // have it persist for THEIR user_id only — no cross-user
  // contamination. When phoneAllowlist + phoneHash are absent the
  // route behaves exactly as v0.1 (founder-only).
  //
  // Per-user routing also enables outbound-sender per-user stop_active
  // enforcement to actually mean what it says.
  readonly phoneAllowlist?: import('../security/phone-allowlist.js').PhoneAllowlistStore;
  readonly phoneHash?: import('../security/phone-allowlist.js').PhoneHashConfig;

  readonly killSwitches: KillSwitches;
  readonly inboundReplyStore: InboundReplyStore;
  readonly alertStore: AlertStore;
  readonly rankResultStore: RankResultStore;
  readonly transitions: AlertStateTransitionStore;
  readonly feedbackStore: FeedbackStore;
  readonly memoryStore: MemorySignalStore;
  readonly auditStore: AuditStore;

  // Phase v0.5.10 — HMAC key for the v0.5.9 applyFeedback consumer arm
  // (sender_feedback_ignored upsert when intent === 'ignore_sender').
  // Loaded from BREVIO_SENDER_HASH_KEY at boot via loadSenderHashKey;
  // never logged.
  readonly senderHashKey: Buffer;

  // Phase v0.5.11 — tunable PIL thresholds (Q5.C). Loaded once at boot via
  // loadPilTunables(env). Optional — when absent, applyPilAggregation is
  // not called (v0.5.10 behavior). Present → v0.5.11 aggregation pipe fires
  // for the 4 PIL-relevant intents (this_mattered, more_like_this,
  // false_positive, ignore_sender) when matchedAlert.sender_email_hash is
  // available.
  readonly pilTunables?: import('../memory/pil-aggregation.js').PilTunables;

  // The reply-parser orchestrator. Injected so tests can stub.
  readonly replyParser: {
    parse: (req: ReplyParserRequest) => Promise<ReplyParseResult>;
  };

  // Phase v0.5.5 — STOP confirmation send dep. Optional: when absent,
  // applyStop records the stop_active signal but does NOT attempt a
  // courtesy confirmation iMessage (silently continues exactly as
  // v0.5.4). When present, applyStop calls .send() exactly once per
  // user per 24h idempotency window with the deterministic canonical
  // body; success/failure is audited via fomo.sendblue.stop_confirmation_{sent,failed}.
  //
  // This is intentionally a narrowly-typed dep (just `.send`) so it
  // can be mocked in tests AND so it is clear at every call site that
  // this is the ONLY allowed outbound exception after STOP. The
  // normal FOMO alert pipeline (outbound-sender.ts) continues to be
  // blocked by `fomo.send.stop_enforced` — the courtesy confirmation
  // is the only message a STOP'd user receives, once per 24h.
  //
  // Q6 (founder-locked): best-effort audit, no retry on failure. The
  // confirmation is a courtesy/trust message, not load-bearing. STOP
  // enforcement (the suppression of future alerts) is load-bearing
  // and works whether or not the confirmation arrived.
  readonly stopConfirmation?: {
    send: (input: import('../adapters/sendblue/client.js').SendInput) => Promise<import('../adapters/sendblue/client.js').SendOutcome>;
  };

  // Phase v0.5.14 — HMR Feedback Acknowledgment Surface. Optional
  // SendBlue-backed ack send for the 4 ackable feedback intents
  // (ignore_sender / this_mattered / more_like_this / false_positive).
  // Same shape as stopConfirmation. When absent, routeReplyFeedback
  // takes the v0.5.10 baseline path (no ack, no fomo.sendblue.feedback_ack_*
  // audit). When present, the route handler threads the inbound E.164
  // (fromNumber) into routeReplyFeedback's input.from_number; the
  // routing module decides whether to render+send+audit based on the
  // 3-gate check (dep present + from_number present + ackable intent).
  readonly feedbackAck?: {
    send: (input: import('../adapters/sendblue/client.js').SendInput) => Promise<import('../adapters/sendblue/client.js').SendOutcome>;
  };

  // Phase v0.7.0A — "Why?" explainability surface send dep. Optional:
  // when absent OR when killSwitches.explain_surface_enabled is false,
  // applyWhy walks the v0.5.10 baseline behavior (state transition +
  // audit only) and does NOT send any outbound explanation iMessage.
  // When BOTH the dep is wired AND the kill switch is on AND the
  // inbound's fromNumber is present, applyWhy composes a deterministic
  // explanation from the most recent eligible alert's rank.reason and
  // sends it back, then audits brevio.explain.served (success or
  // failure). Best-effort: send failures NEVER abort applyWhy's
  // existing state-machine + feedback-routing path.
  //
  // Same narrow `.send` shape as stopConfirmation/feedbackAck so it
  // composes cleanly with the existing SendBlue client and tests can
  // mock easily.
  readonly explainSender?: {
    send: (input: import('../adapters/sendblue/client.js').SendInput) => Promise<import('../adapters/sendblue/client.js').SendOutcome>;
  };

  // Memory V1 PR-17 — disabled-by-default visible memory command seam.
  // This is intentionally NOT a parser. The route may call the app adapter
  // only when a caller explicitly enables the seam, supplies a typed-memory
  // store, and supplies deterministic/caller-provided command context. The
  // inbound SendBlue freeform text is never parsed into memory here.
  readonly visibleMemoryCommand?: {
    readonly enabled?: boolean;
    readonly store?: Parameters<typeof handleVisibleMemoryCommandAppAdapterRequest>[0];
    readonly context?: SendBlueInboundVisibleMemoryCommandContext;
  };

  // Optional clock for tests.
  readonly now?: () => Date;
}

export interface SendBlueInboundVisibleMemoryCommandContext {
  readonly text: string;
  readonly parsedPreference?: VisiblePreferenceRememberOptions;
  readonly query?: VisibleExplicitPreferenceRecallQuery;
  readonly correction?: VisiblePreferenceCorrectOptions;
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
  // Raw request body string (NOT JSON-parsed). Auth check does NOT
  // depend on the body (SendBlue doesn't HMAC-sign the body —
  // confirmed via docs 2026-05-26), but the route still wants the
  // raw bytes for two reasons: (a) JSON.parse the body for payload
  // extraction below, (b) avoid trusting any pre-parsed body the
  // HTTP framework might offer (defense-in-depth against framework-
  // level parsing surprises).
  readonly body: string;
  // Verbatim value of the secret-bearing header on the inbound POST.
  // The route layer reads `deps.webhookSecretHeader` from the
  // request and passes whatever raw string is there. Empty string
  // when the header was absent.
  readonly secretHeaderValue: string;
}

export async function handleSendBlueInbound(
  input: HandleInboundInput,
  deps: SendBlueInboundRouteDeps
): Promise<HttpResponse> {
  const now = deps.now ?? ((): Date => new Date());

  // 1. Audit inbound_received BEFORE auth verification.
  //    Sanitized — only the body length + whether the secret header
  //    was present, NOT the body, NOT the header value.
  await deps.auditStore.write({
    actor_user_id: null,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.inbound_received',
    target: 'route:sendblue.inbound',
    result: 'success',
    detail: {
      body_bytes: input.body.length,
      secret_header_present: input.secretHeaderValue.length > 0,
      secret_header_name: deps.webhookSecretHeader
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

  // 3. Webhook secret verification (header equality, timing-safe).
  //    SendBlue uses a plain shared secret in a header, NOT HMAC.
  //    See client.ts verifySendBlueWebhookSecret docs for evidence.
  //    Fail-closed: any non-ok result → 401 + audit + early return.
  //    The route NEVER parses the body, NEVER transitions state,
  //    NEVER updates memory_signals when auth fails. Per founder
  //    directive 2026-05-26.
  const verify = verifySendBlueWebhookSecret({
    configuredSecret: deps.webhookSecret,
    headerValue: input.secretHeaderValue
  });
  if (!verify.ok) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      // Audit-action name is `signature_invalid` for historical
      // consistency with the rest of the v0.1 audit vocabulary
      // (Slack's signature audit). The `error_code` detail field
      // carries the SendBlue-specific reason so ops can distinguish
      // missing_header from secret_mismatch without renaming the
      // audit action.
      action: 'fomo.sendblue.signature_invalid',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        error_code: verify.reason,
        secret_header_name: deps.webhookSecretHeader
      }
    });
    return jsonResponse(401, { error: 'webhook_unauthorized', reason: verify.reason });
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

  // 5. From-number routing — per-user (Phase v0.5.1 Step 6).
  //
  // Resolution order:
  //   (a) If from_number matches FOMO_FOUNDER_PHONE_NUMBER (env), the
  //       founder is the user. v0.1 backward-compat path.
  //   (b) Otherwise, if phoneAllowlist + phoneHash are wired, hash
  //       the from-number and look up the user_id.
  //   (c) Otherwise (or unknown hash) — 403 unauthorized.
  //
  // The resolved `userId` flows through every subsequent operation;
  // memory_signals.stop_active, feedback_events, audit actor_user_id,
  // and alert_state_transitions all scope to THIS userId, NOT the
  // founder's. That's how friend STOP doesn't bleed into the founder's
  // memory + vice versa.
  let userId: string | null = null;
  if (extracted.fromNumber === deps.founderPhoneNumber) {
    userId = deps.founderUserId;
  } else if (deps.phoneAllowlist && deps.phoneHash) {
    const { hashPhone } = await import('../security/phone-allowlist.js');
    const phoneHashValue = hashPhone(extracted.fromNumber, deps.phoneHash);
    userId = await deps.phoneAllowlist.findUserIdByPhoneHash(phoneHashValue);
  }
  if (!userId) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.sendblue.reply_unauthorized',
      target: 'route:sendblue.inbound',
      result: 'failure',
      detail: {
        from_slug: fromSlug,
        error_code: 'unknown_from_number',
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
    user_id: userId
  });
  if (!dedupOutcome.inserted) {
    await deps.auditStore.write({
      actor_user_id: userId,
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
  const matchedAlert = await findMostRecentSentAlert(deps, userId);

  // 8. Run the parser (deterministic safety pre-pass → classifier).
  //    The parser never persists the reply text; it only feeds the
  //    classifier prompt (egress-redacted).
  const parseResult: ReplyParseResult = await deps.replyParser.parse({
    user_reply_text: extracted.content,
    alert_context: {
      alert_subject: await resolveAlertSubject(deps, userId, matchedAlert),
      alert_sender_name: undefined, // not available without re-reading Gmail
      alert_message_id: matchedAlert?.message_id ?? 'no-alert-match'
    },
    user_id: userId
  });

  // 9. Apply outcome. NEVER throws back to caller; every path audits
  //    its result and returns a 200/202-shaped response. Defensive:
  //    SendBlue keeps retrying anything non-2xx.
  let routeOutcome: RouteOutcome;
  try {
    routeOutcome = await applyParseResult(
      deps,
      userId,
      parseResult,
      matchedAlert,
      fromSlug,
      extracted.providerMessageId,
      now,
      extracted.fromNumber,
      // Phase v0.5.10 — forward-link for the feedback.written audit.
      dedupOutcome.record.id ?? 0
    );
  } catch (err) {
    // True internal error — surface 500 so SendBlue retries (idempotency
    // gate above will dedupe).
    await deps.auditStore.write({
      actor_user_id: userId,
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

  await maybeHandleVisibleMemoryCommand(
    deps,
    userId,
    parseResult,
    now
  );

  return jsonResponse(200, {
    ok: true,
    intent: routeOutcome.classified_intent,
    source: routeOutcome.source
  });
}

async function maybeHandleVisibleMemoryCommand(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  parseResult: ReplyParseResult,
  now: () => Date
): Promise<void> {
  const seam = deps.visibleMemoryCommand;
  if (!seam?.enabled || !seam.store || !seam.context) return;

  // Compliance commands stay load-bearing and separate. Even if a caller
  // supplied memory-command context, STOP/START must not invoke memory command
  // handling or bypass the existing deterministic enforcement path.
  if (
    parseResult.ok &&
    parseResult.source === 'deterministic' &&
    (parseResult.intent === 'stop' || parseResult.intent === 'start')
  ) {
    return;
  }

  await handleVisibleMemoryCommandAppAdapterRequest(seam.store, {
    enabled: true,
    userId,
    text: seam.context.text,
    parsedPreference: seam.context.parsedPreference,
    query: seam.context.query,
    correction: seam.context.correction,
    audit: {
      enabled: true,
      record: createVisibleMemoryCommandAppAdapterAuditStoreRecorder({
        enabled: true,
        auditStore: deps.auditStore,
        occurredAt: now().toISOString()
      })
    }
  });

}

/* ---------------------------------------------------------------------- */
/* Apply parse result → state + feedback + memory                         */
/* ---------------------------------------------------------------------- */

async function applyParseResult(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  result: ReplyParseResult,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string,
  now: () => Date,
  // Phase v0.5.5 — full E.164 of the inbound sender, threaded through
  // so applyStop can send the courtesy confirmation back to them. Only
  // used by the STOP path; other intent handlers ignore it.
  fromNumber: string,
  // Phase v0.5.10 — inbound_replies.id forward-link for the
  // feedback.written audit's correlation field.
  inboundReplyId: number
): Promise<RouteOutcome> {
  // Deterministic compliance commands — stop / start. STOP/START stay
  // a separate compliance/control path per founder lock; they NEVER
  // become preference feedback and do NOT go through the v0.5.10
  // routing module.
  if (result.ok && result.source === 'deterministic' && (result.intent === 'stop' || result.intent === 'start')) {
    if (result.intent === 'stop') {
      return applyStop(deps, userId, fromSlug, providerMessageId, fromNumber, now);
    }
    return applyStart(deps, userId, fromSlug, providerMessageId);
  }

  // Classifier failure — audit only, no state writes.
  if (!result.ok) {
    await deps.auditStore.write({
      actor_user_id: userId,
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

  // ─── Phase v0.5.10 soft-intent dispatch ──────────────────────────
  // Resolve the (intent, intent_source, parser_confidence, snooze_hint)
  // tuple from either the deterministic-allowlist path (intent !=
  // stop/start) or the classifier path. Call routeReplyFeedback FIRST
  // to write the v0.5.9-shaped feedback_event + emit the v0.5.10-extended
  // feedback.written audit + (for ignore_sender only) fire the v0.5.9
  // consumer (sender_feedback_ignored upsert + brevio.feedback.applied
  // audit). THEN dispatch to the existing applyXXX functions for the
  // state-machine transitions + reply_parsed audit emission. The
  // applyXXX functions no longer write feedback_events (the routing
  // module owns those writes).

  type SoftDispatchIntent = Exclude<ReplyIntent, 'unclear'>;
  let dispatchIntent: ReplyIntent;
  let intentSource: 'reply_parser_classifier' | 'reply_parser_deterministic';
  let parserConfidence: number;
  let snoozeHint: 'later' | 'tomorrow' | 'remind_me_later' | 'unspecified' | null = null;
  let lowConfidenceForcedUnclear = false;

  if (result.source === 'deterministic') {
    // Allowlist match. The deterministic pre-pass returned a non-
    // compliance intent (stop/start handled above), so this is a soft
    // intent from the Q3.C allowlist.
    dispatchIntent = result.intent as ReplyIntent; // guaranteed non-compliance here
    intentSource = 'reply_parser_deterministic';
    parserConfidence = 1.0;
  } else {
    const c = result.classification;
    dispatchIntent = c.intent;
    intentSource = 'reply_parser_classifier';
    parserConfidence = c.confidence;
    snoozeHint = c.snooze_hint;
    lowConfidenceForcedUnclear = result.low_confidence_forced_unclear;
  }

  // unclear → routing module is NOT called (returns unclear_no_op
  // semantically). Just dispatch to applyUnclear for the legacy reply_unclear
  // audit + state transition.
  if (dispatchIntent === 'unclear') {
    return applyUnclear(deps, userId, matchedAlert, fromSlug, providerMessageId, lowConfidenceForcedUnclear);
  }

  // Phase v0.5.10 — route the feedback signal through the policy module.
  // This writes the v0.5.9-shaped feedback_event + extended feedback.written
  // audit + (for ignore_sender) fires applyFeedback.
  const routeOutcome = await routeReplyFeedback(
    {
      user_id: userId,
      intent: dispatchIntent as SoftDispatchIntent,
      intent_source: intentSource,
      parser_confidence: parserConfidence,
      inbound_reply_id: inboundReplyId,
      alert_id: matchedAlert?.alert_id ?? null,
      // The v0.1 substrate alerts table did NOT carry sender_email
      // (3D.1 privacy design). The v0.5.9 applyFeedback consumer arm for
      // ignore_sender requires a raw sender_email to hash into the
      // scope_key; when null, applyFeedback returns no_match gracefully.
      // v0.5.11 closes this gap WITHOUT changing the v0.5.9 contract: the
      // alert now carries a privacy-safe sender_email_hash (HMAC), and
      // the new applyPilAggregation call below uses that hash DIRECTLY as
      // the scope_key — bypassing the routeReplyFeedback re-hash path.
      sender_email: null,
      snooze_hint: dispatchIntent === 'snooze' ? snoozeHint : null,
      // Phase v0.5.14 — thread the inbound E.164 so routeReplyFeedback
      // can fire the deterministic ack send for ackable intents. The
      // routing module owns the 3-gate decision (dep + number + ackable);
      // we just pass the number through. NEVER logged or persisted by
      // routeReplyFeedback — used only as the `to` argument to deps.feedbackAck.send.
      from_number: fromNumber
    },
    {
      feedbackStore: deps.feedbackStore,
      auditStore: deps.auditStore,
      memoryStore: deps.memoryStore,
      senderHashKey: deps.senderHashKey,
      now: deps.now ? () => deps.now!().getTime() : undefined,
      // Phase v0.5.14 — optional ack send dep. When undefined (founder
      // hasn't wired in bootstrap), routeReplyFeedback takes the v0.5.10
      // baseline path (no ack, no audit). When present, ackable intents
      // produce a best-effort deterministic ack SMS + a separate audit.
      feedbackAck: deps.feedbackAck
    }
  );

  // Phase v0.5.11 — PIL aggregation. After the v0.5.10 routing module
  // writes the feedback_event, project the four PIL-relevant intents into
  // canonical HMAC-keyed sender_importance / sender_suppressed signals.
  // Hard boundaries:
  //   - Only fires when (a) tunables are wired (i.e. boot supplied them),
  //     (b) routeOutcome.kind === 'wrote' (no unclear no-op), (c) the
  //     matched alert carries a sender_email_hash (the v0.5.11 rank-step
  //     thread populated it), and (d) the intent maps to a PIL arm.
  //   - When sender_email_hash is null (pre-migration alert, fixture, or no
  //     matched alert), the call returns kind='skipped' and writes nothing.
  //   - v0.5.10 sender_feedback_ignored placeholder behavior is UNTOUCHED:
  //     applyIgnoreSender below continues to write its message:<id> row.
  if (
    deps.pilTunables &&
    routeOutcome.kind === 'wrote' &&
    matchedAlert?.sender_email_hash
  ) {
    const pilArm = pilArmFromIntent(dispatchIntent);
    const feedbackEventId = routeOutcome.feedback_event.id;
    if (pilArm !== null && typeof feedbackEventId === 'number') {
      await applyPilAggregation(
        {
          user_id: userId,
          feedback_event_id: feedbackEventId,
          verb: pilArm.verb,
          dimension: pilArm.dimension,
          source_surface: 'email_alert',
          sender_email_hash: matchedAlert.sender_email_hash,
          k_threshold: deps.pilTunables.k_threshold,
          score_delta: deps.pilTunables.score_delta,
          now: deps.now ? () => deps.now!() : undefined
        },
        {
          memoryStore: deps.memoryStore,
          auditStore: deps.auditStore
        }
      );
    }
  }

  // Now dispatch to the existing applyXXX functions for state-machine
  // transitions + the legacy reply_parsed audit. These functions no
  // longer write feedback_events (the routing module above owns those).
  switch (dispatchIntent) {
    case 'snooze':
      return applySnooze(deps, userId, matchedAlert, snoozeHint, fromSlug, providerMessageId, now);
    case 'ignore':
      return applyIgnore(deps, userId, matchedAlert, 'user_ignored', fromSlug, providerMessageId);
    case 'ignore_sender':
      return applyIgnoreSender(deps, userId, matchedAlert, fromSlug, providerMessageId);
    case 'why':
      return applyWhy(deps, userId, matchedAlert, fromSlug, providerMessageId, fromNumber, now);
    case 'false_positive':
      return applyIgnore(deps, userId, matchedAlert, 'false_positive', fromSlug, providerMessageId);
    // Phase v0.5.10 — positive-signal intents. They flow through the
    // routing module above (feedback_event write + audit emission),
    // then through applyPositiveAcknowledge (state-machine transition
    // to 'replied' + reply_parsed audit; no other side effect per Q5.A
    // silent lock). The existing applyXXX functions are alert-action-
    // shaped (ignore/snooze on an alert state machine) and don't fit
    // the positive-signal semantics, so a dedicated handler.
    case 'this_mattered':
    case 'more_like_this':
      return applyPositiveAcknowledge(deps, userId, matchedAlert, dispatchIntent, fromSlug, providerMessageId);
  }
}

// Phase v0.5.11 — map v0.5.10 reply-parser intents onto the PIL aggregation
// arm tuple. Returns null for intents that don't drive PIL writes
// (snooze / ignore / why / unclear).
function pilArmFromIntent(
  intent: string
): { verb: 'approved' | 'corrected' | 'ignored'; dimension: 'importance' | 'pattern' | 'ranker_label' | 'sender' } | null {
  switch (intent) {
    case 'this_mattered':
      return { verb: 'approved', dimension: 'importance' };
    case 'more_like_this':
      return { verb: 'approved', dimension: 'pattern' };
    case 'false_positive':
      return { verb: 'corrected', dimension: 'ranker_label' };
    case 'ignore_sender':
      return { verb: 'ignored', dimension: 'sender' };
    default:
      return null;
  }
}

// Phase v0.5.5 — canonical deterministic STOP confirmation body.
// Exported so smoke-evidence + tests can assert exact wording without
// duplicating the string. Kept terse + friendly per the v0.5.4 Sheila
// feedback (the full tone rewrite is the v0.5.5+ B1 phase candidate);
// must contain the two canonical phrases the smoke-evidence C8 check
// looks for: "unsubscrib"/"STOP" + "START".
export const STOP_CONFIRMATION_BODY = "You're unsubscribed from Brevio. Text START to turn it back on.";

// Phase v0.5.5 — 24h idempotency window for STOP confirmations
// (founder-locked Q5). Duplicate STOP within 24h → no new confirmation.
// STOP after 24h → fresh confirmation may be sent.
export const STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS = 24 * 60 * 60 * 1000;

async function applyStop(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  fromSlug: string,
  providerMessageId: string,
  // Phase v0.5.5 — full E.164 of the inbound sender, for the courtesy
  // confirmation send back to them. Never logged or persisted; only
  // passed through to SendBlue's send API.
  fromNumber: string,
  now: () => Date
): Promise<RouteOutcome> {
  const nowDate = now();
  const nowIso = nowDate.toISOString();
  const nowMs = nowDate.getTime();

  // Phase v0.5.5 — read the existing stop_active signal BEFORE upserting
  // so we can carry forward `stop_confirmation_sent_at` for the 24h
  // idempotency check. Memory_signals.upsert REPLACES detail, so we have
  // to merge ourselves rather than relying on Postgres-side JSONB merge.
  const existing = await deps.memoryStore.get(userId, 'stop_active', null);
  const existingDetail = (existing?.detail ?? {}) as {
    active?: unknown;
    recorded_at?: unknown;
    stop_confirmation_sent_at?: unknown;
  };
  const priorConfirmationSentAtIso =
    typeof existingDetail.stop_confirmation_sent_at === 'string'
      ? existingDetail.stop_confirmation_sent_at
      : null;
  const priorConfirmationSentAtMs = priorConfirmationSentAtIso
    ? Date.parse(priorConfirmationSentAtIso)
    : NaN;
  const withinIdempotencyWindow =
    Number.isFinite(priorConfirmationSentAtMs) &&
    nowMs - priorConfirmationSentAtMs < STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS;

  // 1. Memory signal: stop_active = true. Idempotent — second STOP
  //    just re-upserts the same row. We preserve any prior
  //    stop_confirmation_sent_at so the 24h idempotency check above
  //    remains consistent across multiple STOPs. If we DO end up
  //    sending a fresh confirmation below, we re-upsert with the
  //    bumped timestamp at that point (separate write, after the
  //    SendBlue call resolves).
  await deps.memoryStore.upsert({
    user_id: userId,
    kind: 'stop_active',
    scope_key: null,
    detail: {
      active: true,
      recorded_at: nowIso,
      ...(priorConfirmationSentAtIso
        ? { stop_confirmation_sent_at: priorConfirmationSentAtIso }
        : {})
    },
    source: 'user_confirmed',
    confidence: 1.0
  });
  // 2. Feedback event.
  await deps.feedbackStore.write({
    user_id: userId,
    alert_id: null,
    sender_email: null,
    kind: 'stop',
    detail: { source: 'sendblue_inbound', provider_message_id: providerMessageId }
  });
  // 3. Audit (stop_recorded — the load-bearing enforcement signal).
  await deps.auditStore.write({
    actor_user_id: userId,
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

  // 4. Phase v0.5.5 — STOP confirmation send. Best-effort, idempotent
  //    over 24h, no retry on failure.
  //
  //    Skip when:
  //      - the stopConfirmation dep is absent (founder hasn't wired
  //        it; preserves exact v0.5.4 behavior), OR
  //      - we already sent a confirmation < 24h ago.
  //
  //    The confirmation is the ONLY allowed outbound exception after
  //    STOP — it bypasses the outbound-sender STOP enforcement on
  //    purpose, because it IS the message telling the user that STOP
  //    was received. The normal FOMO alert pipeline continues to be
  //    blocked by `fomo.send.stop_enforced`.
  if (deps.stopConfirmation && !withinIdempotencyWindow) {
    let outcome: import('../adapters/sendblue/client.js').SendOutcome;
    try {
      outcome = await deps.stopConfirmation.send({
        to: fromNumber,
        content: STOP_CONFIRMATION_BODY
      });
    } catch (err) {
      // Network/abort path: SendBlueClient.send normally returns a
      // SendOutcome with kind='send_status_unknown' for these, but
      // defense-in-depth catches the throw too. Per Q6: audit, do not
      // retry, do not re-throw (the inbound webhook 200 response is
      // already in flight; throwing here would cascade to a 500 and
      // trigger a SendBlue webhook retry, which would then idempotency-
      // gate at the inbound_replies UNIQUE constraint — still safe, but
      // noisier than a clean fail-soft).
      // Phase v0.5.15 — sanitized error_code/error_reason; the raw
      // err.message slice is removed entirely (deny-by-default).
      const sanitized = sanitizeProviderError({
        raw_provider_code: 'send_throw',
        network_error_code:
          err && typeof err === 'object' && 'code' in err && typeof (err as { code: unknown }).code === 'string'
            ? ((err as { code: string }).code)
            : null
      });
      await deps.auditStore.write({
        actor_user_id: userId,
        actor_ip: null,
        actor_user_agent: null,
        action: 'fomo.sendblue.stop_confirmation_failed',
        target: 'memory_signal:stop_active',
        result: 'failure',
        detail: {
          from_slug: fromSlug,
          provider_message_id: providerMessageId,
          error_code: sanitized.error_code,
          error_reason: sanitized.error_reason,
          reason: 'best-effort send threw; no retry per v0.5.5 Q6'
        }
      });
      return buildStopOutcome();
    }

    if (outcome.kind === 'sent') {
      // Re-upsert with the bumped stop_confirmation_sent_at so the
      // next STOP within 24h skips this branch via withinIdempotencyWindow.
      await deps.memoryStore.upsert({
        user_id: userId,
        kind: 'stop_active',
        scope_key: null,
        detail: {
          active: true,
          recorded_at: nowIso,
          stop_confirmation_sent_at: nowIso
        },
        source: 'user_confirmed',
        confidence: 1.0
      });
      await deps.auditStore.write({
        actor_user_id: userId,
        actor_ip: null,
        actor_user_agent: null,
        action: 'fomo.sendblue.stop_confirmation_sent',
        target: 'memory_signal:stop_active',
        result: 'success',
        detail: {
          from_slug: fromSlug,
          provider_status: outcome.providerStatus ?? null,
          provider_message_handle: outcome.providerMessageHandle || null,
          message_preview: STOP_CONFIRMATION_BODY,
          idempotency_window_remaining_seconds: Math.floor(
            STOP_CONFIRMATION_IDEMPOTENCY_WINDOW_MS / 1000
          ),
          target_user_id: userId
        }
      });
    } else {
      // 'failed' or 'send_status_unknown' both write the _failed audit
      // and do NOT retry. Per Q6: the confirmation is courtesy; a
      // missed confirmation is recoverable by the user re-STOPing
      // after the 24h window expires. We deliberately do NOT bump
      // stop_confirmation_sent_at on failure — so a re-STOP within
      // 24h will retry once (since priorConfirmationSentAtIso stays
      // null and withinIdempotencyWindow stays false). This is the
      // tightest "no retry but one re-attempt on failed first-send"
      // semantics consistent with Q6.
      await deps.auditStore.write({
        actor_user_id: userId,
        actor_ip: null,
        actor_user_agent: null,
        action: 'fomo.sendblue.stop_confirmation_failed',
        target: 'memory_signal:stop_active',
        result: 'failure',
        detail: (() => {
          // Phase v0.5.15 — sanitized error_code/error_reason.
          const s = sanitizeProviderError({
            raw_provider_code: outcome.providerError?.error_message ?? null,
            http_status: outcome.httpStatus > 0 ? outcome.httpStatus : null
          });
          return {
            from_slug: fromSlug,
            provider_message_id: providerMessageId,
            provider_status: outcome.providerStatus ?? null,
            http_status: outcome.httpStatus,
            outcome_kind: outcome.kind,
            error_code: s.error_code,
            error_reason: s.error_reason,
            reason: 'best-effort send did not return kind=sent; no retry per v0.5.5 Q6'
          };
        })()
      });
    }
  }

  return buildStopOutcome();
}

function buildStopOutcome(): RouteOutcome {
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
  userId: string,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  await deps.memoryStore.upsert({
    user_id: userId,
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
    actor_user_id: userId,
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
  userId: string,
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
      actor_user_id: userId,
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

  // Phase v0.5.10 — feedback_event write moved to feedback-routing.ts
  // policy module. Called by applyParseResult BEFORE this function.

  await deps.auditStore.write({
    actor_user_id: userId,
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
  userId: string,
  matchedAlert: Alert | null,
  feedbackKind: 'user_ignored' | 'false_positive',
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  if (!matchedAlert) {
    await deps.auditStore.write({
      actor_user_id: userId,
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

  // Phase v0.5.10 — feedback_event write moved to feedback-routing.ts
  // policy module. Called by applyParseResult BEFORE this function.

  await deps.auditStore.write({
    actor_user_id: userId,
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
  userId: string,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  // We need a sender to suppress. Without an alert match, we can't
  // know which sender; degrade to audit-only.
  if (!matchedAlert) {
    await deps.auditStore.write({
      actor_user_id: userId,
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
    user_id: userId,
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

  // Phase v0.5.10 — feedback_event write moved to feedback-routing.ts
  // policy module. Called by applyParseResult BEFORE this function.

  await deps.auditStore.write({
    actor_user_id: userId,
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
  userId: string,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string,
  // Phase v0.7.0A — the inbound E.164. Threaded through so the explain
  // surface can send an outbound explanation back to the same number.
  // Same convention as v0.5.5 applyStop + v0.5.14 feedbackAck wiring.
  // NEVER logged, NEVER persisted — used only as the `to` argument to
  // deps.explainSender.send.
  fromNumber: string,
  // Phase v0.7.0A — clock injection so the 24h eligibility window is
  // testable. Caller passes the same `now` used elsewhere in the route.
  now: () => Date
): Promise<RouteOutcome> {
  // 'why' transitions sent → replied (acknowledgement) but does NOT
  // proceed to a terminal state — the user wants info, not closure.
  if (matchedAlert) {
    await walkTransition(deps, matchedAlert, 'sent', 'replied', 'sendblue:why');
    // Phase v0.5.10 — feedback_event write moved to feedback-routing.ts
    // policy module. Called by applyParseResult BEFORE this function.
  }
  await deps.auditStore.write({
    actor_user_id: userId,
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

  // Phase v0.7.0A — Explainability Surface (best-effort outbound).
  //
  // Fires AFTER the v0.5.10 baseline writes (state-machine + reply_parsed
  // audit) so a failure here NEVER regresses existing behavior.
  //
  // Gates (ALL must hold):
  //   (1) deps.killSwitches.explain_surface_enabled === true
  //   (2) deps.explainSender is wired (bootstrap injected SendBlue client)
  //   (3) fromNumber is present (defensive — caller always provides it)
  await serveExplanation({
    deps,
    userId,
    matchedAlert,
    fromNumber,
    now
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

// Phase v0.7.0A — Explain surface helper.
//
// Decides empty-state vs composed-explanation, sends via SendBlue,
// audits brevio.explain.served. Structural-only audit detail per
// founder lock: alert_id_hash, source_surface, template_version,
// empty_state, send_outcome_kind. NEVER raw sender/subject/body/reason.
//
// Best-effort: throw/failure NEVER aborts the caller. The applyWhy
// state-machine + feedback-routing writes are already durable.
async function serveExplanation(args: {
  deps: SendBlueInboundRouteDeps;
  userId: string;
  matchedAlert: Alert | null;
  fromNumber: string;
  now: () => Date;
}): Promise<void> {
  const { deps, userId, matchedAlert, fromNumber, now } = args;

  // Gate 1 — kill switch.
  if (!deps.killSwitches.explain_surface_enabled) return;
  // Gate 2 — dep wired.
  if (deps.explainSender === undefined) return;
  // Gate 3 — fromNumber present (defensive).
  if (typeof fromNumber !== 'string' || fromNumber.length === 0) return;

  // Decide content: empty-state vs composed explanation.
  // EmptyState = matchedAlert is null OR matchedAlert is older than the
  // 24h lookup window OR rank_result is missing OR rank.reason is
  // empty/whitespace OR compose returns null.
  let content: string;
  let emptyState: boolean;

  if (matchedAlert === null) {
    content = EMPTY_STATE_COPY;
    emptyState = true;
  } else {
    const ageMs = now().getTime() - Date.parse(matchedAlert.created_at);
    if (!Number.isFinite(ageMs) || ageMs < 0 || ageMs > EXPLAIN_LOOKUP_WINDOW_MS) {
      content = EMPTY_STATE_COPY;
      emptyState = true;
    } else {
      const rank = await deps.rankResultStore.get(userId, matchedAlert.message_id);
      const composed = composeExplanationFromReason(rank?.reason ?? null);
      if (composed === null) {
        content = EMPTY_STATE_COPY;
        emptyState = true;
      } else {
        content = composed;
        emptyState = false;
      }
    }
  }

  const alertIdForAudit = emptyState ? null : matchedAlert?.alert_id ?? null;
  const auditTarget = alertIdForAudit ? `alert:${alertIdForAudit}` : 'route:sendblue.inbound';

  // Send. Both throw and non-'sent' outcome route to the failure audit.
  let outcome: import('../adapters/sendblue/client.js').SendOutcome;
  try {
    outcome = await deps.explainSender.send({ to: fromNumber, content });
  } catch (err) {
    const sanitized = sanitizeProviderError({
      raw_provider_code: 'send_throw',
      network_error_code:
        err && typeof err === 'object' && 'code' in err && typeof (err as { code: unknown }).code === 'string'
          ? ((err as { code: string }).code)
          : null
    });
    await deps.auditStore.write({
      actor_user_id: userId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.explain.served',
      target: auditTarget,
      result: 'failure',
      detail: {
        alert_id_hash: hashAlertIdForAudit(alertIdForAudit),
        source_surface: EXPLAIN_SOURCE_SURFACE,
        template_version: EXPLAIN_TEMPLATE_VERSION,
        empty_state: emptyState,
        send_outcome_kind: null,
        error_code: sanitized.error_code,
        error_reason: sanitized.error_reason
      }
    });
    return;
  }

  if (outcome.kind === 'sent') {
    await deps.auditStore.write({
      actor_user_id: userId,
      actor_ip: null,
      actor_user_agent: null,
      action: 'brevio.explain.served',
      target: auditTarget,
      result: 'success',
      detail: {
        alert_id_hash: hashAlertIdForAudit(alertIdForAudit),
        source_surface: EXPLAIN_SOURCE_SURFACE,
        template_version: EXPLAIN_TEMPLATE_VERSION,
        empty_state: emptyState,
        send_outcome_kind: outcome.kind
      }
    });
    return;
  }

  // Provider returned 'failed' / 'send_status_unknown'. Same sanitized
  // chokepoint as the v0.5.14 feedback ack failure audit.
  const sanitized = sanitizeProviderError({
    raw_provider_code: outcome.providerError?.error_message ?? null,
    http_status: outcome.httpStatus > 0 ? outcome.httpStatus : null
  });
  await deps.auditStore.write({
    actor_user_id: userId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'brevio.explain.served',
    target: auditTarget,
    result: 'failure',
    detail: {
      alert_id_hash: hashAlertIdForAudit(alertIdForAudit),
      source_surface: EXPLAIN_SOURCE_SURFACE,
      template_version: EXPLAIN_TEMPLATE_VERSION,
      empty_state: emptyState,
      send_outcome_kind: outcome.kind,
      error_code: sanitized.error_code,
      error_reason: sanitized.error_reason
    }
  });
}

async function applyUnclear(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  matchedAlert: Alert | null,
  fromSlug: string,
  providerMessageId: string,
  lowConfidenceForced: boolean
): Promise<RouteOutcome> {
  await deps.auditStore.write({
    actor_user_id: userId,
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

// Phase v0.5.10 — positive-signal intents (this_mattered / more_like_this).
// Per Q5.A silent founder lock: NO new outbound iMessage acknowledgment.
// Per Q4.A consumer-arm lock: NO new applyFeedback consumer arms (the
// routing module above already wrote the feedback_event; this function
// only walks state-machine to `replied` for symmetry with the other
// soft-intent paths and audits a reply_parsed row).
//
// The state-machine transition stops at `replied` (not `ignored`/`snoozed`/
// etc.) — a positive confirmation doesn't change what action the user
// takes on the alert; it just acknowledges the alert was right. Future
// HMR Feedback Acknowledgment phase can render a response message here.
async function applyPositiveAcknowledge(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  matchedAlert: Alert | null,
  intent: 'this_mattered' | 'more_like_this',
  fromSlug: string,
  providerMessageId: string
): Promise<RouteOutcome> {
  if (matchedAlert) {
    await walkTransition(deps, matchedAlert, 'sent', 'replied', `sendblue:${intent}`);
    // No further state transition — positive confirmation acknowledges
    // the alert was right; it doesn't dismiss/snooze the alert action.
  }
  await deps.auditStore.write({
    actor_user_id: userId,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.sendblue.reply_parsed',
    target: matchedAlert ? `alert:${matchedAlert.alert_id}` : 'route:sendblue.inbound',
    result: 'success',
    detail: {
      from_slug: fromSlug,
      provider_message_id: providerMessageId,
      alert_id: matchedAlert?.alert_id,
      intent,
      intent_source: 'classifier',
      alert_matched: matchedAlert !== null
    }
  });
  return Object.freeze({
    classified_intent: intent,
    source: 'classifier' as const,
    alert_matched: matchedAlert !== null,
    applied_state_transition: matchedAlert ? { from: 'sent' as AlertState, to: 'replied' as AlertState } : null,
    // No legacy FeedbackEventKind for the new positive intents — the
    // routing module above wrote a generic 'approved' verb to
    // feedback_events.kind. The RouteOutcome.feedback_kind is null to
    // signal "no legacy-kind row here" (existing route-shape callers
    // expecting a FeedbackEventKind continue to read null safely).
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
  deps: SendBlueInboundRouteDeps,
  userId: string
): Promise<Alert | null> {
  const recent = await deps.alertStore.recent(userId, 20);
  for (const a of recent) {
    const state = (await deps.transitions.currentState(a.alert_id)) ?? 'detected';
    if (state === 'sent') return a;
  }
  return null;
}

async function resolveAlertSubject(
  deps: SendBlueInboundRouteDeps,
  userId: string,
  matchedAlert: Alert | null
): Promise<string> {
  if (!matchedAlert) return '(no alert context available)';
  const rank = await deps.rankResultStore.get(userId, matchedAlert.message_id);
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
  // SendBlue puts the configured webhook secret in a request
  // header. Public docs don't name it; we read whatever header the
  // operator configured via `deps.webhookSecretHeader` (default
  // `sb-signing-secret`, overridable via env at boot time).
  const secretHeaderValue = getHeader(req, deps.webhookSecretHeader);
  return handleSendBlueInbound(
    {
      body: bodyBuf.toString('utf8'),
      secretHeaderValue
    },
    deps
  );
}
