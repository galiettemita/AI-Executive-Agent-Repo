// Postgres-backed AlertStateTransitionStore. Same contract as
// InMemoryAlertStateTransitionStore from this phase.

import { asc, desc, eq } from 'drizzle-orm';

import {
  type AlertStateTransitionInput,
  type AlertStateTransitionRecord,
  type AlertStateTransitionStore
} from '../../core/alert-state-transitions.js';
import { type AlertState, isAlertState } from '../../core/state-machine.js';
import { type DrizzleClient } from '../client.js';
import { alert_state_transitions } from '../schema.js';

export class PostgresAlertStateTransitionStore implements AlertStateTransitionStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(input: AlertStateTransitionInput): Promise<void> {
    if (!isAlertState(input.from_state)) {
      throw new Error(`AlertStateTransitionStore: unknown from_state '${input.from_state}'`);
    }
    if (!isAlertState(input.to_state)) {
      throw new Error(`AlertStateTransitionStore: unknown to_state '${input.to_state}'`);
    }
    const values: typeof alert_state_transitions.$inferInsert = {
      alert_id: input.alert_id,
      user_id: input.user_id,
      from_state: input.from_state,
      to_state: input.to_state,
      reason: input.reason
    };
    if (input.at !== undefined) {
      values.at = new Date(input.at);
    }
    await this.db.insert(alert_state_transitions).values(values);
  }

  async forAlert(alertId: string): Promise<readonly AlertStateTransitionRecord[]> {
    const rows = await this.db
      .select()
      .from(alert_state_transitions)
      .where(eq(alert_state_transitions.alert_id, alertId))
      .orderBy(asc(alert_state_transitions.id));
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        alert_id: r.alert_id,
        user_id: r.user_id,
        from_state: r.from_state as AlertState,
        to_state: r.to_state as AlertState,
        at: r.at.toISOString(),
        reason: r.reason
      })
    );
  }

  async recentForUser(userId: string, limit = 100): Promise<readonly AlertStateTransitionRecord[]> {
    const rows = await this.db
      .select()
      .from(alert_state_transitions)
      .where(eq(alert_state_transitions.user_id, userId))
      .orderBy(desc(alert_state_transitions.id))
      .limit(limit);
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        alert_id: r.alert_id,
        user_id: r.user_id,
        from_state: r.from_state as AlertState,
        to_state: r.to_state as AlertState,
        at: r.at.toISOString(),
        reason: r.reason
      })
    );
  }

  async currentState(alertId: string): Promise<AlertState | null> {
    const rows = await this.db
      .select({ to_state: alert_state_transitions.to_state })
      .from(alert_state_transitions)
      .where(eq(alert_state_transitions.alert_id, alertId))
      .orderBy(desc(alert_state_transitions.id))
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return r.to_state as AlertState;
  }

  async findAlertIdsInState(
    userId: string,
    state: AlertState,
    limit: number
  ): Promise<readonly string[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    // DISTINCT ON (alert_id) ORDER BY alert_id, id DESC picks the most
    // recent transition per alert. We then filter by to_state in an
    // outer query and order oldest-first by the inner row's id so the
    // worker dispatches in queue order.
    const sub = this.db
      .selectDistinctOn([alert_state_transitions.alert_id], {
        alert_id: alert_state_transitions.alert_id,
        to_state: alert_state_transitions.to_state,
        id: alert_state_transitions.id,
        at: alert_state_transitions.at
      })
      .from(alert_state_transitions)
      .where(eq(alert_state_transitions.user_id, userId))
      .orderBy(
        asc(alert_state_transitions.alert_id),
        desc(alert_state_transitions.id)
      )
      .as('latest');
    const rows = await this.db
      .select({ alert_id: sub.alert_id })
      .from(sub)
      .where(eq(sub.to_state, state))
      .orderBy(asc(sub.id))
      .limit(limit);
    return Object.freeze(rows.map((r) => r.alert_id));
  }
}
