// Drizzle client factory — env-driven, fail-closed in production.
//
// Rules (FOMO_PLAN §11):
//   * DATABASE_URL set                → returns a live Drizzle client
//   * DATABASE_URL missing + dev      → returns no_database_url_dev_mode
//                                        (callers fall back to in-memory)
//   * DATABASE_URL missing + production → throws
//   * BREVIO_DEV_MODE=true             → bypasses the production check
//                                        (developer escape hatch — all
//                                        persisted state is lost)
//
// The factory creates a connection pool but does not attempt a query
// during loadDbClient(). Connections are lazy; the first store call is
// what actually hits the database.

import { Pool } from 'pg';
import { drizzle, type NodePgDatabase } from 'drizzle-orm/node-postgres';

import * as schema from './schema.js';

export type DrizzleClient = NodePgDatabase<typeof schema>;

export type DbClientResult =
  | { readonly ok: true; readonly client: DrizzleClient; readonly pool: Pool }
  | { readonly ok: false; readonly reason: 'no_database_url_dev_mode' | 'in_memory_dev_mode' };

export interface DbClientConfig {
  // Override of process.env for testability.
  env?: NodeJS.ProcessEnv;
  // Pool tuning. Defaults are sensible for a small Render Starter dyno.
  poolMax?: number;
  idleTimeoutMs?: number;
  connectionTimeoutMs?: number;
  // Phase v0.5.3 item #3 — best-effort hook called from the pg pool's
  // 'error' event handler (for transient ECONNRESET / server-side
  // timeouts on idle connections). The handler ALWAYS logs to stderr
  // first (primary, works even when DB is down). This hook is called
  // SECOND and is wrapped in try/catch so a DB write failure does NOT
  // crash the process. Typical wiring: an audit-store.write that
  // emits fomo.db.connection_error.
  onPoolError?: (err: Error) => Promise<void> | void;
  // Phase v0.5.3 item #3 — stderr writer override for tests. Defaults
  // to process.stderr. Tests inject a capture buffer.
  stderrWrite?: (line: string) => void;
}

export function loadDbClient(config: DbClientConfig = {}): DbClientResult {
  const env = config.env ?? process.env;
  const databaseUrl = env.DATABASE_URL?.trim();
  const isProduction = env.NODE_ENV === 'production';
  const isDevMode = env.BREVIO_DEV_MODE === 'true';

  if (!databaseUrl) {
    if (isProduction && !isDevMode) {
      throw new Error(
        'DATABASE_URL required in production. Set BREVIO_DEV_MODE=true to use in-memory stores (dev only — all persisted state is lost on restart).'
      );
    }
    return Object.freeze({
      ok: false as const,
      reason: isDevMode ? ('in_memory_dev_mode' as const) : ('no_database_url_dev_mode' as const)
    });
  }

  const pool = new Pool({
    connectionString: databaseUrl,
    max: config.poolMax ?? 10,
    idleTimeoutMillis: config.idleTimeoutMs ?? 30_000,
    connectionTimeoutMillis: config.connectionTimeoutMs ?? 10_000
  });

  // Phase v0.5.3 item #3 — defensive pool 'error' handler. Without
  // this, pg's idle-connection-dropped errors (Neon ECONNRESET in
  // particular) go unhandled and Node treats them as unhandled-error
  // events, crashing the process. v0.5.2 smoke incident 2026-06-01:
  // dev server crashed for 18+ hours mid-smoke when Neon dropped an
  // idle connection. Per founder correction #3:
  //   (a) log structured sanitized error FIRST to stderr — works
  //       even when the DB is down (which is exactly the case we're
  //       in when the pool fires 'error')
  //   (b) call onPoolError best-effort SECOND, wrapped in try/catch
  //       so a DB write failure here does NOT cascade to a crash
  //   (c) NEVER let the error escape — no rethrow, no process.exit
  // The pool's own reconnect logic handles re-establishing connections
  // on the next query attempt; this handler exists only to prevent
  // the unhandled-error crash path.
  const stderrWrite = config.stderrWrite ?? ((line: string) => process.stderr.write(line));
  pool.on('error', (err) => {
    const code = (err as NodeJS.ErrnoException).code ?? 'unknown';
    // Sanitized message — bounded length, never the connection string.
    const message = (err.message ?? String(err)).slice(0, 200);
    const sanitized = {
      ts: new Date().toISOString(),
      service: 'fomo',
      event: 'fomo.db.connection_error',
      severity: 'ERROR',
      attrs: { error_code: code, message }
    };
    try {
      stderrWrite(JSON.stringify(sanitized) + '\n');
    } catch {
      // Cannot even log? Swallow. The alternative is a crash.
    }
    if (config.onPoolError) {
      try {
        const r = config.onPoolError(err);
        if (r && typeof (r as Promise<void>).catch === 'function') {
          (r as Promise<void>).catch(() => undefined);
        }
      } catch {
        // Best-effort. Never let the audit attempt surface.
      }
    }
  });

  const client = drizzle(pool, { schema });

  return Object.freeze({ ok: true as const, client, pool });
}

// Convenience for tests / shutdown hooks that need to close the pool.
export async function closeDbClient(result: DbClientResult): Promise<void> {
  if (result.ok) {
    await result.pool.end();
  }
}
