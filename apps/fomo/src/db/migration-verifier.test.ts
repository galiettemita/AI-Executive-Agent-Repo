// Phase 3G.1 item #1 — regression tests for migration-verifier.
//
// Original incident captured: 2026-05-28 01:06 UTC. The first 3
// inbound POSTs during the 3F.2 smoke run returned HTTP 500 because
// migration `0004_inbound_replies.sql` had been applied via PGlite
// in the gated-pg suite but had NOT been applied against the live
// Neon database. Without this verifier the runtime would boot fine
// and only fail on the first runtime query against the missing
// table — a silent failure mode that lost the founder ~15 min.
//
// fail-before / pass-after demonstration:
//   * Test "incident reproduction" — starts a PGlite DB with EVERY
//     migration applied EXCEPT 0004_inbound_replies. verifyMigrations()
//     returns ok=false with missing=['inbound_replies'] +
//     migration='0004_inbound_replies.sql'. verifyMigrationsOrThrow()
//     throws a PendingMigrationsError naming the table. This is the
//     exact incident shape — caught at boot, not at first inbound POST.
//   * Test "happy path" — same setup with all migrations applied
//     returns ok=true and zero missing.
//
// PGlite is used (NOT the live Neon DB) — per founder directive
// 2026-05-29: "Do not drop real Neon production tables for fault
// injection; use PGlite, Neon test branch, scratch DB, or mocked
// verifier behavior."

import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { after, before, describe, it } from 'node:test';
import { fileURLToPath } from 'node:url';

import { PGlite } from '@electric-sql/pglite';
import { drizzle } from 'drizzle-orm/pglite';

import { type DrizzleClient } from './client.js';
import * as schema from './schema.js';
import {
  PendingMigrationsError,
  REQUIRED_TABLES,
  verifyMigrations,
  verifyMigrationsOrThrow
} from './migration-verifier.js';

const here = path.dirname(fileURLToPath(import.meta.url));
const migrationsDir = path.resolve(here, 'migrations');

async function applyMigrations(instance: PGlite, opts: { skip?: readonly string[] } = {}): Promise<void> {
  const skip = new Set(opts.skip ?? []);
  const entries = await readdir(migrationsDir);
  const sqlFiles = entries.filter((f) => f.endsWith('.sql')).sort();
  for (const file of sqlFiles) {
    if (skip.has(file)) continue;
    const migrationSql = await readFile(path.join(migrationsDir, file), 'utf8');
    for (const stmt of migrationSql.split('--> statement-breakpoint')) {
      const trimmed = stmt.trim();
      if (trimmed.length === 0) continue;
      await instance.exec(trimmed);
    }
  }
}

