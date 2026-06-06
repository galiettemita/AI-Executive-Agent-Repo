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
  { name: 'inbound_replies', migration: '0004_inbound_replies.sql' },
  // 0005_users_v05.sql — Phase v0.5.1. Extends `users` (no row count
  // change but adds phone_e164_encrypted/phone_e164_hash/is_founder
  // columns) and creates the new `invite_tokens` table.
  { name: 'invite_tokens', migration: '0005_users_v05.sql' }
]);

// Phase v0.5.1 Step 4.2 — column-level verification.
//
// Real incident class (forward-projected from 3G.1): table-presence
// is necessary but insufficient. A migration that ALTERs an existing
// table to add a load-bearing column (e.g. 0005's `users.is_founder`,
// 0006's `invite_tokens.intended_phone_encrypted`) is INVISIBLE to
// the table-only verifier. Neon could have the table without the
// column and the runtime would fail at first use of the missing
// column — exactly the silent-failure shape we're hardening against.
//
// We list only LOAD-BEARING columns added by ALTER TABLE migrations.
// Columns added together with their parent table (e.g. all columns
// in 0000_init.sql) don't need to be re-listed here — the parent
// table's presence implies the columns. The verifier rejects the
// boot if any listed column is missing, with the same fail-loud
// shape as PendingMigrationsError.
export interface RequiredColumn {
  readonly table: string;
  readonly column: string;
  readonly migration: string;
}

export const REQUIRED_COLUMNS: readonly RequiredColumn[] = Object.freeze([
  // 0005_users_v05.sql — ALTER TABLE users + new invite_tokens table.
  // The users.* columns are added by ALTER, so they need explicit
  // column-level checks (the table existed before this migration).
  { table: 'users', column: 'phone_e164_encrypted', migration: '0005_users_v05.sql' },
  { table: 'users', column: 'phone_e164_hash', migration: '0005_users_v05.sql' },
  { table: 'users', column: 'is_founder', migration: '0005_users_v05.sql' },
  // The invite_tokens.* columns ship with the CREATE TABLE in 0005
  // so they're implied by the table presence — BUT founder directive
  // 2026-05-29 requires listing them explicitly so a future migration
  // that ALTERs the table can't silently drop one of them.
  { table: 'invite_tokens', column: 'token_hash', migration: '0005_users_v05.sql' },
  { table: 'invite_tokens', column: 'intended_phone_hash', migration: '0005_users_v05.sql' },
  { table: 'invite_tokens', column: 'consumed_at', migration: '0005_users_v05.sql' },
  // 0006_invite_phone_encrypted.sql — ALTER TABLE invite_tokens ADD COLUMN
  { table: 'invite_tokens', column: 'intended_phone_encrypted', migration: '0006_invite_phone_encrypted.sql' },
  // 0007_feedback_events_source_surface.sql — ALTER TABLE feedback_events
  // ADD COLUMN source_surface text NOT NULL DEFAULT 'email_alert' + index
  // (user_id, source_surface). Phase v0.5.9 Brevio-wide feedback substrate.
  { table: 'feedback_events', column: 'source_surface', migration: '0007_feedback_events_source_surface.sql' }
]);

export interface MigrationVerificationResult {
  readonly ok: boolean;
  readonly required_tables: readonly string[];
  readonly missing_tables: readonly { name: string; migration: string }[];
  // Phase v0.5.1 Step 4.2 — columns whose parent table exists but
  // the column itself is missing. When the parent table is also
  // missing, only `missing_tables` carries the entry (we don't
  // double-report; recovery is "apply the migration that adds the
  // table" either way).
  readonly missing_columns: readonly { table: string; column: string; migration: string }[];
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
  // Tables first.
  const missing_tables: { name: string; migration: string }[] = [];
  const present_tables = new Set<string>();
  for (const t of REQUIRED_TABLES) {
    const rows = await db.execute(sql`SELECT to_regclass(${`public.${t.name}`}) AS r`);
    const r = (rows.rows[0] as { r: string | null } | undefined)?.r ?? null;
    if (r === null) {
      missing_tables.push({ name: t.name, migration: t.migration });
    } else {
      present_tables.add(t.name);
    }
  }

  // Phase v0.5.1 Step 4.2 — column-level verification. Only check
  // columns on tables that EXIST; if the parent table is missing,
  // the column-missing entry is redundant noise. The operator will
  // create the table via the named migration, which brings the
  // column with it.
  const missing_columns: { table: string; column: string; migration: string }[] = [];
  // Bulk-fetch column lists for every table we care about. One round-trip
  // per parent table; cheaper than N round-trips per column.
  const tableColumnsCache = new Map<string, Set<string>>();
  for (const c of REQUIRED_COLUMNS) {
    if (!present_tables.has(c.table)) continue;
    let cols = tableColumnsCache.get(c.table);
    if (!cols) {
      const rows = await db.execute(
        sql`SELECT column_name FROM information_schema.columns
            WHERE table_schema = 'public' AND table_name = ${c.table}`
      );
      cols = new Set((rows.rows as { column_name: string }[]).map((r) => r.column_name));
      tableColumnsCache.set(c.table, cols);
    }
    if (!cols.has(c.column)) {
      missing_columns.push({ table: c.table, column: c.column, migration: c.migration });
    }
  }

  return Object.freeze({
    ok: missing_tables.length === 0 && missing_columns.length === 0,
    required_tables: REQUIRED_TABLES.map((t) => t.name),
    missing_tables: Object.freeze(missing_tables) as readonly { name: string; migration: string }[],
    missing_columns: Object.freeze(missing_columns) as readonly { table: string; column: string; migration: string }[]
  });
}

export class PendingMigrationsError extends Error {
  readonly missing_tables: readonly { name: string; migration: string }[];
  readonly missing_columns: readonly { table: string; column: string; migration: string }[];
  constructor(
    missing_tables: readonly { name: string; migration: string }[],
    missing_columns: readonly { table: string; column: string; migration: string }[] = []
  ) {
    const tableLines = missing_tables
      .map((m) => `  - table ${m.name.padEnd(28)} (introduced by ${m.migration})`)
      .join('\n');
    const columnLines = missing_columns
      .map((m) => `  - column ${`${m.table}.${m.column}`.padEnd(48)} (introduced by ${m.migration})`)
      .join('\n');
    const sections: string[] = [];
    if (missing_tables.length > 0) {
      sections.push(
        `${missing_tables.length} required table(s) missing from public schema:\n${tableLines}`
      );
    }
    if (missing_columns.length > 0) {
      sections.push(
        `${missing_columns.length} required column(s) missing on existing table(s):\n${columnLines}`
      );
    }
    super(
      'fomo.migrations.pending — refusing to boot.\n' +
        sections.join('\n\n') +
        '\n\n' +
        'Apply pending migrations explicitly, then restart:\n' +
        '  pnpm --filter @brevio/fomo run migrate:neon\n\n' +
        'No auto-apply. No env bypass. Founder-locked policy (Phase 3G.1, 2026-05-29).'
    );
    this.name = 'PendingMigrationsError';
    this.missing_tables = missing_tables;
    this.missing_columns = missing_columns;
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
    throw new PendingMigrationsError(result.missing_tables, result.missing_columns);
  }
}
