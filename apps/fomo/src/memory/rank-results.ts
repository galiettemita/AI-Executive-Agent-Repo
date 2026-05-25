// Rank Result Store — one row per successful ranker invocation.
//
// Phase 3C.3 adds this store as the caller for the rank_results table
// declared in schema.ts. The Gmail polling worker writes one row after
// each RankerSuccess; RankerFailures stay out of this table (audit-only)
// so the table reads as "successful ranks the founder review can act on."
//
// Idempotency: unique on (user_id, message_id). The write path is
// ON CONFLICT DO NOTHING; the returned flag tells the caller whether
// the row was newly inserted. The worker uses the flag to audit
// `fomo.rank.already_ranked` instead of double-charging model credits
// when Gmail history replays a message_id.
//
// Privacy: this store NEVER accepts body content, headers, or
// attachment filenames. The fields here are the model's DECISION
// (label, score, reason ≤240 chars) and operational metadata
// (model_name, prompt_version, tokens, cost, latency). Callers that
// pass anything else are misusing the abstraction; the field shape
// itself is the guard.

export type RankLabel = 'important' | 'not_important';

export interface RankResult {
  readonly id: number;
  readonly user_id: string;
  readonly message_id: string;
  readonly invocation_id: string;
  readonly model_name: string;
  readonly prompt_version: string;
  readonly label: RankLabel;
  readonly score: number;
  readonly reason: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
  // ISO 8601 — DB-side default now() at write time.
  readonly created_at: string;
}

export interface RankResultInput {
  readonly user_id: string;
  readonly message_id: string;
  readonly invocation_id: string;
  readonly model_name: string;
  readonly prompt_version: string;
  readonly label: RankLabel;
  readonly score: number;
  readonly reason: string;
  readonly latency_ms: number;
  readonly input_tokens: number;
  readonly output_tokens: number;
  readonly estimated_cost_usd: number;
}

export interface RankResultWriteOutcome {
  // True when the row was newly inserted. False when (user_id, message_id)
  // already had a row — the existing row is unchanged.
  readonly inserted: boolean;
  // ID of the row — either the newly-inserted id or the existing one.
  // Always populated so Phase 3D.1 callers can use it as the alert
  // foreign key (alerts.rank_result_id) without a second query.
  readonly rank_result_id: number;
}

export interface RankResultStore {
  // ON CONFLICT (user_id, message_id) DO NOTHING. Returns { inserted }.
  write(input: RankResultInput): Promise<RankResultWriteOutcome>;
  // Returns the most recent row for (user_id, message_id), or null.
  // Used by tests + the 3C.4 evidence script.
  get(userId: string, messageId: string): Promise<RankResult | null>;
  // Count of rows for a user, optionally filtered to a single label.
  count(userId: string, label?: RankLabel): Promise<number>;
  // Most recent N rows for a user, newest first. Used by the 3C.4
  // evidence script to surface recent decisions for human inspection.
  recent(userId: string, limit: number): Promise<readonly RankResult[]>;
}

export class InMemoryRankResultStore implements RankResultStore {
  private nextId = 1;
  private readonly rows: RankResult[] = [];

  async write(input: RankResultInput): Promise<RankResultWriteOutcome> {
    const existing = this.rows.find(
      (r) => r.user_id === input.user_id && r.message_id === input.message_id
    );
    if (existing) {
      return Object.freeze({ inserted: false, rank_result_id: existing.id });
    }
    const id = this.nextId++;
    this.rows.push(
      Object.freeze({
        id,
        user_id: input.user_id,
        message_id: input.message_id,
        invocation_id: input.invocation_id,
        model_name: input.model_name,
        prompt_version: input.prompt_version,
        label: input.label,
        score: input.score,
        reason: input.reason,
        latency_ms: input.latency_ms,
        input_tokens: input.input_tokens,
        output_tokens: input.output_tokens,
        estimated_cost_usd: input.estimated_cost_usd,
        created_at: new Date().toISOString()
      })
    );
    return Object.freeze({ inserted: true, rank_result_id: id });
  }

  async get(userId: string, messageId: string): Promise<RankResult | null> {
    const row = this.rows.find(
      (r) => r.user_id === userId && r.message_id === messageId
    );
    return row ?? null;
  }

  async count(userId: string, label?: RankLabel): Promise<number> {
    return this.rows.filter(
      (r) => r.user_id === userId && (label === undefined || r.label === label)
    ).length;
  }

  async recent(userId: string, limit: number): Promise<readonly RankResult[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const userRows = this.rows.filter((r) => r.user_id === userId);
    // Newest first by created_at; ties broken by id descending.
    userRows.sort((a, b) => {
      if (a.created_at !== b.created_at) {
        return b.created_at.localeCompare(a.created_at);
      }
      return b.id - a.id;
    });
    return Object.freeze(userRows.slice(0, limit));
  }
}
