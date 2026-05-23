// Postgres-backed GmailCursorStore. Same contract as
// InMemoryGmailCursorStore. Upsert uses ON CONFLICT (user_id).

import { eq } from 'drizzle-orm';

import {
  type GmailCursor,
  type GmailCursorInput,
  type GmailCursorStore
} from '../../memory/gmail-cursors.js';
import { type DrizzleClient } from '../client.js';
import { gmail_cursors } from '../schema.js';

export class PostgresGmailCursorStore implements GmailCursorStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async upsert(input: GmailCursorInput): Promise<void> {
    const values: typeof gmail_cursors.$inferInsert = {
      user_id: input.user_id,
      history_id: input.history_id
    };
    if (input.updated_at !== undefined) {
      values.updated_at = new Date(input.updated_at);
    }
    await this.db
      .insert(gmail_cursors)
      .values(values)
      .onConflictDoUpdate({
        target: gmail_cursors.user_id,
        set: {
          history_id: input.history_id,
          updated_at: values.updated_at ?? new Date()
        }
      });
  }

  async get(userId: string): Promise<GmailCursor | null> {
    const rows = await this.db
      .select()
      .from(gmail_cursors)
      .where(eq(gmail_cursors.user_id, userId))
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return Object.freeze({
      user_id: r.user_id,
      history_id: r.history_id,
      updated_at: r.updated_at.toISOString()
    });
  }

  async delete(userId: string): Promise<boolean> {
    const result = await this.db
      .delete(gmail_cursors)
      .where(eq(gmail_cursors.user_id, userId))
      .returning({ id: gmail_cursors.user_id });
    return result.length > 0;
  }
}
