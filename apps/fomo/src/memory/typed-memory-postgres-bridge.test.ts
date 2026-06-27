import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { after, before, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { PGlite } from '@electric-sql/pglite';
import { drizzle } from 'drizzle-orm/pglite';

import { type DrizzleClient } from '../db/client.js';
import * as schema from '../db/schema.js';
import { PostgresMemorySignalStore } from '../db/stores/memory-postgres.js';
import {
  MemorySignalsBackedTypedMemoryStore,
  readTypedMemory,
  typedMemoryScopeKeyForBridgedCorrectionSignal,
  typedMemoryScopeKeyForBridgedMemorySignal
} from './typed-memory.js';

const RUN_PG = process.env.BREVIO_RUN_PG_TESTS === 'true';

let pglite: PGlite | null = null;
let db: DrizzleClient | null = null;

async function setup(): Promise<{ readonly pglite: PGlite; readonly db: DrizzleClient }> {
  const instance = new PGlite();
  const here = path.dirname(fileURLToPath(import.meta.url));
  const migrationsDir = path.resolve(here, '../db/migrations');
  const entries = await readdir(migrationsDir);
  const sqlFiles = entries.filter((f) => f.endsWith('.sql')).sort();

  for (const file of sqlFiles) {
    const migrationSql = await readFile(path.join(migrationsDir, file), 'utf8');
    for (const stmt of migrationSql.split('--> statement-breakpoint')) {
      const trimmed = stmt.trim();
      if (trimmed.length === 0) continue;
      await instance.exec(trimmed);
    }
  }

  const wrapped = drizzle(instance, { schema }) as unknown as DrizzleClient;
  return { pglite: instance, db: wrapped };
}

describe('M1 memory_signals-backed typed memory facade — Postgres bridge', { skip: !RUN_PG ? 'BREVIO_RUN_PG_TESTS not set' : false }, () => {
  before(async () => {
    const result = await setup();
    pglite = result.pglite;
    db = result.db;
  });

  after(async () => {
    if (pglite) await pglite.close();
    pglite = null;
    db = null;
  });

  it('bridges real Postgres memory_signals with cross-user isolation and stable ordering', async () => {
    assert.ok(db);
    const memoryStore = new PostgresMemorySignalStore(db);
    const typedStore = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    await memoryStore.upsert({
      user_id: 'pg-user-a',
      kind: 'sender_suppressed',
      scope_key: 'sharedsenderhash',
      detail: { suppressed: true, unknown_metadata: null },
      source: 'user_confirmed',
      updated_at: '2026-06-24T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'pg-user-b',
      kind: 'sender_suppressed',
      scope_key: 'sharedsenderhash',
      detail: { suppressed: false, unknown_metadata: 'other-user' },
      source: 'user_confirmed',
      updated_at: '2026-06-24T12:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'pg-user-a',
      kind: 'sender_feedback_ignored',
      scope_key: 'othersenderhash',
      detail: { ignored_count: 2, unknown_metadata: null },
      source: 'feedback_derived',
      updated_at: '2026-06-24T12:00:00.000Z'
    });

    const rows = await readTypedMemory(typedStore, 'pg-user-a', { kinds: ['correction'] });

    assert.deepEqual(
      rows.map((row) => ({
        user_id: row.user_id,
        scope_key: row.scope_key,
        rule: row.kind === 'correction' ? row.rule : null,
        value: row.kind === 'correction' ? row.value : null
      })),
      [
        {
          user_id: 'pg-user-a',
          scope_key: typedMemoryScopeKeyForBridgedCorrectionSignal('sender_suppressed', 'sharedsenderhash'),
          rule: 'sender_suppressed',
          value: { suppressed: true, unknown_metadata: null }
        },
        {
          user_id: 'pg-user-a',
          scope_key: typedMemoryScopeKeyForBridgedCorrectionSignal('sender_feedback_ignored', 'othersenderhash'),
          rule: 'sender_feedback_ignored',
          value: { ignored_count: 2, unknown_metadata: null }
        }
      ]
    );
  });

  it('excludes deleted and tombstoned real Postgres memory_signals from typed reads', async () => {
    assert.ok(db);
    const memoryStore = new PostgresMemorySignalStore(db);
    const typedStore = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    await memoryStore.upsert({
      user_id: 'pg-deleted-user',
      kind: 'quietness_preference',
      scope_key: null,
      detail: { max_per_day: 3, deleted: true },
      source: 'user_confirmed',
      updated_at: '2026-06-24T13:00:00.000Z'
    });
    await memoryStore.upsert({
      user_id: 'pg-deleted-user',
      kind: 'timing_preference',
      scope_key: null,
      detail: { window: 'morning', tombstoned_at: '2026-06-24T13:05:00.000Z' },
      source: 'founder_set',
      updated_at: '2026-06-24T13:05:00.000Z'
    });

    assert.deepEqual(await readTypedMemory(typedStore, 'pg-deleted-user', { kinds: ['preference'] }), []);
    assert.equal(
      await typedStore.get(
        'pg-deleted-user',
        'preference',
        typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference')
      ),
      null
    );
  });

  it('keeps the Postgres-backed M1 bridge no-migration and read-only', async () => {
    assert.ok(db);
    assert.ok(pglite);
    const memoryStore = new PostgresMemorySignalStore(db);
    const typedStore = new MemorySignalsBackedTypedMemoryStore(memoryStore);

    const tableResult = await pglite.query<{ tablename: string }>(
      `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename`
    );
    assert.equal(tableResult.rows.some((row) => /typed[_-]?memory/i.test(row.tablename)), false);
    assert.equal(tableResult.rows.some((row) => row.tablename === 'memory_signals'), true);

    await assert.rejects(
      () =>
        typedStore.write({
          user_id: 'pg-readonly-user',
          kind: 'preference',
          scope_key: typedMemoryScopeKeyForBridgedMemorySignal('quietness_preference'),
          source: 'user_stated',
          source_ref: 'reply:readonly',
          confidence: 'high',
          stale_marked_at: null,
          retracted: false,
          superseded_by: null,
          attribute: 'quietness_preference',
          value: { max_per_day: 5 }
        }),
      /read-only/
    );
  });
});
