// Outbound Sender Worker — Phase 3E.1.
//
// One cycle = one runOnce() call. For each user with a Gmail cursor
// (the v0.1 founder), find every alert whose latest state-transition
// is 'approved' but not yet 'sent' / 'send_status_unknown' / 'failed',
// render a deterministic founder text, and dispatch
// sendblue.send_user_message exactly once per alert.
//
// Scope (founder directive 2026-05-25 — 3E.1 substrate only):
//   * NO LLM-generated voice / freeform copy. Use the deterministic
//     founder-text-template only.
//   * NO auto-retry of `send_status_unknown` — ambiguous outcomes
//     reach a terminal state and stay there until an operator looks.
//   * NO reply parsing. NO webhook handling. NO Slack edits.
//   * NO auto-send (intent='manual_send'; FOMO_AUTO_SEND_ENABLED stays
//     off through v0.1).
//
// Defense-in-depth at the worker boundary (in addition to the gate):
//
//   * Founder-phone allowlist: the worker's dep MUST resolve every
//     user to ONE allowlisted destination (the founder's number, from
//     FOMO_FOUNDER_PHONE_NUMBER). If `destinationFor(user_id)` returns
//     null, the worker refuses to dispatch and audits
//     `fomo.send.unauthorized_destination` without calling SendBlue.
//   * Idempotency: re-finding an alert in state 'approved' is the
//     only trigger; once the worker transitions it to sent /
//     send_status_unknown / failed, the next cycle's
//     findAlertIdsInState('approved', ...) does not return it. The
//     state machine itself is the idempotency guard.
//
// Audit footprint per processed alert (happy path):
//   policy.decided  (gmail.read)
//   tool.invoked    (gmail.read)
//   fomo.send.attempted
//   policy.decided  (sendblue.send_user_message)
//   tool.invoked    (sendblue.send_user_message)
//   fomo.send.{succeeded|failed|status_unknown}
//   state.transitioned
//
// On unauthorized destination (defense-in-depth):
//   fomo.send.unauthorized_destination
//   state.transitioned (approved → failed)

import { type SendOutcome } from '../adapters/sendblue/client.js';
import { type AlertStateTransitionStore } from '../core/alert-state-transitions.js';
import { type AuditStore } from '../core/audit.js';
import {
  applyEgressForSlackCard,
  type RawEmailContext,
  type SlackEgressView
} from '../core/egress-policy.js';
import {
  FOUNDER_TEXT_TEMPLATE_VERSION,
  renderFounderText
} from '../core/founder-text-template.js';
import { decidePolicy, type PolicyGateDeps } from '../core/policy-gate.js';
import { transition } from '../core/state-machine.js';
import { type ToolInvocationStore } from '../core/tool-invocations.js';
import { AuthorizedToolCall, type DispatchTable } from '../dispatch/dispatcher.js';
import { type AlertStore } from '../memory/alerts.js';
import { type GmailCursorStore } from '../memory/gmail-cursors.js';
import { type MemorySignalStore } from '../memory/memory-signals.js';
import { type RankResultStore, type RankLabel } from '../memory/rank-results.js';

export interface OutboundSenderDeps {
  readonly dispatch: DispatchTable;
  readonly auditStore: AuditStore;
  readonly toolInvocationStore: ToolInvocationStore;
  readonly gateDeps: PolicyGateDeps;

  readonly cursorStore: GmailCursorStore;
  readonly alertStore: AlertStore;
  readonly rankResultStore: RankResultStore;
  readonly transitions: AlertStateTransitionStore;
  // Phase 3F.1 — required for STOP enforcement. The worker consults
  // the user's `stop_active` memory signal before dispatching ANY
  // send. When active, the worker refuses (audit
  // `fomo.send.stop_enforced`, transition `approved → failed`) and
  // NEVER calls SendBlue. Per founder directive 2026-05-26: STOP
  // enforcement is deterministic, no LLM decides whether STOP means
  // stop. The signal is flipped by the /sendblue/inbound route's
  // deterministic pre-pass on STOP / UNSUBSCRIBE / CANCEL.
  readonly memoryStore: MemorySignalStore;

