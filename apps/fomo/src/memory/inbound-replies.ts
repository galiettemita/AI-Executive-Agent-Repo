// Inbound Replies Store — one row per inbound SendBlue webhook the
// /sendblue/inbound route has accepted as authentic. Phase 3F.1.
//
// LOAD-BEARING idempotency: SendBlue retries on non-2xx. The route
// MUST be safe to receive the same payload N times without
// double-writing feedback events, double-flipping memory signals
// (especially stop_active), or double-firing state transitions.
//
// Identity: provider_message_id (SendBlue's unique id for the
// inbound message). UNIQUE on this column. Caller calls
// `record(input)` which performs INSERT ON CONFLICT DO NOTHING and
// returns { inserted: boolean }. On `inserted: false` the caller
// audits `fomo.sendblue.reply_duplicate` and returns 200 OK
// immediately.
//
// Privacy invariants — same as alerts:
//   * NO raw webhook payload
//   * NO founder reply text
//   * NO full from-phone (route audit logs a 4-char from_slug suffix
//     elsewhere; this table has only operational identifiers)
//   * The processing outcome (intent, state transition, memory
//     signal flip) lives in audit_log + feedback_events + memory_signals
//     + alert_state_transitions — this table is purely the dedup gate.

export interface InboundReplyRecord {
  readonly id?: number;
  readonly provider_message_id: string;
  readonly user_id: string;
  // ISO 8601 — DB-side default now() at write time.
  readonly received_at: string;
}

export interface InboundReplyInput {
  readonly provider_message_id: string;
  readonly user_id: string;
}

export interface InboundReplyWriteOutcome {
  // True when the row was newly inserted (first time we've seen this
  // provider_message_id). False when the UNIQUE constraint hit on
  // provider_message_id (SendBlue retry — caller must NOT re-process).
  readonly inserted: boolean;
  // The full record. Always populated.
  readonly record: InboundReplyRecord;
}

export interface InboundReplyStore {
  // ON CONFLICT (provider_message_id) DO NOTHING. Returns
  // { inserted: false, record } when conflict; { inserted: true,
  // record } on first insert.
  record(input: InboundReplyInput): Promise<InboundReplyWriteOutcome>;
  // Returns the row for a provider_message_id, or null.
  getByProviderMessageId(providerMessageId: string): Promise<InboundReplyRecord | null>;
  count(userId: string): Promise<number>;
  recent(userId: string, limit: number): Promise<readonly InboundReplyRecord[]>;
}

export class InMemoryInboundReplyStore implements InboundReplyStore {
  private readonly rows: InboundReplyRecord[] = [];
  private nextId = 1;

  async record(input: InboundReplyInput): Promise<InboundReplyWriteOutcome> {
    const existing = this.rows.find(
      (r) => r.provider_message_id === input.provider_message_id
    );
    if (existing) {
      return Object.freeze({ inserted: false, record: existing });
    }
    const record: InboundReplyRecord = Object.freeze({
      id: this.nextId++,
      provider_message_id: input.provider_message_id,
      user_id: input.user_id,
      received_at: new Date().toISOString()
    });
    this.rows.push(record);
    return Object.freeze({ inserted: true, record });
  }

  async getByProviderMessageId(providerMessageId: string): Promise<InboundReplyRecord | null> {
    return this.rows.find((r) => r.provider_message_id === providerMessageId) ?? null;
  }

  async count(userId: string): Promise<number> {
    return this.rows.filter((r) => r.user_id === userId).length;
  }

  async recent(userId: string, limit: number): Promise<readonly InboundReplyRecord[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const userRows = this.rows.filter((r) => r.user_id === userId);
    userRows.sort((a, b) => b.received_at.localeCompare(a.received_at));
    return Object.freeze(userRows.slice(0, limit));
  }
}
