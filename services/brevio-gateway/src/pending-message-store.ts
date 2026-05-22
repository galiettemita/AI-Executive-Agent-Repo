// PendingMessageStore — tracks user messages mid-OAuth so the simulator/frontend
// can re-dispatch the original chat after token arrival without re-typing.
//
// Atomic consume (returns row once, then null) prevents double-redemption races
// when the user has multiple tabs open during OAuth. TTL 10 min; older rows
// pruned periodically by callers.

const TTL_MS = 10 * 60 * 1000;

export interface PendingMessage {
  pending_message_id: string;
  user_id: string;
  original_text: string;
  channel: string | null;
  session_id: string | null;
  created_at: number;
}

export interface PendingMessageStore {
  put(input: Omit<PendingMessage, 'created_at'>): Promise<void>;
  peek(pendingId: string, userId: string): Promise<PendingMessage | null>;
  consume(pendingId: string, userId: string): Promise<PendingMessage | null>;
  prune(now?: number): Promise<number>;
}

interface InternalRow extends PendingMessage {
  consumed: boolean;
}

export class InMemoryPendingMessageStore implements PendingMessageStore {
  private readonly rows = new Map<string, InternalRow>();

  async put(input: Omit<PendingMessage, 'created_at'>): Promise<void> {
    this.rows.set(input.pending_message_id, {
      ...input,
      created_at: Date.now(),
      consumed: false
    });
  }

  async peek(pendingId: string, userId: string): Promise<PendingMessage | null> {
    const row = this.rows.get(pendingId);
    if (!row) return null;
    if (row.user_id !== userId) return null;
    if (row.consumed) return null;
    if (Date.now() - row.created_at > TTL_MS) return null;
    return {
      pending_message_id: row.pending_message_id,
      user_id: row.user_id,
      original_text: row.original_text,
      channel: row.channel,
      session_id: row.session_id,
      created_at: row.created_at
    };
  }

  async consume(pendingId: string, userId: string): Promise<PendingMessage | null> {
    const row = this.rows.get(pendingId);
    if (!row) return null;
    if (row.user_id !== userId) return null;
    if (row.consumed) return null;
    if (Date.now() - row.created_at > TTL_MS) {
      this.rows.delete(pendingId);
      return null;
    }
    row.consumed = true;
    return {
      pending_message_id: row.pending_message_id,
      user_id: row.user_id,
      original_text: row.original_text,
      channel: row.channel,
      session_id: row.session_id,
      created_at: row.created_at
    };
  }

  async prune(now: number = Date.now()): Promise<number> {
    let pruned = 0;
    for (const [k, v] of this.rows) {
      if (now - v.created_at > TTL_MS) {
        this.rows.delete(k);
        pruned++;
      }
    }
    return pruned;
  }
}

export const PENDING_MESSAGE_TTL_MS = TTL_MS;