  // Returns the ONE allowlisted destination phone for this user (in
  // E.164 form, e.g. '+14155551234'), or null when this user has no
  // configured destination.
  //
  //   v0.1 (founder-only): returns FOMO_FOUNDER_PHONE_NUMBER for the
  //     founder's user_id; null for everyone else.
  //   v0.5.x (friend beta): also resolves friend user_ids via the phone
  //     allowlist store (decrypts users.phone_e164_encrypted on demand).
  //     Async because the store is async.
  //
  // The worker treats null as "refuse to dispatch" and surfaces
  // `fomo.send.unauthorized_destination` in audit — defense-in-depth
  // against a misconfig that would otherwise let the system text an
  // arbitrary number.
  readonly destinationFor: (user_id: string) => Promise<string | null> | string | null;

  // ID generator for invocation_id (per dispatch call). Defaults to a
  // counter-prefixed string; tests inject a deterministic one.
  readonly newInvocationId?: () => string;
  // Optional clock; defaults to Date.now.
  readonly now?: () => number;
  // Max alerts to process per user per cycle. Defaults to 5. Caps
  // founder spam if the system ever queues many approvals at once.
  readonly maxAlertsPerUser?: number;
}

export type OutboundSenderOutcome =
  | 'sent'
  | 'failed'
  | 'status_unknown'
  | 'unauthorized_destination'
  | 'preflight_skipped'
  // Phase 3F.1 — STOP enforcement refused the send. Alert transitions
  // approved → failed (reason: stop_enforced). NEVER calls SendBlue.
  | 'stop_enforced';

export interface OutboundSenderAlertResult {
  readonly user_id: string;
  readonly alert_id: string;
  readonly outcome: OutboundSenderOutcome;
  // Operator-facing diagnostic. Never the rendered message content.
  readonly reason: string;
}

export interface OutboundSenderUserOutcome {
  readonly user_id: string;
  readonly alerts_considered: number;
  readonly alerts_sent: number;
  readonly alerts_failed: number;
  readonly alerts_status_unknown: number;
  readonly alerts_unauthorized: number;
  readonly alerts_preflight_skipped: number;
  // Phase 3F.1 — count of alerts the STOP-enforcement check refused
  // to send. Per-user; rolls up to alerts_stop_enforced in the cycle
  // report. Useful smoke evidence: prove STOP blocks future sends.
  readonly alerts_stop_enforced: number;
}

export interface OutboundSenderCycleReport {
  readonly started_at: string;
  readonly finished_at: string;
  readonly users_total: number;
  readonly users_with_approved_alerts: number;
  readonly alerts_considered: number;
  readonly alerts_sent: number;
  readonly alerts_failed: number;
  readonly alerts_status_unknown: number;
  readonly alerts_unauthorized: number;
  readonly alerts_preflight_skipped: number;
  // Phase 3F.1 — alerts the STOP-enforcement check refused.
  readonly alerts_stop_enforced: number;
  readonly results: readonly OutboundSenderAlertResult[];
  readonly user_outcomes: readonly OutboundSenderUserOutcome[];
}

function defaultInvocationIdGenerator(): () => string {
  let n = 0;
  const seed = Math.random().toString(36).slice(2, 8);
  return () => `outbound-send-${seed}-${++n}`;
}

