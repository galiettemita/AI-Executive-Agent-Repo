// Slack interactivity inbound — Phase 3D.2.
//
// One endpoint:
//
//   POST /slack/interactivity
//     Auth: SLACK signing-secret HMAC verification (NOT session auth).
//     The request comes from Slack's servers, signed with the app's
//     signing secret. We MUST verify the signature before parsing the
//     payload, AND verify the timestamp is fresh (≤300s) to thwart
//     replay attacks.
//
// Defense-in-depth layers:
//
//   1. Audit `fomo.slack.interaction_received` for every inbound POST,
//      BEFORE signature verification. Even a flood of unsigned requests
//      is visible in the audit log.
//   2. Reject signatures: stale timestamp / malformed / mismatched HMAC.
//   3. Reject malformed payloads (missing fields).
//   4. Reject wrong channel (channel.id !== SLACK_FOUNDER_CHANNEL_ID).
//   5. Reject wrong user (only when SLACK_FOUNDER_USER_ID is set;
//      best-effort — the runbook recommends setting it).
//   6. Idempotency: if the alert is already in approved/rejected state,
//      return 200 with a fresh-cycle skip (first-wins). Slack retries
//      up to 3× on 5xx; we MUST be safe to receive the same payload
//      multiple times.
//
// Privacy invariants in audit + feedback rows:
//   * NEVER persist the raw Slack payload string.
//   * NEVER persist the full Slack user_id (only a truncated slug suffix).
//   * NEVER persist message text. Operational identifiers only:
//     alert_id, action_id, decision, channel_id, user_slug.
//
// On approval/rejection success:
//   * write alert state transition queued_for_review → approved | rejected
//   * write feedback event founder_approved / founder_rejected
//   * audit fomo.slack.approval_captured (success row)
//   * schedule chat.update of the original card (visual feedback). The
//     chat.update is FIRE-AND-FORGET — its success / failure is NOT a
//     gate criterion for the route's response. The DB-side transition
//     has already landed.
//
// Response shape:
//   * 200 with empty JSON for happy path / idempotent duplicate.
//     Slack treats anything else as a retry trigger.
//   * 401 when signature verification fails (Slack will not retry).
//   * 400 when payload is malformed / alert unknown (no retry).
//   * 403 when authorization fails (wrong channel / wrong user).
//   * 500 only on truly unexpected internal error (Slack WILL retry).

import { Buffer } from 'node:buffer';
import type http from 'node:http';

import { type SlackClient, verifySlackSignature } from '../adapters/slack/client.js';
import { type AuditStore } from '../core/audit.js';
import { type AlertStateTransitionStore } from '../core/alert-state-transitions.js';
import { type KillSwitches } from '../core/kill-switches.js';
import { transition } from '../core/state-machine.js';
import { type AlertState } from '../core/state-machine.js';
import { type Alert, type AlertStore } from '../memory/alerts.js';
import { type FeedbackStore, mapLegacyFeedbackKind } from '../memory/feedback-events.js';
import { type RankResultStore } from '../memory/rank-results.js';
import { applyEgressForSlackCard, type RawEmailContext } from '../core/egress-policy.js';

/* ---------------------------------------------------------------------- */
/* Deps                                                                   */
/* ---------------------------------------------------------------------- */

export interface SlackInteractivityRouteDeps {
  // Slack signing secret from the app's Basic Information panel.
  // Required for signature verification.
  readonly signingSecret: string;
  // Founder channel id. Inbound interactions from any other channel
  // are rejected as unauthorized.
  readonly founderChannelId: string;
  // Optional founder Slack user-id. When set, only this user can
  // approve/reject. When unset, the runbook recommends setting it; the
  // route accepts any user (best-effort).
  readonly founderUserId?: string;

  readonly killSwitches: KillSwitches;
  readonly alertStore: AlertStore;
  readonly rankResultStore: RankResultStore;
  readonly transitions: AlertStateTransitionStore;
  readonly feedbackStore: FeedbackStore;
  readonly auditStore: AuditStore;

  // SlackClient used for chat.update of the original card. Optional —
  // when undefined, the route still completes the state transition
  // and audit, just skips the visual update.
  readonly slackClient?: SlackClient;

  // Optional: a function that, given a Gmail message_id and user_id,
  // returns the RawEmailContext used to build the resolution card via
  // applyEgressForSlackCard. For 3D.2 we re-derive the redacted view
  // from the rank_results.reason and stored metadata. The route can't
  // call gmail.read directly (no token context here). If undefined,
  // chat.update is skipped (still a successful approval).
  readonly resolveEmailContext?: (
    user_id: string,
    message_id: string
  ) => Promise<RawEmailContext | null>;

  // Optional clock for tests.
  readonly now?: () => Date;
}

