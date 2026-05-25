// Postgres-backed RankResultStore. Same contract as InMemoryRankResultStore.
// Write uses ON CONFLICT (user_id, message_id) DO NOTHING.

import { and, desc, eq, sql } from 'drizzle-orm';

import {
  type RankLabel,
  type RankResult,
  type RankResultInput,
  type RankResultStore,
  type RankResultWriteOutcome
} from '../../memory/rank-results.js';
import { type DrizzleClient } from '../client.js';
import { rank_results } from '../schema.js';

export class PostgresRankResultStore implements RankResultStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(input: RankResultInput): Promise<RankResultWriteOutcome> {
    const returned = await this.db
      .insert(rank_results)
      .values({
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
        estimated_cost_usd: input.estimated_cost_usd
      })
      .onConflictDoNothing({ target: [rank_results.user_id, rank_results.message_id] })
      .returning({ id: rank_results.id });
    return Object.freeze({ inserted: returned.length > 0 });
  }

  async get(userId: string, messageId: string): Promise<RankResult | null> {
    const rows = await this.db
      .select()
      .from(rank_results)
      .where(and(eq(rank_results.user_id, userId), eq(rank_results.message_id, messageId)))
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return Object.freeze({
      id: r.id,
      user_id: r.user_id,
      message_id: r.message_id,
      invocation_id: r.invocation_id,
      model_name: r.model_name,
      prompt_version: r.prompt_version,
      label: r.label as RankLabel,
      score: r.score,
      reason: r.reason,
      latency_ms: r.latency_ms,
      input_tokens: r.input_tokens,
      output_tokens: r.output_tokens,
      estimated_cost_usd: r.estimated_cost_usd,
      created_at: r.created_at.toISOString()
    });
  }

  async count(userId: string, label?: RankLabel): Promise<number> {
    const whereClause =
      label === undefined
        ? eq(rank_results.user_id, userId)
        : and(eq(rank_results.user_id, userId), eq(rank_results.label, label));
    const rows = await this.db
      .select({ n: sql<number>`count(*)::int` })
      .from(rank_results)
      .where(whereClause);
    return rows[0]?.n ?? 0;
  }

  async recent(userId: string, limit: number): Promise<readonly RankResult[]> {
    if (!Number.isInteger(limit) || limit <= 0) return Object.freeze([]);
    const rows = await this.db
      .select()
      .from(rank_results)
      .where(eq(rank_results.user_id, userId))
      .orderBy(desc(rank_results.created_at), desc(rank_results.id))
      .limit(limit);
    return Object.freeze(
      rows.map((r) =>
        Object.freeze({
          id: r.id,
          user_id: r.user_id,
          message_id: r.message_id,
          invocation_id: r.invocation_id,
          model_name: r.model_name,
          prompt_version: r.prompt_version,
          label: r.label as RankLabel,
          score: r.score,
          reason: r.reason,
          latency_ms: r.latency_ms,
          input_tokens: r.input_tokens,
          output_tokens: r.output_tokens,
          estimated_cost_usd: r.estimated_cost_usd,
          created_at: r.created_at.toISOString()
        })
      )
    );
  }
}
