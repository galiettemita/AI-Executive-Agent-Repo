// Phase 3G.1 item #1 — Neon migration verification at boot.
//
// Real incident (2026-05-28 01:06 UTC): the first three POSTs to
// /sendblue/inbound during the 3F.2 smoke run returned HTTP 500
// because migration 0004_inbound_replies had been applied via PGlite
// in the gated `gated-pg.test.ts` suite, but had NOT been applied
// against the live Neon database. The runtime crashed in the
// inbound_replies INSERT with a generic "Failed query" error. Same
// shape hit during 3D.2 when the `alerts` table was missing.
//
// Founder-locked policy (2026-05-29 6-question gate for 3G.1):
//   - Fail-loud everywhere. No auto-apply in any environment.
//   - No easy production bypass. The verifier has NO env override.
//   - Print the named list of missing tables at fatal.
//   - Document an explicit migration command (see scripts/migrate-neon.ts
//     and the `pnpm migrate:neon` script in apps/fomo/package.json).
//
// Run shape:
//   1. main() loads a DB client.
//   2. main() calls verifyMigrationsOrThrow(db).
//   3. On any missing table, the function throws a named-list error
//      and main() exits with code 1 before any worker installs.
//   4. The same function is exercised by unit tests against PGlite
//      with controlled missing-table scenarios.

import { sql } from 'drizzle-orm';

import { type DrizzleClient } from './client.js';

// Each required table maps to the migration file that introduces it.
// Order matches `apps/fomo/src/db/migrations/`. When a new migration
// adds a table, append it here AND ensure the file ships in the same
// PR. The verifier returns the named list so an operator can correlate
// missing tables to the migration file that adds them.
export interface RequiredTable {
  readonly name: string;
  readonly migration: string;
}

export const REQUIRED_TABLES: readonly RequiredTable[] = Object.freeze([
  // 0000_init.sql — cross-verified against gated-pg.test.ts:88-102
  { name: 'alert_state_transitions', migration: '0000_init.sql' },
  { name: 'audit_log', migration: '0000_init.sql' },
  { name: 'consent', migration: '0000_init.sql' },
  { name: 'cost_records', migration: '0000_init.sql' },
  { name: 'feedback_events', migration: '0000_init.sql' },
  { name: 'memory_signals', migration: '0000_init.sql' },
  { name: 'oauth_tokens', migration: '0000_init.sql' },
  { name: 'tool_invocations', migration: '0000_init.sql' },
  { name: 'users', migration: '0000_init.sql' },
  // 0001_gmail_cursors.sql
  { name: 'gmail_cursors', migration: '0001_gmail_cursors.sql' },
  // 0002_rank_results.sql
  { name: 'rank_results', migration: '0002_rank_results.sql' },
  // 0003_alerts.sql
  { name: 'alerts', migration: '0003_alerts.sql' },
  // 0004_inbound_replies.sql
  { name: 'inbound_replies', migration: '0004_inbound_replies.sql' }
]);

export interface MigrationVerificationResult {
  readonly ok: boolean;
  readonly required_tables: readonly string[];
  readonly missing_tables: readonly { name: string; migration: string }[];
}

/**
 * Queries `to_regclass('public.<table>')` for every required table and
 * returns the structured result. Does NOT throw. Read-only — never
 * mutates the database.
 *
 * The PASS criterion is "every required table exists in the `public`
 * schema." This intentionally ignores other migration-tracking
 * mechanisms (drizzle journal, `__migrations` tables, etc.) so the
 * check works against any DB that satisfies the runtime contract,
 * not just the one Drizzle owns end-to-end.
 */
export async function verifyMigrations(db: DrizzleClient): Promise<MigrationVerificationResult> {
  const missing: { name: string; migration: string }[] = [];
  for (const t of REQUIRED_TABLES) {
    const rows = await db.execute(sql`SELECT to_regclass(${`public.${t.name}`}) AS r`);
    // pg's to_regclass returns null when the table is missing.
    const r = (rows.rows[0] as { r: string | null } | undefined)?.r ?? null;
    if (r === null) {
      missing.push({ name: t.name, migration: t.migration });
    }
  }
  return Object.freeze({
    ok: missing.length === 0,
    required_tables: REQUIRED_TABLES.map((t) => t.name),
    missing_tables: Object.freeze(missing) as readonly { name: string; migration: string }[]
  });
}

export class PendingMigrationsError extends Error {
  readonly missing_tables: readonly { name: string; migration: string }[];
  constructor(missing: readonly { name: string; migration: string }[]) {
    const lines = missing
      .map((m) => `  - ${m.name.padEnd(28)} (introduced by ${m.migration})`)
      .join('\n');
    super(
      'fomo.migrations.pending — refusing to boot. ' +
        `${missing.length} required table(s) missing from public schema:\n${lines}\n\n` +
        'Apply pending migrations explicitly, then restart:\n' +
        '  pnpm --filter @brevio/fomo run migrate:neon\n\n' +
        'No auto-apply. No env bypass. Founder-locked policy (Phase 3G.1, 2026-05-29).'
    );
    this.name = 'PendingMigrationsError';
    this.missing_tables = missing;
  }
}

/**
 * Fail-loud wrapper around verifyMigrations. Throws a named
 * PendingMigrationsError when any required table is missing,
 * with a fully-formatted message a human can act on directly.
 *
 * No env override. No auto-apply. Founder-locked.
 */
export async function verifyMigrationsOrThrow(db: DrizzleClient): Promise<void> {
  const result = await verifyMigrations(db);
  if (!result.ok) {
    throw new PendingMigrationsError(result.missing_tables);
  }
}