/* ---------------------------------------------------------------------- */
/* HTTP response shape (mirrors oauth-google.ts)                          */
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
/* Slack payload shape                                                    */
/* ---------------------------------------------------------------------- */

// Subset of the Slack block_actions payload we use. Everything else is
// ignored. Slack's full payload is large (~50 fields); narrowing here
// is intentional — anything we DON'T read here, we DON'T accidentally
// persist or echo.
interface SlackInteractivityPayload {
  readonly type: string; // 'block_actions'
  readonly user?: { readonly id?: string; readonly username?: string };
  readonly channel?: { readonly id?: string; readonly name?: string };
  readonly actions?: ReadonlyArray<{
    readonly type?: string;
    readonly action_id?: string;
    readonly block_id?: string;
    readonly value?: string;
  }>;
  readonly message?: { readonly ts?: string };
  readonly container?: { readonly message_ts?: string; readonly channel_id?: string };
}

function parsePayload(form: URLSearchParams): SlackInteractivityPayload | null {
  const raw = form.get('payload');
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === 'object') return parsed as SlackInteractivityPayload;
  } catch {
    return null;
  }
  return null;
}

/* ---------------------------------------------------------------------- */
/* Audit helpers (sanitized — no payload leakage)                         */
/* ---------------------------------------------------------------------- */

// Truncate a Slack user_id ("U01ABCDEFGH") to its 4-char suffix slug
// ("EFGH"). Enough to correlate audit rows across a smoke test without
// persisting the full identifier.
function userSlug(userId: string | undefined): string {
  if (!userId) return '<unknown>';
  return userId.length <= 4 ? userId : userId.slice(-4);
}

// Same for channel ids.
function channelSlug(channelId: string | undefined): string {
  if (!channelId) return '<unknown>';
  return channelId.length <= 4 ? channelId : channelId.slice(-4);
}

interface InteractionAuditDetail {
  readonly alert_id?: string;
  readonly action_id?: string;
  readonly decision_code?: string;
  readonly user_slug?: string;
  readonly channel_slug?: string;
  readonly from_state?: AlertState;
  readonly to_state?: AlertState;
}

/* ---------------------------------------------------------------------- */
/* Decision extraction                                                    */
/* ---------------------------------------------------------------------- */

type Decision = 'approved' | 'rejected';

function decisionFromAction(action: { action_id?: string } | undefined): Decision | null {
  if (!action) return null;
  if (action.action_id === 'fomo.approve') return 'approved';
  if (action.action_id === 'fomo.reject') return 'rejected';
  return null;
}

function alertIdFromAction(action: { action_id?: string; block_id?: string; value?: string } | undefined): string | null {
  if (!action) return null;
  // block_id format: "fomo_alert:<alert_id>". Prefer block_id over
  // value (block_id is harder to tamper with at the Slack-client side
  // because it's set by buildFounderReviewBlocks).
  if (typeof action.block_id === 'string' && action.block_id.startsWith('fomo_alert:')) {
    const id = action.block_id.slice('fomo_alert:'.length);
    if (id.length > 0) return id;
  }
  // Fallback: button value (set to alert_id by the block builder).
  if (typeof action.value === 'string' && action.value.length > 0) return action.value;
  return null;
}

/* ---------------------------------------------------------------------- */
/* Handler                                                                */
/* ---------------------------------------------------------------------- */

export interface HandleInteractivityInput {
  // Raw request body string (NOT JSON-parsed). Required for signature
  // verification — the HMAC is computed over the original bytes.
  readonly body: string;
  // Verbatim X-Slack-Request-Timestamp header.
  readonly timestamp: string;
  // Verbatim X-Slack-Signature header.
  readonly signature: string;
}