describe('migration-verifier', () => {
  let pglite: PGlite | null = null;
  let db: DrizzleClient | null = null;

  before(async () => {
    pglite = new PGlite();
    await applyMigrations(pglite);
    db = drizzle(pglite, { schema }) as unknown as DrizzleClient;
  });

  after(async () => {
    if (pglite) await pglite.close();
    pglite = null;
    db = null;
  });

  it('REQUIRED_TABLES is frozen and lists every public-schema table the runtime depends on', () => {
    assert.ok(Object.isFrozen(REQUIRED_TABLES));
    // Cross-check the count matches the gated-pg test (which is the
    // authoritative end-to-end migration apply target). 13 tables as
    // of Phase 3F.1 (after 0004_inbound_replies).
    assert.equal(REQUIRED_TABLES.length, 13);
  });

  it('verifyMigrations returns ok=true when every required table exists', async () => {
    assert.ok(db);
    const result = await verifyMigrations(db);
    assert.equal(result.ok, true);
    assert.deepEqual(result.missing_tables, []);
    assert.equal(result.required_tables.length, REQUIRED_TABLES.length);
  });

  it('verifyMigrationsOrThrow does NOT throw when every required table exists', async () => {
    assert.ok(db);
    await verifyMigrationsOrThrow(db);
    // No exception is the assertion. Reaching here is the pass.
  });

  describe('incident reproduction (2026-05-28 inbound_replies missing on Neon)', () => {
    let scratchPglite: PGlite | null = null;
    let scratchDb: DrizzleClient | null = null;

    before(async () => {
      scratchPglite = new PGlite();
      // Apply every migration EXCEPT the one that introduces
      // inbound_replies — the exact shape that caused the 3F.2 smoke
      // 500s. The runtime would have booted fine; only the first
      // /sendblue/inbound POST would have failed.
      await applyMigrations(scratchPglite, { skip: ['0004_inbound_replies.sql'] });
      scratchDb = drizzle(scratchPglite, { schema }) as unknown as DrizzleClient;
    });

    after(async () => {
      if (scratchPglite) await scratchPglite.close();
      scratchPglite = null;
      scratchDb = null;
    });

    it('verifyMigrations returns ok=false with inbound_replies named as missing', async () => {
      assert.ok(scratchDb);
      const result = await verifyMigrations(scratchDb);
      assert.equal(result.ok, false);
      assert.equal(result.missing_tables.length, 1);
      assert.equal(result.missing_tables[0].name, 'inbound_replies');
      assert.equal(result.missing_tables[0].migration, '0004_inbound_replies.sql');
    });

    it('verifyMigrationsOrThrow throws PendingMigrationsError with the named table + migration file', async () => {
      assert.ok(scratchDb);
      await assert.rejects(
        () => verifyMigrationsOrThrow(scratchDb!),
        (err: unknown) => {
          assert.ok(err instanceof PendingMigrationsError, 'expected PendingMigrationsError');
          assert.equal(err.missing_tables.length, 1);
          assert.equal(err.missing_tables[0].name, 'inbound_replies');
          assert.match(err.message, /fomo\.migrations\.pending/);
          assert.match(err.message, /inbound_replies/);
          assert.match(err.message, /0004_inbound_replies\.sql/);
          assert.match(err.message, /pnpm --filter @brevio\/fomo run migrate:neon/);
          assert.match(err.message, /No auto-apply\. No env bypass\./);
          return true;
        }
      );
    });
  });

  describe('incident reproduction (multiple missing tables — generalized fault)', () => {
    let scratchPglite: PGlite | null = null;
    let scratchDb: DrizzleClient | null = null;

    before(async () => {
      scratchPglite = new PGlite();
      // Skip everything past 0000_init: alerts + inbound_replies +
      // gmail_cursors + rank_results all missing. Proves the verifier
      // surfaces ALL of them (not just the first one) so the operator
      // can apply them in one shot.
      await applyMigrations(scratchPglite, {
        skip: [
          '0001_gmail_cursors.sql',
          '0002_rank_results.sql',
          '0003_alerts.sql',
          '0004_inbound_replies.sql'
        ]
      });
      scratchDb = drizzle(scratchPglite, { schema }) as unknown as DrizzleClient;
    });

    after(async () => {
      if (scratchPglite) await scratchPglite.close();
      scratchPglite = null;
      scratchDb = null;
    });

    it('verifyMigrations names every missing table with its source migration', async () => {
      assert.ok(scratchDb);
      const result = await verifyMigrations(scratchDb);
      assert.equal(result.ok, false);
      const missingNames = result.missing_tables.map((m) => m.name).sort();
      assert.deepEqual(missingNames, ['alerts', 'gmail_cursors', 'inbound_replies', 'rank_results']);
      const byName = new Map(result.missing_tables.map((m) => [m.name, m.migration]));
      assert.equal(byName.get('gmail_cursors'), '0001_gmail_cursors.sql');
      assert.equal(byName.get('rank_results'), '0002_rank_results.sql');
      assert.equal(byName.get('alerts'), '0003_alerts.sql');
      assert.equal(byName.get('inbound_replies'), '0004_inbound_replies.sql');
    });

    it('PendingMigrationsError message lists every missing table on its own line', async () => {
      assert.ok(scratchDb);
      let caught: PendingMigrationsError | null = null;
      try {
        await verifyMigrationsOrThrow(scratchDb);
      } catch (err) {
        caught = err as PendingMigrationsError;
      }
      assert.ok(caught instanceof PendingMigrationsError);
      assert.match(caught!.message, /alerts.*0003_alerts\.sql/);
      assert.match(caught!.message, /gmail_cursors.*0001_gmail_cursors\.sql/);
      assert.match(caught!.message, /inbound_replies.*0004_inbound_replies\.sql/);
      assert.match(caught!.message, /rank_results.*0002_rank_results\.sql/);
    });
  });

  describe('READ-ONLY invariant', () => {
    it('verifyMigrations does not write any row to any required table', async () => {
      assert.ok(pglite);
      assert.ok(db);
      const before = await pglite.query<{ tablename: string; n_live_tup: number }>(
        `SELECT relname AS tablename, n_live_tup FROM pg_stat_user_tables WHERE schemaname='public' ORDER BY relname`
      );
      await verifyMigrations(db);
      const after = await pglite.query<{ tablename: string; n_live_tup: number }>(
        `SELECT relname AS tablename, n_live_tup FROM pg_stat_user_tables WHERE schemaname='public' ORDER BY relname`
      );
      // n_live_tup should be unchanged for every table — the verifier
      // never writes. (PGlite resets statistics on every connection,
      // but the relative delta inside one connection still holds.)
      assert.deepEqual(after.rows, before.rows);
    });
  });
});
