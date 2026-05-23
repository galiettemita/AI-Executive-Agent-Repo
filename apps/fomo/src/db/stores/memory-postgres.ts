// Postgres-backed MemorySignalStore. Same contract as
// InMemoryMemorySignalStore from Phase 2C.
//
// Two implementation notes specific to Postgres:
//
//   * scope_key NULL handling — the in-memory store identifies signals by
//     (user_id, kind, scope_key) where scope_key may be null. Postgres
//     unique indexes treat NULL as non-equal to NULL, which would let
//     duplicate null-scoped signals slip in. We persist null as the
//     empty-string sentinel '' and translate at the layer boundary.
//
//   * upsert via ON CONFLICT — Drizzle's onConflictDoUpdate targets the
//     unique index (user_id, kind, scope_key). Confidence is clamped
//     before the call.

import { and, asc, eq } from 'drizzle-orm';

import { redact } from '../../core/safe-logger.js';
import {
  defaultConfidence,
  type MemorySignal,
  type MemorySignalInput,
  type MemorySignalKind,
  type MemorySignalSource,
  type MemorySignalStore
} from '../../memory/memory-signals.js';
import { type DrizzleClient } from '../client.js';
import { memory_signals } from '../schema.js';

function clamp01(n: number): number {
  if (Number.isNaN(n)) return 0;
  if (n < 0) return 0;
  if (n > 1) return 1;
  return n;
}

// Empty string is the on-disk sentinel for "no scope". The in-memory store
// uses null; we translate at the boundary so Postgres unique index works.
const NULL_SENTINEL = '';

function scopeKeyIn(scopeKey: string | null): string {
  return scopeKey ?? NULL_SENTINEL;
}

function scopeKeyOut(stored: string): string | null {
  return stored === NULL_SENTINEL ? null : stored;
}

export class PostgresMemorySignalStore implements MemorySignalStore {
  private readonly db: DrizzleClient;

  constructor(db: DrizzleClient) {
    this.db = db;
  }

  async upsert(input: MemorySignalInput): Promise<void> {
    const detail = redact(input.detail) as Record<string, unknown>;
    const confidence = clamp01(input.confidence ?? defaultConfidence(input.source));
    const scope_key = scopeKeyIn(input.scope_key);

    const values: typeof memory_signals.$inferInsert = {
      user_id: input.user_id,
      kind: input.kind,
      scope_key,
      detail,
      confidence,
      source: input.source
    };
    if (input.updated_at !== undefined) {
      values.updated_at = new Date(input.updated_at);
    }

    await this.db
      .insert(memory_signals)
      .values(values)
      .onConflictDoUpdate({
        target: [memory_signals.user_id, memory_signals.kind, memory_signals.scope_key],
        set: {
          detail,
          confidence,
          source: input.source,
          updated_at: values.updated_at ?? new Date()
        }
      });
  }

  async get(
    userId: string,
    kind: MemorySignalKind,
    scopeKey: string | null = null
  ): Promise<MemorySignal | null> {
    const rows = await this.db
      .select()
      .from(memory_signals)
      .where(
        and(
          eq(memory_signals.user_id, userId),
          eq(memory_signals.kind, kind),
          eq(memory_signals.scope_key, scopeKeyIn(scopeKey))
        )
      )
      .limit(1);
    const r = rows[0];
    if (!r) return null;
    return Object.freeze({
      id: r.id,
      updated_at: r.updated_at.toISOString(),
      user_id: r.user_id,
      kind: r.kind as MemorySignalKind,
      scope_key: scopeKeyOut(r.scope_key),
      detail: r.detail as Record<string, unknown>,
      confidence: r.confidence,
      source: r.source as MemorySignalSource
    });
  }

  async list(userId: string): Promise<readonly MemorySignal[]> {
    const rows = await this.db
      .select()
      .from(memory_signals)
      .where(eq(memory_signals.user_id, userId))
      .orderBy(asc(memory_signals.id));
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        updated_at: r.updated_at.toISOString(),
        user_id: r.user_id,
        kind: r.kind as MemorySignalKind,
        scope_key: scopeKeyOut(r.scope_key),
        detail: r.detail as Record<string, unknown>,
        confidence: r.confidence,
        source: r.source as MemorySignalSource
      })
    );
  }

  async listByKind(userId: string, kind: MemorySignalKind): Promise<readonly MemorySignal[]> {
    const rows = await this.db
      .select()
      .from(memory_signals)
      .where(and(eq(memory_signals.user_id, userId), eq(memory_signals.kind, kind)))
      .orderBy(asc(memory_signals.id));
    return rows.map((r) =>
      Object.freeze({
        id: r.id,
        updated_at: r.updated_at.toISOString(),
        user_id: r.user_id,
        kind: r.kind as MemorySignalKind,
        scope_key: scopeKeyOut(r.scope_key),
        detail: r.detail as Record<string, unknown>,
        confidence: r.confidence,
        source: r.source as MemorySignalSource
      })
    );
  }

  async delete(userId: string, kind: MemorySignalKind, scopeKey: string | null): Promise<boolean> {
    const result = await this.db
      .delete(memory_signals)
      .where(
        and(
          eq(memory_signals.user_id, userId),
          eq(memory_signals.kind, kind),
          eq(memory_signals.scope_key, scopeKeyIn(scopeKey))
        )
      )
      .returning({ id: memory_signals.id });
    return result.length > 0;
  }
}