export async function handleSlackInteractivity(
  input: HandleInteractivityInput,
  deps: SlackInteractivityRouteDeps
): Promise<HttpResponse> {
  const now = deps.now ?? ((): Date => new Date());

  // 1. Audit interaction_received BEFORE signature verification.
  //    Sanitized — only the timestamp and body length, not the body.
  await deps.auditStore.write({
    actor_user_id: null,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.slack.interaction_received',
    target: 'route:slack.interactivity',
    result: 'success',
    detail: {
      slack_timestamp: input.timestamp,
      body_bytes: input.body.length
    }
  });

  // 2. Kill switch — defense-in-depth at request time. The bootstrap is
  //    expected to skip mounting this route when slack_review_enabled is
  //    false, but the handler still refuses to do anything when the
  //    switch is off. Returns 200 (we don't want to leak whether the
  //    route is wired) but does NOT process the request.
  if (!deps.killSwitches.slack_review_enabled) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.signature_invalid',
      target: 'route:slack.interactivity',
      result: 'failure',
      detail: { error_code: 'kill_switch_off' }
    });
    return jsonResponse(200, {});
  }

  // 3. Signature verification.
  const verify = verifySlackSignature({
    signingSecret: deps.signingSecret,
    timestamp: input.timestamp,
    signature: input.signature,
    body: input.body,
    now: () => Math.floor(now().getTime() / 1000)
  });
  if (!verify.ok) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.signature_invalid',
      target: 'route:slack.interactivity',
      result: 'failure',
      detail: { error_code: verify.reason }
    });
    return jsonResponse(401, { error: 'signature_invalid' });
  }

  // 4. Parse payload (form-urlencoded with a `payload` field carrying JSON).
  let form: URLSearchParams;
  try {
    form = new URLSearchParams(input.body);
  } catch {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.payload_invalid',
      target: 'route:slack.interactivity',
      result: 'failure',
      detail: { error_code: 'body_not_form_encoded' }
    });
    return jsonResponse(400, { error: 'payload_invalid' });
  }
  const payload = parsePayload(form);
  if (!payload || payload.type !== 'block_actions') {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.payload_invalid',
      target: 'route:slack.interactivity',
      result: 'failure',
      detail: { error_code: 'unexpected_payload_type' }
    });
    return jsonResponse(400, { error: 'payload_invalid' });
  }

  const action = payload.actions?.[0];
  const decision = decisionFromAction(action);
  const alert_id = alertIdFromAction(action);
  const channel_id = payload.channel?.id ?? payload.container?.channel_id;
  const user_id_slack = payload.user?.id;
  const msg_ts = payload.message?.ts ?? payload.container?.message_ts;

  const detailBase: InteractionAuditDetail = {
    alert_id: alert_id ?? undefined,
    action_id: action?.action_id,
    user_slug: userSlug(user_id_slack),
    channel_slug: channelSlug(channel_id)
  };

  if (!decision || !alert_id) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.payload_invalid',
      target: 'route:slack.interactivity',
      result: 'failure',
      detail: { ...detailBase, error_code: 'missing_decision_or_alert_id' }
    });
    return jsonResponse(400, { error: 'payload_invalid' });
  }

  // 5. Channel + user authorization.
  if (channel_id !== deps.founderChannelId) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.approval_unauthorized',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { ...detailBase, error_code: 'wrong_channel' }
    });
    return jsonResponse(403, { error: 'unauthorized', reason: 'wrong_channel' });
  }
  if (deps.founderUserId && user_id_slack !== deps.founderUserId) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.approval_unauthorized',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { ...detailBase, error_code: 'wrong_user' }
    });
    return jsonResponse(403, { error: 'unauthorized', reason: 'wrong_user' });
  }

  // 6. Look up alert.
  const alert = await deps.alertStore.get(alert_id);
  if (!alert) {
    await deps.auditStore.write({
      actor_user_id: null,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.payload_invalid',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { ...detailBase, error_code: 'unknown_alert_id' }
    });
    return jsonResponse(400, { error: 'unknown_alert_id' });
  }

  // 7. Idempotency: first terminal decision wins.
  const currentState = (await deps.transitions.currentState(alert_id)) ?? 'detected';
  const targetState: AlertState = decision === 'approved' ? 'approved' : 'rejected';

  if (currentState === 'approved' || currentState === 'rejected') {
    await deps.auditStore.write({
      actor_user_id: alert.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.approval_duplicate',
      target: `alert:${alert_id}`,
      result: 'success',
      detail: { ...detailBase, from_state: currentState, to_state: targetState }
    });
    return jsonResponse(200, { ok: true, duplicate: true, current_state: currentState });
  }

  // Validate the transition before writing. If currentState is not
  // queued_for_review (e.g. someone manually moved the alert to failed),
  // the transition is invalid — we audit and return 400 rather than
  // forcing a state-machine violation.
  const validation = transition(currentState, targetState, `slack:${decision}`);
  if ('error' in validation) {
    await deps.auditStore.write({
      actor_user_id: alert.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.slack.payload_invalid',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        ...detailBase,
        error_code: 'invalid_state_transition',
        from_state: currentState,
        to_state: targetState
      }
    });
    return jsonResponse(400, { error: 'invalid_state_transition', from: currentState, to: targetState });
  }

  // 8. Apply: state transition + feedback event + success audit.
  const decidedAt = now().toISOString();

  await deps.transitions.write({
    alert_id,
    user_id: alert.user_id,
    from_state: currentState,
    to_state: targetState,
    reason: `slack:${decision} actor_slug=${userSlug(user_id_slack)}`
  });

  // Phase v0.5.9 — explicit source_surface='email_alert' (Slack interactivity
  // is the founder-review path for the email_alert surface). The legacy kind
  // is still passed; storage keeps it literal; the audit emitter below uses
  // mapLegacyFeedbackKind to derive the extended detail.
  const legacyKind = decision === 'approved' ? 'founder_approved' : 'founder_rejected';
  const writtenEvent = await deps.feedbackStore.write({
    user_id: alert.user_id,
    alert_id,
    kind: legacyKind,
    sender_email: null,
    source_surface: 'email_alert',
    detail: {
      rank_result_id: alert.rank_result_id,
      message_id: alert.message_id,
      actor_slug: userSlug(user_id_slack)
    }
  });

  // Phase v0.5.9 — emit feedback.written audit with the extended Q6.A detail
  // (source_surface, verb, dimension, role, legacy_kind). The legacy kind
  // (`founder_approved` / `founder_rejected`) maps to verb=approved/rejected
  // + role=founder via mapLegacyFeedbackKind. Slack interactivity does NOT
  // invoke the applyFeedback consumer: verb='approved'/'rejected' do not
  // match the v0.5.9 hardcoded consumer arm (which is (email_alert, ignored,
  // sender) only). Future surfaces / future phases can wire the consumer
  // here when they activate.
  const mapped = mapLegacyFeedbackKind(legacyKind);
  await deps.auditStore.write({
    actor_user_id: alert.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'feedback.written',
    target: `alert:${alert_id}`,
    result: 'success',
    detail: {
      feedback_event_id: writtenEvent.id ?? null,
      source_surface: 'email_alert',
      verb: mapped?.verb,
      dimension: mapped?.overlay.dimension,
      role: mapped?.overlay.role,
      legacy_kind: legacyKind,
      sender_present: false,
      // Phase v0.5.10 — intent_source symmetry. Slack interactivity is
      // the founder approve/reject path; identified here so audit
      // consumers can split feedback-event creators by source.
      intent_source: 'slack_interactivity'
    }
  });

  await deps.auditStore.write({
    actor_user_id: alert.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.slack.approval_captured',
    target: `alert:${alert_id}`,
    result: 'success',
    detail: {
      ...detailBase,
      from_state: currentState,
      to_state: targetState,
      decided_at: decidedAt
    }
  });

  // 9. chat.update — fire-and-forget; failures are NON-fatal. The state
  //    transition has already landed. We log update failures into audit
  //    but the founder's HTTP response is already success.
  if (deps.slackClient && msg_ts && deps.resolveEmailContext) {
    try {
      const raw = await deps.resolveEmailContext(alert.user_id, alert.message_id);
      if (raw) {
        const view = applyEgressForSlackCard(raw);
        // Fetch the rank_results row to re-populate the card.
        const rank = await deps.rankResultStore.get(alert.user_id, alert.message_id);
        if (rank) {
          await deps.slackClient.updateFounderReviewCard({
            ts: msg_ts,
            channel: channel_id,
            alert_id,
            user_id: alert.user_id,
            view,
            rank: {
              label: rank.label,
              score: rank.score,
              reason: rank.reason,
              model_name: rank.model_name,
              prompt_version: rank.prompt_version
            },
            decision: {
              kind: targetState as 'approved' | 'rejected',
              at: decidedAt,
              actor: user_id_slack ?? 'unknown'
            }
          });
        }
      }
    } catch (err) {
      // Non-fatal — the state transition succeeded.
      await deps.auditStore.write({
        actor_user_id: alert.user_id,
        actor_ip: null,
        actor_user_agent: null,
        action: 'fomo.slack.failed',
        target: `alert:${alert_id}`,
        result: 'failure',
        detail: {
          ...detailBase,
          error_code: 'chat_update_failed',
          reason: err instanceof Error ? err.message : String(err)
        }
      });
    }
  }

  return jsonResponse(200, { ok: true, alert_id, decision: targetState });
}

// `Alert` import is used by callers; keep it available without unused-import warning.
export type { Alert };

/* ---------------------------------------------------------------------- */
/* HTTP adapter                                                           */
/* ---------------------------------------------------------------------- */

// Reads an entire request body up to a small limit. Slack interactivity
// payloads are ~5-10KB; 32KB is generous without DoS risk.
async function readBody(req: http.IncomingMessage, maxBytes = 32_768): Promise<Buffer> {
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

// Returns the response to send, or null when the request did not match
// the slack-interactivity route. The server in index.ts checks for null
// and falls through.
export async function tryHandleSlackInteractivityRequest(
  req: http.IncomingMessage,
  deps: SlackInteractivityRouteDeps
): Promise<HttpResponse | null> {
  const method = req.method ?? 'GET';
  const url = new URL(req.url ?? '/', 'http://localhost');
  if (!(method === 'POST' && url.pathname === '/slack/interactivity')) {
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
  return handleSlackInteractivity(
    {
      body: bodyBuf.toString('utf8'),
      timestamp: getHeader(req, 'x-slack-request-timestamp'),
      signature: getHeader(req, 'x-slack-signature')
    },
    deps
  );
}
