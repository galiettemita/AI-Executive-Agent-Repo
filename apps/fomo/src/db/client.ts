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

  const client = drizzle(pool, { schema });

  return Object.freeze({ ok: true as const, client, pool });
}

// Convenience for tests / shutdown hooks that need to close the pool.
export async function closeDbClient(result: DbClientResult): Promise<void> {
  if (result.ok) {
    await result.pool.end();
  }
}
