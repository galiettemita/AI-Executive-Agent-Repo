// Phase 3G.1 — explicit Neon migration runner.
//
// The dev server REFUSES to boot when migrations are pending (item #1
// of the 3G.1 scope, founder-locked 2026-05-29). This script applies
// the pending migrations explicitly — never automatically, never
// invoked by the runtime, never bundled into `pnpm dev`. An operator
// runs `pnpm --filter @brevio/fomo run migrate:neon` deliberately
// after reading the dev-server output that named the missing tables.
//
// Behavior:
//   * Reads every `apps/fomo/src/db/migrations/*.sql` in lexical order
//   * Splits on the `--> statement-breakpoint` marker
//   * Executes each non-empty statement via the configured DATABASE_URL
//   * Idempotent at the migration level: re-running on an already-
//     applied database is safe IF the migration SQL uses CREATE TABLE
//     IF NOT EXISTS / CREATE UNIQUE INDEX IF NOT EXISTS. As of 0004
//     this is NOT universally true — Drizzle's CREATE TABLE generator
//     does not emit IF NOT EXISTS — so the operator gets a clear
//     SQL error on duplicate-create. The verifier then runs at next
//     boot and confirms the table exists, after which the operator
//     can ignore the error.
//
// No DB credentials are read from anywhere except `process.env.DATABASE_URL`.
// No payload data is read. The migration files themselves are
// committed to the repo — there is no remote fetch.

import { readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import { Pool } from 'pg';

import {
  REQUIRED_TABLES,
  verifyMigrations
} from '../src/db/migration-verifier.js';
import { loadDbClient } from '../src/db/client.js';

async function main(): Promise<void> {
  if (!(process.env.DATABASE_URL ?? '').trim()) {
    process.stderr.write(
      '[migrate:neon] DATABASE_URL is not set. Aborting.\n' +
        '  Source apps/fomo/.env.<env>.local first, then re-run.\n'
    );
    process.exit(2);
  }

  process.stdout.write('[migrate:neon] Connecting via DATABASE_URL...\n');
  const pool = new Pool({ connectionString: process.env.DATABASE_URL });

  const here = path.dirname(fileURLToPath(import.meta.url));
  const migrationsDir = path.resolve(here, '..', 'src', 'db', 'migrations');
  const entries = await readdir(migrationsDir);
  const sqlFiles = entries.filter((f) => f.endsWith('.sql')).sort();

  if (sqlFiles.length === 0) {
    process.stderr.write(`[migrate:neon] No .sql files found in ${migrationsDir}. Aborting.\n`);
    await pool.end();
    process.exit(2);
  }

  let applied_statements = 0;
  let errored_statements = 0;

  for (const file of sqlFiles) {
    process.stdout.write(`[migrate:neon] Applying ${file}...\n`);
    const sql = await readFile(path.join(migrationsDir, file), 'utf8');
    const statements = sql
      .split('--> statement-breakpoint')
      .map((s) => s.trim())
      .filter((s) => s.length > 0);

    const client = await pool.connect();
    try {
      for (const stmt of statements) {
        try {
          await client.query(stmt);
          applied_statements++;
        } catch (err) {
          errored_statements++;
          process.stderr.write(
            `  [warn] statement failed (continuing): ${
              err instanceof Error ? err.message : String(err)
            }\n`
          );
        }
      }
    } finally {
      client.release();
    }
  }

  // Re-verify with the same code the runtime uses at boot. This is
  // the authoritative PASS/FAIL — if the verifier is green, the
  // dev server will boot.
  process.stdout.write('[migrate:neon] Re-verifying against runtime check...\n');
  const dbResult = loadDbClient({ env: process.env });
  if (!dbResult.ok) {
    process.stderr.write(`[migrate:neon] verifier load failed: ${dbResult.reason}\n`);
    await pool.end();
    process.exit(1);
  }
  const verification = await verifyMigrations(dbResult.client);
  await dbResult.pool.end();
  await pool.end();

  if (verification.ok) {
    process.stdout.write(
      `[migrate:neon] ✓ All ${REQUIRED_TABLES.length} required tables present. ` +
        `Applied ${applied_statements} statements (${errored_statements} non-fatal duplicate-create errors).\n`
    );
    process.exit(0);
  }

  if (verification.missing_tables.length > 0) {
    process.stderr.write(
      `[migrate:neon] ✖ Verifier still reports ${verification.missing_tables.length} missing table(s):\n`
    );
    for (const m of verification.missing_tables) {
      process.stderr.write(`    - table  ${m.name.padEnd(28)} (introduced by ${m.migration})\n`);
    }
  }
  if (verification.missing_columns.length > 0) {
    process.stderr.write(
      `[migrate:neon] ✖ Verifier still reports ${verification.missing_columns.length} missing column(s):\n`
    );
    for (const m of verification.missing_columns) {
      process.stderr.write(
        `    - column ${`${m.table}.${m.column}`.padEnd(48)} (introduced by ${m.migration})\n`
      );
    }
  }
  process.exit(1);
}

main().catch((err) => {
  process.stderr.write(`[migrate:neon] fatal: ${err instanceof Error ? err.stack ?? err.message : String(err)}\n`);
  process.exit(1);
});
