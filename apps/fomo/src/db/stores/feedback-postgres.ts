// Postgres-backed FeedbackStore. Same contract as InMemoryFeedbackStore
// from Phase 2C. Detail is redacted via safe-logger.redact on write.

import { and, count, desc, eq } from 'drizzle-orm';

import { redact } from '../../core/safe-logger.js';
import {
  type FeedbackEvent,
  type FeedbackEventInput,
  type FeedbackEventKind,
  type FeedbackStore
} from '../../memory/feedback-events.js';
import { type DrizzleClient } from '../client.js';
import { feedback_events } from '../schema.js';

export class PostgresFeedbackStore implements FeedbackStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(input: FeedbackEventInput): Promise<void> {
    const detail = input.detail ? (redact(input.detail) as Record<string, unknown>) : null;
    const values: typeof feedback_events.$inferInsert = {
      user_id: input.user_id,
      alert_id: input.alert_id,
      sender_email: input.sender_email,
      kind: input.kind,
      detail
    };
    if (input.occurred_at !== undefined) {
      values.occurred_at = new Date(input.occurred_at);
    }
    await this.db.insert(feedback_events).values(values);
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
        kind: r.kind as FeedbackEventKind,
        detail: r.detail as Record<string, unknown> | null
      })
    );
  }

  async countByKind(userId: string, kind: FeedbackEventKind): Promise<number> {
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
