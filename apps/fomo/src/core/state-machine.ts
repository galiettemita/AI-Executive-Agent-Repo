// Alert State Machine — the v0.1 lifecycle of a single FOMO alert.
//
// FOMO_PLAN §9.5 lists the lifecycle states. This module ships them plus an
// explicit allowed-transitions graph + pure transition function. Phase 2C is
// substrate only — no persistence, no caller. Phase 3 will wire it into the
// Gmail polling worker, the ranker, the founder-review handler, the SendBlue
// send path, and the reply parser.
//
// The shape is intentionally a function from (from, to, reason) → either a
// recorded transition record or an invalid_transition error. No mutation, no
// side effects. State is stored externally (Phase 3) and the machine validates
// each proposed move.

export const ALERT_STATES = [
  'detected',
  'ranked',
  'gated_out',
  'queued_for_review',
  'rejected',
  'approved',
  'sent',
  'send_status_unknown',
  'failed',
  'replied',
  'snoozed',
  'ignored'
] as const;

export type AlertState = (typeof ALERT_STATES)[number];

export const INITIAL_STATE: AlertState = 'detected';

// Allowed transitions. Empty array = terminal.
//
// Notes on the graph:
//   * Any in-progress state can fall to 'failed' on an unexpected error.
//   * 'approved' can short-circuit to 'send_status_unknown' if SendBlue
//     responded 2xx but our local state write failed before recording 'sent'.
//   * 'replied' is a transient classification step — after the reply parser
//     runs, the alert resolves to 'snoozed' (later/tomorrow intents) or
//     'ignored' (ignore / ignore_sender intents). Other reply intents (open,
//     why, clarify) record feedback events but do not change alert state.
//     For terminal accounting they stay at 'replied' until a snooze refire
//     or timeout flips them — for v0.1 we treat 'replied' as effectively
//     terminal once feedback is recorded.
//   * Terminals (no outgoing transitions): gated_out, rejected, failed,
//     snoozed, ignored.
const TRANSITIONS: Readonly<Record<AlertState, readonly AlertState[]>> = Object.freeze({
  detected: Object.freeze(['ranked', 'failed'] as const),
  ranked: Object.freeze(['queued_for_review', 'gated_out', 'failed'] as const),
  queued_for_review: Object.freeze(['approved', 'rejected', 'failed'] as const),
  approved: Object.freeze(['sent', 'send_status_unknown', 'failed'] as const),
  sent: Object.freeze(['replied', 'ignored', 'send_status_unknown'] as const),
  send_status_unknown: Object.freeze(['sent', 'failed'] as const),
  replied: Object.freeze(['snoozed', 'ignored'] as const),
  gated_out: Object.freeze([] as const),
  rejected: Object.freeze([] as const),
  failed: Object.freeze([] as const),
  snoozed: Object.freeze([] as const),
  ignored: Object.freeze([] as const)
});

export function isAlertState(value: unknown): value is AlertState {
  return typeof value === 'string' && (ALERT_STATES as readonly string[]).includes(value);
}

export function isValidTransition(from: AlertState, to: AlertState): boolean {
  return TRANSITIONS[from].includes(to);
}

export function isTerminal(state: AlertState): boolean {
  return TRANSITIONS[state].length === 0;
}

export function getAllowedTransitions(from: AlertState): readonly AlertState[] {
  return TRANSITIONS[from];
}

export function getTerminalStates(): readonly AlertState[] {
  return ALERT_STATES.filter(isTerminal);
}

export interface AlertStateTransition {
  readonly from: AlertState;
  readonly to: AlertState;
  // ISO 8601 timestamp.
  readonly at: string;
  // Operator-facing reason (for audit log / state_transitions table).
  readonly reason: string;
}

export interface InvalidTransition {
  readonly error: 'invalid_transition';
  readonly from: AlertState;
  readonly to: AlertState;
  readonly reason: string;
}

export function isInvalidTransition(
  value: AlertStateTransition | InvalidTransition
): value is InvalidTransition {
  return 'error' in value;
}

export function transition(
  from: AlertState,
  to: AlertState,
  reason: string,
  now: Date = new Date()
): AlertStateTransition | InvalidTransition {
  if (!isValidTransition(from, to)) {
    const allowed = getAllowedTransitions(from);
    return {
      error: 'invalid_transition',
      from,
      to,
      reason:
        allowed.length === 0
          ? `${from} is terminal; no transitions allowed (rejected: → ${to})`
          : `${from} → ${to} is not a valid transition; allowed: [${allowed.join(', ')}]`
    };
  }
  return Object.freeze({
    from,
    to,
    at: now.toISOString(),
    reason
  });
}
