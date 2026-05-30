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
  REQUIRED_COLUMNS,
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
    // authoritative end-to-end migration apply target). 14 tables as
    // of Phase v0.5.1 (after 0005_users_v05 adds `invite_tokens`).
    assert.equal(REQUIRED_TABLES.length, 14);
  });

  it('verifyMigrations returns ok=true when every required table AND column exists', async () => {
    assert.ok(db);
    const result = await verifyMigrations(db);
    assert.equal(result.ok, true);
    assert.deepEqual(result.missing_tables, []);
    assert.deepEqual(result.missing_columns, []);
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

  /* ------------------------------------------------------------------ */
  /* Step 4.2 — column-level verification (load-bearing v0.5.1 columns) */
  /* ------------------------------------------------------------------ */
  //
  // Real incident class hardened against: 3G.1 verified table presence
  // only. A migration that ALTERs an existing table to add a load-
  // bearing column (e.g. 0005's users.is_founder, 0006's
  // invite_tokens.intended_phone_encrypted) is INVISIBLE to that
  // verifier. Neon could have the table without the column and the
  // runtime would fail at first use of the missing column. These
  // tests prove the column-level fail-loud behavior.

  describe('column-level verification (Step 4.2)', () => {
    it('REQUIRED_COLUMNS covers every load-bearing v0.5.1 column added by ALTER TABLE migrations', () => {
      // Founder directive 2026-05-29: spec is the test. If a new
      // migration adds a load-bearing column, this list must be
      // updated AND the test below must include the column.
      const have = new Set(REQUIRED_COLUMNS.map((c) => `${c.table}.${c.column}`));
      for (const expected of [
        'users.phone_e164_encrypted',
        'users.phone_e164_hash',
        'users.is_founder',
        'invite_tokens.token_hash',
        'invite_tokens.intended_phone_hash',
        'invite_tokens.intended_phone_encrypted',
        'invite_tokens.consumed_at'
      ]) {
        assert.ok(have.has(expected), `REQUIRED_COLUMNS missing ${expected}`);
      }
    });

    it('table exists but required column missing → verifier fails with named column', async () => {
      // Spin up a fresh PGlite with every migration EXCEPT 0006 —
      // that leaves invite_tokens in place but without
      // intended_phone_encrypted. This is the exact drift shape the
      // step exists to catch.
      const scratch = new PGlite();
      try {
        await applyMigrations(scratch, { skip: ['0006_invite_phone_encrypted.sql'] });
        const scratchDb = drizzle(scratch, { schema }) as unknown as DrizzleClient;

        const result = await verifyMigrations(scratchDb);
        assert.equal(result.ok, false);
        // Tables — all present (we didn't drop invite_tokens, we just
        // skipped the ALTER that adds the new column).
        assert.deepEqual(result.missing_tables, []);
        // Column — named.
        const names = result.missing_columns.map((m) => `${m.table}.${m.column}`);
        assert.ok(
          names.includes('invite_tokens.intended_phone_encrypted'),
          `expected invite_tokens.intended_phone_encrypted in missing_columns; got: ${names.join(',')}`
        );
        const drift = result.missing_columns.find(
          (m) => m.table === 'invite_tokens' && m.column === 'intended_phone_encrypted'
        );
        assert.ok(drift);
        assert.equal(drift.migration, '0006_invite_phone_encrypted.sql');
      } finally {
        await scratch.close();
      }
    });

    it('multiple missing columns → ALL are named with their source migrations', async () => {
      // Skip BOTH 0005 and 0006. 0005 adds users.* + invite_tokens
      // (CREATE TABLE). Skipping 0005 entirely drops the invite_tokens
      // table — so the missing_tables list reports it, NOT the column.
      // For column-only drift, skip 0006 (one missing column) and
      // ALSO patch users so its phone_e164_hash column is missing.
      const scratch = new PGlite();
      try {
        await applyMigrations(scratch, { skip: ['0006_invite_phone_encrypted.sql'] });
        // Drop the users.phone_e164_hash column to simulate a partial
        // 0005 apply (e.g. an operator hand-rolled a portion of the
        // migration on Neon).
        await scratch.exec('ALTER TABLE users DROP COLUMN phone_e164_hash');
        const scratchDb = drizzle(scratch, { schema }) as unknown as DrizzleClient;

        const result = await verifyMigrations(scratchDb);
        assert.equal(result.ok, false);
        const names = result.missing_columns.map((m) => `${m.table}.${m.column}`).sort();
        assert.ok(names.includes('users.phone_e164_hash'));
        assert.ok(names.includes('invite_tokens.intended_phone_encrypted'));
        // Each missing column carries its source migration so the
        // operator can correlate to the recovery action.
        const byColumn = new Map(result.missing_columns.map((m) => [`${m.table}.${m.column}`, m.migration]));
        assert.equal(byColumn.get('users.phone_e164_hash'), '0005_users_v05.sql');
        assert.equal(byColumn.get('invite_tokens.intended_phone_encrypted'), '0006_invite_phone_encrypted.sql');
      } finally {
        await scratch.close();
      }
    });

    it('column drift skipped when the parent table itself is missing (no double-reporting)', async () => {
      // Skip 0005 entirely → invite_tokens table is missing. The
      // column entries that belong to invite_tokens MUST NOT also
      // appear in missing_columns — the operator's recovery action
      // is "apply 0005," which restores both table + columns. Double-
      // reporting is noise.
      const scratch = new PGlite();
      try {
        await applyMigrations(scratch, {
          skip: ['0005_users_v05.sql', '0006_invite_phone_encrypted.sql']
        });
        const scratchDb = drizzle(scratch, { schema }) as unknown as DrizzleClient;

        const result = await verifyMigrations(scratchDb);
        assert.equal(result.ok, false);
        assert.ok(result.missing_tables.find((m) => m.name === 'invite_tokens'));
        for (const c of result.missing_columns) {
          assert.notEqual(c.table, 'invite_tokens', 'columns on a missing table must not double-report');
        }
      } finally {
        await scratch.close();
      }
    });

    it('PendingMigrationsError message names ALL missing columns on their own lines', async () => {
      const scratch = new PGlite();
      try {
        await applyMigrations(scratch, { skip: ['0006_invite_phone_encrypted.sql'] });
        const scratchDb = drizzle(scratch, { schema }) as unknown as DrizzleClient;
        let caught: PendingMigrationsError | null = null;
        try {
          await verifyMigrationsOrThrow(scratchDb);
        } catch (err) {
          caught = err as PendingMigrationsError;
        }
        assert.ok(caught instanceof PendingMigrationsError);
        // The message must name the column AND the source migration
        // so the operator can act without parsing JSON.
        assert.match(caught!.message, /invite_tokens\.intended_phone_encrypted/);
        assert.match(caught!.message, /0006_invite_phone_encrypted\.sql/);
        assert.match(caught!.message, /pnpm --filter @brevio\/fomo run migrate:neon/);
      } finally {
        await scratch.close();
      }
    });

    it('PendingMigrationsError carries structured missing_columns alongside missing_tables', () => {
      const err = new PendingMigrationsError(
        [{ name: 'oauth_tokens', migration: '0000_init.sql' }],
        [{ table: 'invite_tokens', column: 'intended_phone_encrypted', migration: '0006_invite_phone_encrypted.sql' }]
      );
      assert.equal(err.missing_tables.length, 1);
      assert.equal(err.missing_columns.length, 1);
      assert.match(err.message, /oauth_tokens/);
      assert.match(err.message, /intended_phone_encrypted/);
    });

    it('column-level verification is READ-ONLY (no information_schema mutation)', async () => {
      assert.ok(db);
      // information_schema is non-mutable from user SQL anyway, but
      // we still assert the verifier doesn't somehow trigger ALTER
      // statements by counting the row count before + after.
      const before = await pglite!.query<{ n: string }>(
        `SELECT count(*)::text AS n FROM information_schema.columns WHERE table_schema='public'`
      );
      await verifyMigrations(db);
      const after = await pglite!.query<{ n: string }>(
        `SELECT count(*)::text AS n FROM information_schema.columns WHERE table_schema='public'`
      );
      assert.equal(after.rows[0].n, before.rows[0].n);
    });
  });
});
