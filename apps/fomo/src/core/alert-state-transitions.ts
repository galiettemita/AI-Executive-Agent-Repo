// Alert State Transitions — persistent log of every state move an alert
// makes through its lifecycle. The state machine in state-machine.ts is a
// pure function; this store is where transitions get committed alongside
// the alert_id and user_id they belong to.
//
// FOMO_PLAN §9.10 puts state transitions in the persistence skeleton.
// Phase 2C shipped the pure transition function; Phase 2E adds the store
// so Phase 3's workflow code can persist + read the lifecycle for any
// given alert. No caller yet — this is substrate.

import { type AlertState, isAlertState } from './state-machine.js';

export interface AlertStateTransitionRecord {
  readonly id?: number;
  readonly alert_id: string;
  readonly user_id: string;
  readonly from_state: AlertState;
  readonly to_state: AlertState;
  // ISO 8601.
  readonly at: string;
  // Operator-facing reason for the transition. Free-form string;
  // safe-logger redact() is NOT applied here because the reason is
  // operator-authored, not user-payload-derived. Callers are responsible
  // for writing reasons that do not carry private content (per
  // FOMO_DESIGN §17 — no email body in logs).
  readonly reason: string;
}

export interface AlertStateTransitionInput {
  alert_id: string;
  user_id: string;
  from_state: AlertState;
  to_state: AlertState;
  reason: string;
  at?: string;
}

export interface AlertStateTransitionStore {
  write(input: AlertStateTransitionInput): Promise<void>;
  forAlert(alertId: string): Promise<readonly AlertStateTransitionRecord[]>;
  recentForUser(userId: string, limit?: number): Promise<readonly AlertStateTransitionRecord[]>;
  // Returns the most recent to_state for this alert, or null if no
  // transitions have been recorded for it.
  currentState(alertId: string): Promise<AlertState | null>;
}

export class InMemoryAlertStateTransitionStore implements AlertStateTransitionStore {
  private records: AlertStateTransitionRecord[] = [];
  private nextId = 1;
  private readonly capacity: number;

  constructor(capacity = 50_000) {
    this.capacity = capacity;
  }

  async write(input: AlertStateTransitionInput): Promise<void> {
    // Defensive validation — the gate against an in-memory corruption that
    // would let an unknown state slip into the log.
    if (!isAlertState(input.from_state)) {
      throw new Error(`AlertStateTransitionStore: unknown from_state '${input.from_state}'`);
    }
    if (!isAlertState(input.to_state)) {
      throw new Error(`AlertStateTransitionStore: unknown to_state '${input.to_state}'`);
    }
    this.records.push(
      Object.freeze({
        id: this.nextId++,
        alert_id: input.alert_id,
        user_id: input.user_id,
        from_state: input.from_state,
        to_state: input.to_state,
        at: input.at ?? new Date().toISOString(),
        reason: input.reason
      })
    );
    if (this.records.length > this.capacity) {
      this.records.splice(0, this.records.length - this.capacity);
    }
  }

  async forAlert(alertId: string): Promise<readonly AlertStateTransitionRecord[]> {
    return this.records.filter((r) => r.alert_id === alertId);
  }

  async recentForUser(userId: string, limit = 100): Promise<readonly AlertStateTransitionRecord[]> {
    const filtered = this.records.filter((r) => r.user_id === userId);
    return filtered.slice(-limit).reverse();
  }

  async currentState(alertId: string): Promise<AlertState | null> {
    const transitions = this.records.filter((r) => r.alert_id === alertId);
    if (transitions.length === 0) return null;
    return transitions[transitions.length - 1]!.to_state;
  }
}
