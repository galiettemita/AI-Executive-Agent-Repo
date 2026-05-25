// Postgres-backed AlertStore. Same contract as InMemoryAlertStore.
// create() uses ON CONFLICT (rank_result_id) DO NOTHING.

import { desc, eq, sql } from 'drizzle-orm';

import {
  type Alert,
  type AlertInput,
  type AlertLabel,
  type AlertStore,
  type AlertWriteOutcome
} from '../../memory/alerts.js';
import { type DrizzleClient } from '../client.js';
import { alerts } from '../schema.js';

function rowToAlert(r: typeof alerts.$inferSelect): Alert {
  return Object.freeze({
    alert_id: r.alert_id,
    user_id: r.user_id,
    message_id: r.message_id,
    rank_result_id: r.rank_result_id,
    label: r.label as AlertLabel,
    score: r.score,
    created_at: r.created_at.toISOString()
  });
}

export class PostgresAlertStore implements AlertStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async create(input: AlertInput): Promise<AlertWriteOutcome> {
    const returned = await this.db
      .insert(alerts)
      .values({
        alert_id: input.alert_id,
        user_id: input.user_id,
        message_id: input.message_id,
        rank_result_id: input.rank_result_id,
        label: input.label,
        score: input.score
      })
      .onConflictDoNothing({ target: alerts.rank_result_id })
      .returning();
    if (returned.length > 0) {
      const row = returned[0]!;
      return Object.freeze({ inserted: true, alert: rowToAlert(row) });
    }
    // Conflict: fetch the existing row so the caller has the canonical
    // alert_id (which differs from input.alert_id by definition).
    const existing = await this.getByRankResult(input.rank_result_id);
    if (!existing) {
      // Race condition: the conflicting row was deleted between insert
      // and select. v0.1 has no delete path for alerts, so treat as fatal.
      throw new Error(
        `PostgresAlertStore.create: ON CONFLICT fired for rank_result_id=${input.rank_result_id} but the existing row could not be re-fetched`
      );
    }
    return Object.freeze({ inserted: false, alert: existing });
  }

  async get(alertId: string): Promise<Alert | null> {
    const rows = await this.db
      .select()
      .from(alerts)
      .where(eq(alerts.alert_id, alertId))
      .limit(1);
    const r = rows[0];
    return r ? rowToAlert(r) : null;
  }

  async getByRankResult(rankResultId: number): Promise<Alert | null> {
    const rows = await this.db
      .select()
      .from(alerts)
      .where(eq(alerts.rank_result_id, rankResultId))
      .limit(1);
    const r = rows[0];
    return r ? rowToAlert(r) : null;
  }

  async count(userId: string): Promise<number> {
    const rows = await this.db
      .select({ n: sql<number>`count(*)::int` })
      .from(alerts)
      .where(eq(alerts.user_id, userId));
    return rows[0]?.n ?? 0;
  }

  async recent(userId: string, limit: number): Promise<readonly Alert[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const rows = await this.db
      .select()
      .from(alerts)
      .where(eq(alerts.user_id, userId))
      .orderBy(desc(alerts.created_at), desc(alerts.alert_id))
      .limit(limit);
    return Object.freeze(rows.map(rowToAlert));
  }
}