export async function runOutboundOnce(
  deps: OutboundSenderDeps
): Promise<OutboundSenderCycleReport> {
  const now = deps.now ?? Date.now;
  const newInvocationId = deps.newInvocationId ?? defaultInvocationIdGenerator();
  const maxAlertsPerUser = deps.maxAlertsPerUser ?? 5;

  const started_at = new Date(now()).toISOString();
  const results: OutboundSenderAlertResult[] = [];
  const userOutcomes: OutboundSenderUserOutcome[] = [];
  let users_with_approved_alerts = 0;
  let alerts_considered = 0;
  let alerts_sent = 0;
  let alerts_failed = 0;
  let alerts_status_unknown = 0;
  let alerts_unauthorized = 0;
  let alerts_preflight_skipped = 0;
  let alerts_stop_enforced = 0;

  const userIds = await deps.cursorStore.listUserIds();

  for (const user_id of userIds) {
    const approvedIds = await deps.transitions.findAlertIdsInState(
      user_id,
      'approved',
      maxAlertsPerUser
    );
    if (approvedIds.length === 0) {
      userOutcomes.push(
        Object.freeze({
          user_id,
          alerts_considered: 0,
          alerts_sent: 0,
          alerts_failed: 0,
          alerts_status_unknown: 0,
          alerts_unauthorized: 0,
          alerts_preflight_skipped: 0,
          alerts_stop_enforced: 0
        })
      );
      continue;
    }
    users_with_approved_alerts++;

    // Phase 3F.1 — STOP enforcement check, ONCE per user per cycle
    // BEFORE we attempt any send. Per founder directive 2026-05-26:
    // STOP enforcement is deterministic (no LLM). If stop_active is
    // true, refuse every approved alert this cycle and transition
    // each to `failed` (reason: stop_enforced). NEVER call SendBlue.
    // Per-user check is correct: STOP is a global user-level
    // compliance signal, not per-alert.
    const stopActive = await isStopActive(deps, user_id);

    let user_sent = 0;
    let user_failed = 0;
    let user_status_unknown = 0;
    let user_unauthorized = 0;
    let user_preflight_skipped = 0;
    let user_stop_enforced = 0;

    for (const alert_id of approvedIds) {
      alerts_considered++;
      if (stopActive) {
        const r = await processStopEnforced(deps, user_id, alert_id);
        results.push(r);
        user_stop_enforced++;
        alerts_stop_enforced++;
        continue;
      }
      const r = await processOneAlert(deps, user_id, alert_id, newInvocationId);
      results.push(r);
      switch (r.outcome) {
        case 'sent': user_sent++; alerts_sent++; break;
        case 'failed': user_failed++; alerts_failed++; break;
        case 'status_unknown': user_status_unknown++; alerts_status_unknown++; break;
        case 'unauthorized_destination': user_unauthorized++; alerts_unauthorized++; break;
        case 'preflight_skipped': user_preflight_skipped++; alerts_preflight_skipped++; break;
        case 'stop_enforced': user_stop_enforced++; alerts_stop_enforced++; break;
      }
    }

    userOutcomes.push(
      Object.freeze({
        user_id,
        alerts_considered: approvedIds.length,
        alerts_sent: user_sent,
        alerts_failed: user_failed,
        alerts_status_unknown: user_status_unknown,
        alerts_unauthorized: user_unauthorized,
        alerts_preflight_skipped: user_preflight_skipped,
        alerts_stop_enforced: user_stop_enforced
      })
    );
  }

  const finished_at = new Date(now()).toISOString();

  return Object.freeze({
    started_at,
    finished_at,
    users_total: userIds.length,
    users_with_approved_alerts,
    alerts_considered,
    alerts_sent,
    alerts_failed,
    alerts_status_unknown,
    alerts_unauthorized,
    alerts_preflight_skipped,
    alerts_stop_enforced,
    results: Object.freeze(results.map((r) => Object.freeze(r))),
    user_outcomes: Object.freeze(userOutcomes.map((o) => Object.freeze(o)))
  });
}

// Processes a single alert end-to-end. NEVER throws to the caller —
// every failure path is captured as an audit row + state transition.
// Phase 3F.1 — STOP enforcement helper. Returns true when the
// user has an active `stop_active` memory signal. Per founder
// directive 2026-05-26: deterministic, no LLM. The signal is
// flipped by the /sendblue/inbound route's deterministic pre-pass
// when the founder texts STOP / UNSUBSCRIBE / CANCEL. START flips
// it back. Idempotent.
async function isStopActive(
  deps: OutboundSenderDeps,
  user_id: string
): Promise<boolean> {
  const signal = await deps.memoryStore.get(user_id, 'stop_active', null);
  if (!signal) return false;
  const detail = signal.detail as { active?: unknown };
  return detail.active === true;
}

// Process an alert that the STOP-enforcement check refused. Audits
// fomo.send.stop_enforced + transitions approved → failed (reason:
// stop_enforced). NEVER calls SendBlue. Sanitized audit detail.
async function processStopEnforced(
  deps: OutboundSenderDeps,
  user_id: string,
  alert_id: string
): Promise<OutboundSenderAlertResult> {
  const alert = await deps.alertStore.get(alert_id);
  if (!alert) {
    // Defensive: findAlertIdsInState returned this id, so the row
    // should exist. If somehow it doesn't, treat as preflight skip.
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.stop_enforced',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { alert_id, error_code: 'unknown_alert_id' }
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'preflight_skipped' as const,
      reason: 'unknown_alert_id (during stop_enforced check)'
    });
  }

  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.send.stop_enforced',
    target: `alert:${alert_id}`,
    result: 'success',
    detail: {
      alert_id,
      message_id: alert.message_id,
      rank_result_id: alert.rank_result_id,
      // NOTE: no destination_slug here — STOP enforcement runs
      // BEFORE destinationFor is consulted. The audit captures only
      // the alert reference + the policy outcome.
      reason: 'stop_active memory_signal is true for this user'
    }
  });

  await writeTransition(deps, {
    alert_id,
    user_id,
    from_state: 'approved',
    to_state: 'failed',
    reason: 'stop_enforced: user has active STOP request (stop_active memory_signal=true)'
  });

  return Object.freeze({
    user_id,
    alert_id,
    outcome: 'stop_enforced' as const,
    reason: 'stop_active memory_signal=true; refused to send'
  });
}

