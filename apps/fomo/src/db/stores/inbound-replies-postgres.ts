// Postgres-backed InboundReplyStore. Same contract as
// InMemoryInboundReplyStore. record() uses ON CONFLICT
// (provider_message_id) DO NOTHING — the load-bearing idempotency
// gate for the /sendblue/inbound route against SendBlue retries.

import { desc, eq, sql } from 'drizzle-orm';

import {
  type InboundReplyInput,
  type InboundReplyRecord,
  type InboundReplyStore,
  type InboundReplyWriteOutcome
} from '../../memory/inbound-replies.js';
import { type DrizzleClient } from '../client.js';
import { inbound_replies } from '../schema.js';

function rowToRecord(r: typeof inbound_replies.$inferSelect): InboundReplyRecord {
  return Object.freeze({
    id: r.id,
    provider_message_id: r.provider_message_id,
    user_id: r.user_id,
    received_at: r.received_at.toISOString()
  });
}

export class PostgresInboundReplyStore implements InboundReplyStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async record(input: InboundReplyInput): Promise<InboundReplyWriteOutcome> {
    const returned = await this.db
      .insert(inbound_replies)
      .values({
        provider_message_id: input.provider_message_id,
        user_id: input.user_id
      })
      .onConflictDoNothing({ target: inbound_replies.provider_message_id })
      .returning();
    if (returned.length > 0) {
      return Object.freeze({ inserted: true, record: rowToRecord(returned[0]!) });
    }
    // Conflict: fetch the existing row so the caller has the canonical
    // received_at (the original first-write timestamp, not now).
    const existing = await this.getByProviderMessageId(input.provider_message_id);
    if (!existing) {
      throw new Error(
        `PostgresInboundReplyStore.record: ON CONFLICT fired for provider_message_id=${input.provider_message_id} but the existing row could not be re-fetched`
      );
    }
    return Object.freeze({ inserted: false, record: existing });
  }

  async getByProviderMessageId(providerMessageId: string): Promise<InboundReplyRecord | null> {
    const rows = await this.db
      .select()
      .from(inbound_replies)
      .where(eq(inbound_replies.provider_message_id, providerMessageId))
      .limit(1);
    const r = rows[0];
    return r ? rowToRecord(r) : null;
  }

  async count(userId: string): Promise<number> {
    const rows = await this.db
      .select({ n: sql<number>`count(*)::int` })
      .from(inbound_replies)
      .where(eq(inbound_replies.user_id, userId));
    return rows[0]?.n ?? 0;
  }

  async recent(userId: string, limit: number): Promise<readonly InboundReplyRecord[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const rows = await this.db
      .select()
      .from(inbound_replies)
      .where(eq(inbound_replies.user_id, userId))
      .orderBy(desc(inbound_replies.received_at), desc(inbound_replies.id))
      .limit(limit);
    return Object.freeze(rows.map(rowToRecord));
  }
}
