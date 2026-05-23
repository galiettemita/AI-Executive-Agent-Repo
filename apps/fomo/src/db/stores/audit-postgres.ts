// Postgres-backed AuditStore. Same contract as InMemoryAuditStore from
// Phase 2A. Detail is redacted via safe-logger.redact on write — matching
// the in-memory behavior so callers cannot tell which backend they got.

import { desc, eq } from 'drizzle-orm';

import { type AuditAction, type AuditEntry, type AuditResult, type AuditStore } from '../../core/audit.js';
import { redact } from '../../core/safe-logger.js';
import { type DrizzleClient } from '../client.js';
import { audit_log } from '../schema.js';

export class PostgresAuditStore implements AuditStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async write(
    entry: Omit<AuditEntry, 'id' | 'occurred_at'> & { occurred_at?: string }
  ): Promise<void> {
    const detail = entry.detail ? (redact(entry.detail) as Record<string, unknown>) : null;
    const values: typeof audit_log.$inferInsert = {
      actor_user_id: entry.actor_user_id,
      actor_ip: entry.actor_ip,
      actor_user_agent: entry.actor_user_agent,
      action: entry.action,
      target: entry.target,
      result: entry.result,
      detail
    };
    if (entry.occurred_at !== undefined) {
      values.occurred_at = new Date(entry.occurred_at);
    }
    await this.db.insert(audit_log).values(values);
  }

  async recent(userId: string, limit = 100): Promise<AuditEntry[]> {
    const rows = await this.db
      .select()
      .from(audit_log)
      .where(eq(audit_log.actor_user_id, userId))
      .orderBy(desc(audit_log.occurred_at))
      .limit(limit);
    return rows.map((r) => ({
      id: r.id,
      occurred_at: r.occurred_at.toISOString(),
      actor_user_id: r.actor_user_id,
      actor_ip: r.actor_ip,
      actor_user_agent: r.actor_user_agent,
      action: r.action as AuditAction,
      target: r.target,
      result: r.result as AuditResult,
      detail: r.detail as Record<string, unknown> | null
    }));
  }
}