async function processOneAlert(
  deps: OutboundSenderDeps,
  user_id: string,
  alert_id: string,
  newInvocationId: () => string
): Promise<OutboundSenderAlertResult> {
  // Re-verify the current state. findAlertIdsInState already filtered
  // to 'approved', but a tiny race window exists (another worker
  // restart between the find and this loop). Refuse to act on
  // anything other than 'approved'.
  const currentState = await deps.transitions.currentState(alert_id);
  if (currentState !== 'approved') {
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.attempted',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        error_code: 'precondition_failed',
        current_state: currentState ?? 'unknown'
      }
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'preflight_skipped' as const,
      reason: `precondition_failed: current_state=${currentState ?? 'unknown'}`
    });
  }

  // Load alert + rank_result.
  const alert = await deps.alertStore.get(alert_id);
  if (!alert) {
    // Should not happen if findAlertIdsInState returned this id, but
    // defense-in-depth.
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.attempted',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { alert_id, error_code: 'unknown_alert_id' }
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'preflight_skipped' as const,
      reason: 'unknown_alert_id'
    });
  }
  const rank = await deps.rankResultStore.get(user_id, alert.message_id);
  if (!rank) {
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.attempted',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: { alert_id, error_code: 'rank_result_missing' }
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'preflight_skipped' as const,
      reason: 'rank_result_missing'
    });
  }

  // Defense-in-depth: founder-phone allowlist. v0.5.x: destinationFor
  // may be async (friend phone lookup via the encrypted-allowlist store).
  const destination = await Promise.resolve(deps.destinationFor(user_id));
  if (!destination) {
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.unauthorized_destination',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        message_id: alert.message_id,
        error_code: 'no_destination_for_user'
      }
    });
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'failed',
      reason: `outbound-sender: no destination phone configured for user ${user_id}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'unauthorized_destination' as const,
      reason: 'no destination phone configured for user'
    });
  }

  // Re-read Gmail via dispatch to recover the egress view. This keeps
  // the privacy invariant intact: the worker NEVER stores body content
  // anywhere — it re-fetches via the existing gmail.read substrate,
  // applies egress, renders the template, then discards the raw view.
  const gmailRawOrSkip = await runGmailRead(deps, user_id, alert.message_id, newInvocationId());
  if (gmailRawOrSkip.kind === 'skip') {
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.attempted',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        message_id: alert.message_id,
        error_code: 'gmail_read_failed',
        reason: gmailRawOrSkip.reason
      }
    });
    // Treat as ambiguous — we have no idea if the message still exists.
    // Per the three-outcome rule, default to send_status_unknown rather
    // than failed, because failed implies a terminal we've confirmed.
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'send_status_unknown',
      reason: `gmail.read failed: ${gmailRawOrSkip.reason}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'status_unknown' as const,
      reason: `gmail.read failed: ${gmailRawOrSkip.reason}`
    });
  }

  const view: SlackEgressView = applyEgressForSlackCard(gmailRawOrSkip.raw);
  const rendered = renderFounderText({
    view,
    rank: { label: rank.label as RankLabel, score: rank.score }
  });

  // Audit the send attempt BEFORE invoking the provider. Even if the
  // process dies after this row, the operator sees that we tried.
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.send.attempted',
    target: `alert:${alert_id}`,
    result: 'success',
    detail: {
      alert_id,
      message_id: alert.message_id,
      rank_result_id: alert.rank_result_id,
      destination_slug: destinationSlug(destination),
      template_version: rendered.template_version,
      label: rank.label,
      score: rank.score,
      content_chars: rendered.text.length
    }
  });

  // Dispatch sendblue.send_user_message.
  const sendDecision = decidePolicy(
    { tool_id: 'sendblue.send_user_message', user_id, intent: 'manual_send' },
    deps.gateDeps
  );
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'policy.decided',
    target: 'tool:sendblue.send_user_message',
    result: 'success',
    detail: {
      tool_id: 'sendblue.send_user_message',
      decision_code: sendDecision.code,
      allowed: sendDecision.allowed,
      alert_id
    }
  });

  const sendAuthorized = AuthorizedToolCall.fromDecision(sendDecision);
  const sendInvocationId = `outbound-send-${alert_id}`;
  if (!sendAuthorized) {
    await deps.toolInvocationStore.write({
      user_id,
      tool_id: 'sendblue.send_user_message',
      invocation_id: sendInvocationId,
      policy_decision: sendDecision.code,
      status: 'denied',
      latency_ms: 0
    });
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'tool.invoked',
      target: 'tool:sendblue.send_user_message',
      result: 'failure',
      detail: {
        tool_id: 'sendblue.send_user_message',
        invocation_id: sendInvocationId,
        policy_decision: sendDecision.code,
        status: 'denied'
      }
    });
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.failed',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        error_code: 'gate_denied',
        decision_code: sendDecision.code
      }
    });
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'failed',
      reason: `outbound-sender gate denied: ${sendDecision.code}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'failed' as const,
      reason: `gate denied: ${sendDecision.code}`
    });
  }

  const sendResult = await deps.dispatch.execute<SendOutcome>(
    sendAuthorized,
    { to: destination, content: rendered.text },
    { user_id, invocation_id: sendInvocationId }
  );

  await deps.toolInvocationStore.write({
    user_id,
    tool_id: 'sendblue.send_user_message',
    invocation_id: sendInvocationId,
    policy_decision: sendDecision.code,
    status: sendResult.ok ? 'success' : 'failure',
    latency_ms: sendResult.latency_ms,
    error_code: sendResult.ok ? null : sendResult.code,
    error_reason: sendResult.ok ? null : sendResult.reason
  });
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'tool.invoked',
    target: 'tool:sendblue.send_user_message',
    result: sendResult.ok ? 'success' : 'failure',
    detail: {
      tool_id: 'sendblue.send_user_message',
      invocation_id: sendInvocationId,
      policy_decision: sendDecision.code,
      status: sendResult.ok ? 'success' : 'failure'
    }
  });

  if (!sendResult.ok) {
    // Executor threw / unauthorized. Treat as ambiguous: we don't know
    // if the request reached SendBlue or not.
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.status_unknown',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        error_code: sendResult.code,
        reason: sendResult.reason
      }
    });
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'send_status_unknown',
      reason: `dispatch error: ${sendResult.code} ${sendResult.reason}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'status_unknown' as const,
      reason: `dispatch error: ${sendResult.code}`
    });
  }

  // Translate the provider's outcome.kind into a state transition + audit.
  const outcome = sendResult.output;
  if (outcome.kind === 'sent') {
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.succeeded',
      target: `alert:${alert_id}`,
      result: 'success',
      detail: {
        alert_id,
        message_id: alert.message_id,
        rank_result_id: alert.rank_result_id,
        destination_slug: destinationSlug(destination),
        provider_status: outcome.providerStatus,
        provider_message_handle: outcome.providerMessageHandle,
        template_version: FOUNDER_TEXT_TEMPLATE_VERSION
      }
    });
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'sent',
      reason: `sendblue ok: provider_status=${outcome.providerStatus ?? '?'}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'sent' as const,
      reason: `provider_status=${outcome.providerStatus ?? '?'}`
    });
  }
  if (outcome.kind === 'failed') {
    // Phase 3G.1 item #2 — OPTED_OUT drift detection.
    //
    // SendBlue's carrier-level opt-out list and our local
    // memory_signals.stop_active can drift. The user texts STOP → our
    // /sendblue/inbound route records stop_active=true AND SendBlue
    // records carrier-level opt-out. A subsequent operation (a manual
    // SQL clear, a memory-store reset) can flip our local back to
    // active=false while SendBlue's spam-rule firewall keeps blocking.
    // When that drift happens, the next send returns HTTP 400 with
    // `error_message: "OPTED_OUT"`. We catch it here, re-write
    // stop_active=true with source='opt_out_drift_carrier', and emit
    // a named audit so the operator sees the drift loudly.
    //
    // The alert still transitions approved → failed below (same as any
    // other clear-failure 4xx) so the alert is not retried.
    if (outcome.providerError?.error_message === 'OPTED_OUT') {
      await deps.memoryStore.upsert({
        user_id,
        kind: 'stop_active',
        scope_key: null,
        detail: {
          active: true,
          recorded_at: new Date().toISOString(),
          drift_detected_via: 'sendblue_opted_out_response'
        },
        confidence: 1,
        source: 'opt_out_drift_carrier'
      });
      await deps.auditStore.write({
        actor_user_id: user_id,
        actor_ip: null,
        actor_user_agent: null,
        action: 'fomo.send.opt_out_drift_detected',
        target: `alert:${alert_id}`,
        result: 'success',
        detail: {
          alert_id,
          message_id: alert.message_id,
          rank_result_id: alert.rank_result_id,
          destination_slug: destinationSlug(destination),
          provider_error_message: outcome.providerError.error_message,
          provider_error_reason: outcome.providerError.error_reason,
          provider_error_code: outcome.providerError.error_code,
          stop_active_synced: true
        }
      });
    }

    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'fomo.send.failed',
      target: `alert:${alert_id}`,
      result: 'failure',
      detail: {
        alert_id,
        message_id: alert.message_id,
        rank_result_id: alert.rank_result_id,
        destination_slug: destinationSlug(destination),
        provider_status: outcome.providerStatus,
        http_status: outcome.httpStatus,
        reason: outcome.reason,
        // Phase 3G.1 item #2: surface the three named safe fields in
        // the failed-send detail so the operator sees the category
        // (e.g. OPTED_OUT) without parsing reason text. NEVER the raw
        // response body.
        provider_error_message: outcome.providerError?.error_message ?? null,
        provider_error_reason: outcome.providerError?.error_reason ?? null,
        provider_error_code: outcome.providerError?.error_code ?? null
      }
    });
    await writeTransition(deps, {
      alert_id,
      user_id,
      from_state: 'approved',
      to_state: 'failed',
      reason: `sendblue failed: ${outcome.reason}`
    });
    return Object.freeze({
      user_id,
      alert_id,
      outcome: 'failed' as const,
      reason: outcome.reason
    });
  }
  // status_unknown — ambiguous; caller must NOT auto-retry.
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'fomo.send.status_unknown',
    target: `alert:${alert_id}`,
    result: 'failure',
    detail: {
      alert_id,
      message_id: alert.message_id,
      rank_result_id: alert.rank_result_id,
      destination_slug: destinationSlug(destination),
      provider_status: outcome.providerStatus,
      http_status: outcome.httpStatus,
      reason: outcome.reason
    }
  });
  await writeTransition(deps, {
    alert_id,
    user_id,
    from_state: 'approved',
    to_state: 'send_status_unknown',
    reason: `sendblue ambiguous: ${outcome.reason}`
  });
  return Object.freeze({
    user_id,
    alert_id,
    outcome: 'status_unknown' as const,
    reason: outcome.reason
  });
}

/* ====================================================================== */
/* Helpers                                                                */
/* ====================================================================== */

type GmailReadResolution =
  | { readonly kind: 'ok'; readonly raw: RawEmailContext }
  | { readonly kind: 'skip'; readonly reason: string };

async function runGmailRead(
  deps: OutboundSenderDeps,
  user_id: string,
  message_id: string,
  invocation_id: string
): Promise<GmailReadResolution> {
  const decision = decidePolicy(
    { tool_id: 'gmail.read', user_id, intent: 'read' },
    deps.gateDeps
  );
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'policy.decided',
    target: 'tool:gmail.read',
    result: 'success',
    detail: {
      tool_id: 'gmail.read',
      decision_code: decision.code,
      allowed: decision.allowed
    }
  });
  const authorized = AuthorizedToolCall.fromDecision(decision);
  if (!authorized) {
    await deps.toolInvocationStore.write({
      user_id,
      tool_id: 'gmail.read',
      invocation_id,
      policy_decision: decision.code,
      status: 'denied',
      latency_ms: 0
    });
    await deps.auditStore.write({
      actor_user_id: user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'tool.invoked',
      target: 'tool:gmail.read',
      result: 'failure',
      detail: {
        tool_id: 'gmail.read',
        invocation_id,
        policy_decision: decision.code,
        status: 'denied'
      }
    });
    return { kind: 'skip', reason: `gate denied: ${decision.code}` };
  }
  const result = await deps.dispatch.execute<RawEmailContext>(
    authorized,
    { message_id },
    { user_id, invocation_id }
  );
  await deps.toolInvocationStore.write({
    user_id,
    tool_id: 'gmail.read',
    invocation_id,
    policy_decision: decision.code,
    status: result.ok ? 'success' : 'failure',
    latency_ms: result.latency_ms,
    error_code: result.ok ? null : result.code,
    error_reason: result.ok ? null : result.reason
  });
  await deps.auditStore.write({
    actor_user_id: user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'tool.invoked',
    target: 'tool:gmail.read',
    result: result.ok ? 'success' : 'failure',
    detail: {
      tool_id: 'gmail.read',
      invocation_id,
      policy_decision: decision.code,
      status: result.ok ? 'success' : 'failure'
    }
  });
  if (!result.ok) {
    return { kind: 'skip', reason: `${result.code}: ${result.reason}` };
  }
  return { kind: 'ok', raw: result.output };
}

// 4-char suffix of a destination phone, for audit traceability.
// '+14155551234' → '1234'. Never the full number.
function destinationSlug(destination: string): string {
  if (!destination) return '<unknown>';
  return destination.length <= 4 ? destination : destination.slice(-4);
}

async function writeTransition(
  deps: OutboundSenderDeps,
  input: {
    alert_id: string;
    user_id: string;
    from_state: Parameters<typeof transition>[0];
    to_state: Parameters<typeof transition>[1];
    reason: string;
  }
): Promise<void> {
  const validated = transition(input.from_state, input.to_state, input.reason);
  if ('error' in validated) {
    await deps.auditStore.write({
      actor_user_id: input.user_id,
      actor_ip: null,
      actor_user_agent: null,
      action: 'state.transitioned',
      target: `alert:${input.alert_id}`,
      result: 'failure',
      detail: {
        alert_id: input.alert_id,
        from_state: input.from_state,
        to_state: input.to_state,
        error_code: 'invalid_transition',
        reason: validated.reason
      }
    });
    return;
  }
  await deps.transitions.write({
    alert_id: input.alert_id,
    user_id: input.user_id,
    from_state: input.from_state,
    to_state: input.to_state,
    reason: input.reason
  });
  await deps.auditStore.write({
    actor_user_id: input.user_id,
    actor_ip: null,
    actor_user_agent: null,
    action: 'state.transitioned',
    target: `alert:${input.alert_id}`,
    result: 'success',
    detail: {
      alert_id: input.alert_id,
      from_state: input.from_state,
      to_state: input.to_state
    }
  });
}

/* ====================================================================== */
/* startOutboundSender — interval wrapper (mirrors gmail-poll's pattern)  */
/* ====================================================================== */

export interface OutboundPollingHandle {
  stop(): Promise<void>;
}

export interface StartOutboundOptions {
  readonly intervalMs: number;
  readonly onCycle?: (report: OutboundSenderCycleReport) => void;
  readonly onError?: (err: unknown) => void;
  // Optional cap on number of cycles before auto-stop. Mirrors
  // FOMO_GMAIL_POLLING_MAX_CYCLES — used by 3E.2 smoke tests so the
  // worker cannot keep firing.
  readonly maxCycles?: number;
}

export function startOutboundSender(
  deps: OutboundSenderDeps,
  opts: StartOutboundOptions
): OutboundPollingHandle {
  if (!Number.isInteger(opts.intervalMs) || opts.intervalMs <= 0) {
    throw new Error(
      `startOutboundSender: intervalMs must be a positive integer (got ${opts.intervalMs})`
    );
  }

  let stopped = false;
  let inflight: Promise<void> = Promise.resolve();
  let timer: ReturnType<typeof setTimeout> | null = null;
  let cyclesRun = 0;

  const tick = (): void => {
    if (stopped) return;
    inflight = (async () => {
      try {
        const report = await runOutboundOnce(deps);
        cyclesRun++;
        if (!stopped) opts.onCycle?.(report);
      } catch (err) {
        if (!stopped) opts.onError?.(err);
      }
    })();
    void inflight.finally(() => {
      if (stopped) return;
      if (opts.maxCycles !== undefined && cyclesRun >= opts.maxCycles) {
        stopped = true;
        return;
      }
      timer = setTimeout(tick, opts.intervalMs);
    });
  };

  tick();

  return {
    async stop() {
      stopped = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      await inflight.catch(() => undefined);
    }
  };
}

