// Postgres-backed FeedbackStore. Same contract as InMemoryFeedbackStore.
// Phase v0.5.9 — adds source_surface gate (write-time validation against
// BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES) and returns
// the written event (with id) for the applyFeedback consumer pipeline.

import { and, count, desc, eq } from 'drizzle-orm';

import { redact } from '../../core/safe-logger.js';
import {
  type BrevioFeedbackEventKind,
  type BrevioFeedbackSurface,
  type FeedbackEvent,
  type FeedbackEventInput,
  type FeedbackEventKind,
  type FeedbackStore,
  resolveAndGateSourceSurface
} from '../../memory/feedback-events.js';
import { type DrizzleClient } from '../client.js';
import { feedback_events } from '../schema.js';

export class PostgresFeedbackStore implements FeedbackStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(input: FeedbackEventInput): Promise<FeedbackEvent> {
    // Throws BrevioFeedbackError on rejection. Caller catches and emits the
    // sanitized failure audit; no row is written in that case.
    const source_surface = resolveAndGateSourceSurface(input);
    const detail = input.detail ? (redact(input.detail) as Record<string, unknown>) : null;
    const values: typeof feedback_events.$inferInsert = {
      user_id: input.user_id,
      alert_id: input.alert_id,
      sender_email: input.sender_email,
      kind: input.kind,
      source_surface,
      detail
    };
    if (input.occurred_at !== undefined) {
      values.occurred_at = new Date(input.occurred_at);
    }
    const inserted = await this.db
      .insert(feedback_events)
      .values(values)
      .returning({
        id: feedback_events.id,
        occurred_at: feedback_events.occurred_at,
        user_id: feedback_events.user_id,
        alert_id: feedback_events.alert_id,
        sender_email: feedback_events.sender_email,
        kind: feedback_events.kind,
        source_surface: feedback_events.source_surface,
        detail: feedback_events.detail
      });
    const row = inserted[0];
    if (!row) {
      // Defense-in-depth — Postgres RETURNING should always yield a row on
      // a successful INSERT. If this fires, the migration is missing or the
      // DB is in an inconsistent state; surface the failure rather than
      // silently returning a stale event.
      throw new Error('PostgresFeedbackStore.write: INSERT ... RETURNING returned no row');
    }
    return Object.freeze({
      id: row.id,
      occurred_at: row.occurred_at.toISOString(),
      user_id: row.user_id,
      alert_id: row.alert_id,
      sender_email: row.sender_email,
      kind: row.kind,
      // The drizzle column type is string at the schema level; the runtime
      // gate above guarantees the value is a BrevioFeedbackSurface — assert
      // the narrowed type here so callers don't need to re-validate.
      source_surface: row.source_surface as BrevioFeedbackSurface,
      detail: row.detail as Record<string, unknown> | null
    });
  }

  async recent(userId: string, limit = 100): Promise<readonly FeedbackEvent[]> {
    const rows = await this.db
      .select()
      .from(feedback_events)
      .where(eq(feedback_events.user_id, userId))
      .orderBy(desc(feedback_events.occurred_at))
      .limit(limit);
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        occurred_at: r.occurred_at.toISOString(),
        user_id: r.user_id,
        alert_id: r.alert_id,
        sender_email: r.sender_email,
        kind: r.kind,
        source_surface: r.source_surface as BrevioFeedbackSurface,
        detail: r.detail as Record<string, unknown> | null
      })
    );
  }

  async countByKind(userId: string, kind: FeedbackEventKind | BrevioFeedbackEventKind): Promise<number> {
    const rows = await this.db
      .select({ n: count() })
      .from(feedback_events)
      .where(and(eq(feedback_events.user_id, userId), eq(feedback_events.kind, kind)));
    return Number(rows[0]?.n ?? 0);
  }

  async countBySender(userId: string, senderEmail: string): Promise<number> {
    const rows = await this.db
      .select({ n: count() })
      .from(feedback_events)
      .where(and(eq(feedback_events.user_id, userId), eq(feedback_events.sender_email, senderEmail)));
    return Number(rows[0]?.n ?? 0);
  }
}
