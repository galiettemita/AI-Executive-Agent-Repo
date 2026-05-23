// Postgres-backed ToolInvocationStore. Same contract as
// InMemoryToolInvocationStore. Metadata is redacted via safe-logger.redact
// on write — load-bearing privacy invariant (FOMO_PLAN §9.10 + §16.4).

import { and, count, desc, eq } from 'drizzle-orm';

import { redact } from '../../core/safe-logger.js';
import {
  type ToolInvocationInput,
  type ToolInvocationRecord,
  type ToolInvocationStatus,
  type ToolInvocationStore
} from '../../core/tool-invocations.js';
import { type DrizzleClient } from '../client.js';
import { tool_invocations } from '../schema.js';

export class PostgresToolInvocationStore implements ToolInvocationStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(input: ToolInvocationInput): Promise<void> {
    const metadata = input.metadata ? (redact(input.metadata) as Record<string, unknown>) : null;
    const values: typeof tool_invocations.$inferInsert = {
      user_id: input.user_id,
      tool_id: input.tool_id,
      invocation_id: input.invocation_id,
      policy_decision: input.policy_decision,
      status: input.status,
      latency_ms: input.latency_ms ?? null,
      error_code: input.error_code ?? null,
      error_reason: input.error_reason ?? null,
      metadata
    };
    if (input.occurred_at !== undefined) {
      values.occurred_at = new Date(input.occurred_at);
    }
    await this.db.insert(tool_invocations).values(values);
  }

  async recent(userId: string, limit = 100): Promise<readonly ToolInvocationRecord[]> {
    const rows = await this.db
      .select()
      .from(tool_invocations)
      .where(eq(tool_invocations.user_id, userId))
      .orderBy(desc(tool_invocations.occurred_at))
      .limit(limit);
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        occurred_at: r.occurred_at.toISOString(),
        user_id: r.user_id,
        tool_id: r.tool_id,
        invocation_id: r.invocation_id,
        policy_decision: r.policy_decision,
        status: r.status as ToolInvocationStatus,
        latency_ms: r.latency_ms,
        error_code: r.error_code,
        error_reason: r.error_reason,
        metadata: r.metadata as Record<string, unknown> | null
      })
    );
  }

  async countByTool(userId: string, toolId: string): Promise<number> {
    const rows = await this.db
      .select({ n: count() })
      .from(tool_invocations)
      .where(and(eq(tool_invocations.user_id, userId), eq(tool_invocations.tool_id, toolId)));
    return Number(rows[0]?.n ?? 0);
  }

  async countByStatus(userId: string, status: ToolInvocationStatus): Promise<number> {
    const rows = await this.db
      .select({ n: count() })
      .from(tool_invocations)
      .where(and(eq(tool_invocations.user_id, userId), eq(tool_invocations.status, status)));
    return Number(rows[0]?.n ?? 0);
  }

  async byInvocationId(invocationId: string): Promise<ToolInvocationRecord | null> {
    const rows = await this.db
      .select()
      .from(tool_invocations)
      .where(eq(tool_invocations.invocation_id, invocationId))
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return Object.freeze({
      id: r.id,
      occurred_at: r.occurred_at.toISOString(),
      user_id: r.user_id,
      tool_id: r.tool_id,
      invocation_id: r.invocation_id,
      policy_decision: r.policy_decision,
      status: r.status as ToolInvocationStatus,
      latency_ms: r.latency_ms,
      error_code: r.error_code,
      error_reason: r.error_reason,
      metadata: r.metadata as Record<string, unknown> | null
    });
  }
}
