// Gmail Cursor Store — per-user Gmail history_id cursor for incremental
// polling. FOMO_PLAN §13 lists gmail_cursors as a workflow table; Phase
// 3B.1 adds it because the OAuth callback initializes a cursor (sets it
// to the user's current history_id at connect time) and Phase 3B.2's
// polling worker will read + advance it.
//
// Identity: user_id is the primary key. Each user has exactly one Gmail
// cursor. `history_id` is opaque (Gmail-supplied uint64; we store as
// string for safety against JS number precision).

export interface GmailCursor {
  readonly user_id: string;
  readonly history_id: string;
  // ISO 8601 — when the cursor was initialized or last advanced.
  readonly updated_at: string;
}

export interface GmailCursorInput {
  user_id: string;
  history_id: string;
  updated_at?: string;
}

export interface GmailCursorStore {
  // Idempotent upsert. Use after OAuth connect (initialize) and after
  // each poll cycle (advance).
  upsert(input: GmailCursorInput): Promise<void>;
  get(userId: string): Promise<GmailCursor | null>;
  // Returns true when a cursor existed and was removed; false otherwise.
  // Used by /me/disconnect flows in later phases.
  delete(userId: string): Promise<boolean>;
  // All user_ids with a stored Gmail cursor. The Phase 3B.2 polling
  // worker iterates this set: a cursor row is always created at OAuth
  // connect time, so cursor presence is the canonical signal of "user
  // has an active Gmail connection." Order is unspecified.
  listUserIds(): Promise<readonly string[]>;
}

export class InMemoryGmailCursorStore implements GmailCursorStore {
  private readonly cursors = new Map<string, GmailCursor>();

  async upsert(input: GmailCursorInput): Promise<void> {
    this.cursors.set(
      input.user_id,
      Object.freeze({
        user_id: input.user_id,
        history_id: input.history_id,
        updated_at: input.updated_at ?? new Date().toISOString()
      })
    );
  }

  async get(userId: string): Promise<GmailCursor | null> {
    return this.cursors.get(userId) ?? null;
  }

  async delete(userId: string): Promise<boolean> {
    return this.cursors.delete(userId);
  }

  async listUserIds(): Promise<readonly string[]> {
    return Object.freeze([...this.cursors.keys()]);
  }
}
